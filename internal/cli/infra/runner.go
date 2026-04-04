package infra

import (
	"bufio"
	"bytes"
	ansible "civa/ansible"
	infssh "civa/internal/cli/infra/ssh"
	"civa/internal/cli/infra/storage"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

type plannedRunMetadata struct {
	WebServer     string   `json:"webServer"`
	SSHAuthMethod string   `json:"sshAuthMethod"`
	SSHUser       string   `json:"sshUser"`
	SSHPort       int      `json:"sshPort"`
	Components    []string `json:"components"`
	InventoryFile string   `json:"inventoryFile"`
	VarsFile      string   `json:"varsFile"`
	AuthFile      string   `json:"authFile,omitempty"`
	PlanFile      string   `json:"planFile"`
	PlaybookFile  string   `json:"playbookFile"`
}

func civaHomeDirectory() string {
	return storage.CivaHomeDirectory()
}

func runRootDirectoryPath() string {
	return storage.RunRootDirectoryPath()
}

func latestPlanPointerFilePath() string {
	return storage.LatestPlanPointerFilePath()
}

func resolvePlanInputFile(cfg *config) (string, error) {
	if err := finalizePaths(cfg); err != nil {
		return "", err
	}

	if strings.TrimSpace(cfg.PlanName) != "" {
		planPath := planPathForName(cfg.PlanName)
		if _, err := os.Stat(planPath); err == nil {
			cfg.PlanInputFile = planPath
			return planPath, nil
		}

		versionedPlanName, ok, err := latestPlanVersionName(cfg.PlanName)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", fmt.Errorf("generated plan not found: %s", cfg.PlanName)
		}

		planPath = planPathForName(versionedPlanName)
		cfg.PlanInputFile = planPath
		cfg.PlanName = versionedPlanName
		return planPath, nil
	}

	if strings.TrimSpace(cfg.PlanInputFile) != "" {
		if _, err := os.Stat(cfg.PlanInputFile); err != nil {
			return "", fmt.Errorf("plan file not found: %s", cfg.PlanInputFile)
		}
		return cfg.PlanInputFile, nil
	}

	return "", fmt.Errorf("preview/apply require a generated plan name or --plan-file")
}

func readLatestPlanPointer() (string, error) {
	content, err := os.ReadFile(latestPlanPointerFilePath())
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(content))
	if path == "" {
		return "", fmt.Errorf("latest plan pointer is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

func writeLatestPlanPointer(planPath string) error {
	if err := os.MkdirAll(filepath.Dir(latestPlanPointerFilePath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(latestPlanPointerFilePath(), []byte(planPath+"\n"), 0o644)
}

func planPathForName(name string) string {
	return storage.PlanPathForName(name)
}

func planDirForName(name string) string {
	return storage.PlanDirForName(name)
}

func listPlans(filter string) error {
	filter = sanitizePlanName(filter)

	entries, err := os.ReadDir(runRootDirectoryPath())
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No generated plans found.")
			return nil
		}
		return err
	}

	latestPlan, _ := readLatestPlanPointer()

	type planEntry struct {
		Name    string
		Path    string
		Version int
	}

	plans := make([]planEntry, 0, len(entries))

	printed := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		planName := entry.Name()
		baseName, version := splitPlanVersionName(planName)
		if filter != "" && baseName != filter {
			continue
		}

		planPath := planPathForName(planName)
		if _, err := os.Stat(planPath); err != nil {
			continue
		}
		plans = append(plans, planEntry{Name: planName, Path: planPath, Version: version})
	}

	if filter != "" {
		sort.Slice(plans, func(i, j int) bool {
			if plans[i].Version == plans[j].Version {
				return plans[i].Name > plans[j].Name
			}
			return plans[i].Version > plans[j].Version
		})
		if len(plans) == 0 {
			fmt.Printf("No generated plans found for %s.\n", filter)
			return nil
		}
		fmt.Printf("Plan versions for %s:\n", filter)
	} else {
		sort.Slice(plans, func(i, j int) bool {
			return plans[i].Name > plans[j].Name
		})
	}

	for _, plan := range plans {
		marker := ""
		if latestPlan == plan.Path {
			marker = " (latest)"
		}

		fmt.Printf("- %s%s\n  %s\n", plan.Name, marker, plan.Path)
		printed = true
	}

	if !printed {
		fmt.Println("No generated plans found.")
	}
	return nil
}

func removePlan(cfg *config) error {
	requestedName := strings.TrimSpace(cfg.PlanName)
	targetDir := planDirForName(requestedName)
	planPath := planPathForName(requestedName)
	if _, err := os.Stat(planPath); err != nil {
		versionedName, ok, resolveErr := latestPlanVersionName(requestedName)
		if resolveErr != nil {
			return resolveErr
		}
		if !ok {
			return fmt.Errorf("generated plan not found: %s", requestedName)
		}
		requestedName = versionedName
		targetDir = planDirForName(requestedName)
		planPath = planPathForName(requestedName)
	}

	if !cfg.AssumeYes {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("non-interactive plan removal requires --yes")
		}

		confirmed, err := promptPlanRemovalConfirmation(requestedName)
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
		if !confirmed {
			fmt.Fprintln(os.Stderr, "civa plan remove was cancelled by the user.")
			return nil
		}
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("failed to remove plan %s: %w", requestedName, err)
	}

	if latestPlan, err := readLatestPlanPointer(); err == nil && latestPlan == planPath {
		if nextLatest, nextErr := latestGeneratedPlanFile(); nextErr == nil {
			if writeErr := writeLatestPlanPointer(nextLatest); writeErr != nil {
				return writeErr
			}
		} else {
			_ = os.Remove(latestPlanPointerFilePath())
		}
	}

	fmt.Printf("Removed generated plan %s\n", requestedName)
	return nil
}

func splitPlanVersionName(planName string) (string, int) {
	planName = strings.TrimSpace(planName)
	if planName == "" {
		return "", 0
	}

	re := regexp.MustCompile(`^(.*?)-v([0-9]+)$`)
	match := re.FindStringSubmatch(planName)
	if len(match) != 3 {
		return planName, 1
	}

	version, err := strconv.Atoi(match[2])
	if err != nil || version < 1 {
		return planName, 1
	}

	return match[1], version
}

func latestPlanVersionName(baseName string) (string, bool, error) {
	baseName = sanitizePlanName(baseName)
	if baseName == "" {
		return "", false, nil
	}

	entries, err := os.ReadDir(runRootDirectoryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	latestName := ""
	latestVersion := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		base, version := splitPlanVersionName(name)
		if base != baseName {
			continue
		}
		if _, err := os.Stat(planPathForName(name)); err != nil {
			continue
		}
		if version > latestVersion {
			latestVersion = version
			latestName = name
		}
	}

	if latestName == "" {
		return "", false, nil
	}

	return latestName, true, nil
}

func nextPlanVersionName(baseName string) (string, error) {
	baseName = sanitizePlanName(baseName)
	if baseName == "" {
		return "", fmt.Errorf("plan name is empty")
	}

	latestName, found, err := latestPlanVersionName(baseName)
	if err != nil {
		return "", err
	}
	if !found {
		return baseName, nil
	}

	_, latestVersion := splitPlanVersionName(latestName)
	return fmt.Sprintf("%s-v%d", baseName, latestVersion+1), nil
}

func latestGeneratedPlanFile() (string, error) {
	entries, err := os.ReadDir(runRootDirectoryPath())
	if err != nil {
		return "", err
	}

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(runRootDirectoryPath(), entry.Name(), "plan.md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no generated plan found under %s", runRootDirectoryPath())
}

func loadPlannedRun(planPath string) (*config, *runtimeState, error) {
	metadata, err := loadPlannedRunMetadata(planPath)
	if err == nil {
		return metadataToPlannedRun(planPath, metadata)
	}

	return loadPlannedRunFromMarkdown(planPath)
}

func loadPlannedRunMetadata(planPath string) (*plannedRunMetadata, error) {
	metadataPath := planMetadataPath(planPath)
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	var metadata plannedRunMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func metadataToPlannedRun(planPath string, metadata *plannedRunMetadata) (*config, *runtimeState, error) {
	loadedCfg := defaultConfig(commandApply)
	loadedCfg.WebServer = metadata.WebServer
	loadedCfg.SSHAuthMethod = metadata.SSHAuthMethod
	loadedCfg.SSHUser = metadata.SSHUser
	loadedCfg.SSHPort = metadata.SSHPort
	loadedCfg.Components = append([]string(nil), metadata.Components...)
	resolvedPlanFile := metadata.PlanFile
	if strings.TrimSpace(resolvedPlanFile) == "" {
		resolvedPlanFile = planPath
	}
	resolvedPlanFile = normalizePlannedArtifactPath(planPath, resolvedPlanFile)

	state := &runtimeState{
		InventoryFile: normalizePlannedArtifactPath(planPath, metadata.InventoryFile),
		VarsFile:      normalizePlannedArtifactPath(planPath, metadata.VarsFile),
		AuthFile:      normalizePlannedArtifactPath(planPath, metadata.AuthFile),
		MetadataFile:  planMetadataPath(planPath),
		PlanFile:      resolvedPlanFile,
		PlaybookFile:  normalizePlannedArtifactPath(planPath, metadata.PlaybookFile),
	}

	if state.InventoryFile == "" || state.VarsFile == "" || state.PlaybookFile == "" || state.PlanFile == "" {
		return nil, nil, fmt.Errorf("plan metadata is missing execution artifacts: %s", planPath)
	}
	if len(loadedCfg.Components) == 0 {
		return nil, nil, fmt.Errorf("plan metadata is missing selected components: %s", planPath)
	}

	for _, path := range []string{state.InventoryFile, state.VarsFile, state.PlaybookFile, state.PlanFile} {
		if _, err := os.Stat(path); err != nil {
			return nil, nil, fmt.Errorf("planned artifact not found: %s", path)
		}
	}
	if state.AuthFile != "" {
		if _, err := os.Stat(state.AuthFile); err != nil {
			return nil, nil, fmt.Errorf("planned auth file not found: %s", state.AuthFile)
		}
	}

	return &loadedCfg, state, nil
}

func loadPlannedRunFromMarkdown(planPath string) (*config, *runtimeState, error) {
	file, err := os.Open(planPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open plan file %s: %w", planPath, err)
	}
	defer file.Close()

	loadedCfg := defaultConfig(commandApply)
	loadedCfg.PlanInputFile = planPath
	state := &runtimeState{PlanFile: planPath}
	section := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		switch trimmed {
		case "## Mode":
			section = "mode"
			continue
		case "## Selected Components":
			section = "components"
			continue
		case "## Generated Files":
			section = "files"
			continue
		case "## Command":
			section = "command"
			continue
		}

		if section == "mode" {
			switch {
			case strings.HasPrefix(trimmed, "- Web server: "):
				server := strings.TrimPrefix(trimmed, "- Web server: ")
				switch strings.ToLower(server) {
				case "traefik":
					loadedCfg.WebServer = webServerTraefik
				case "nginx":
					loadedCfg.WebServer = webServerNginx
				case "caddy":
					loadedCfg.WebServer = webServerCaddy
				default:
					loadedCfg.WebServer = webServerNone
				}
			case strings.HasPrefix(trimmed, "- SSH auth method: "):
				method := strings.TrimPrefix(trimmed, "- SSH auth method: ")
				if strings.EqualFold(method, "password") {
					loadedCfg.SSHAuthMethod = sshAuthMethodPassword
				} else {
					loadedCfg.SSHAuthMethod = sshAuthMethodKey
				}
			case strings.HasPrefix(trimmed, "- SSH user: "):
				loadedCfg.SSHUser = strings.TrimPrefix(trimmed, "- SSH user: ")
			case strings.HasPrefix(trimmed, "- SSH port: "):
				port, convErr := strconv.Atoi(strings.TrimPrefix(trimmed, "- SSH port: "))
				if convErr == nil {
					loadedCfg.SSHPort = port
				}
			}
		}

		if section == "components" && strings.HasPrefix(trimmed, "- ") {
			label := strings.TrimPrefix(trimmed, "- ")
			for _, option := range componentOptions {
				if option.Label == label {
					loadedCfg.Components = append(loadedCfg.Components, option.Value)
					break
				}
			}
		}

		if section == "files" {
			switch {
			case strings.HasPrefix(trimmed, "- Inventory: "):
				state.InventoryFile = strings.TrimPrefix(trimmed, "- Inventory: ")
			case strings.HasPrefix(trimmed, "- Vars: "):
				state.VarsFile = strings.TrimPrefix(trimmed, "- Vars: ")
			case strings.HasPrefix(trimmed, "- SSH auth file: "):
				value := strings.TrimPrefix(trimmed, "- SSH auth file: ")
				if value != "not used" {
					state.AuthFile = value
				}
			case strings.HasPrefix(trimmed, "- Playbook: "):
				state.PlaybookFile = strings.TrimPrefix(trimmed, "- Playbook: ")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to read plan file %s: %w", planPath, err)
	}

	state.InventoryFile = normalizePlannedArtifactPath(planPath, state.InventoryFile)
	state.VarsFile = normalizePlannedArtifactPath(planPath, state.VarsFile)
	state.PlaybookFile = normalizePlannedArtifactPath(planPath, state.PlaybookFile)
	state.AuthFile = normalizePlannedArtifactPath(planPath, state.AuthFile)

	if state.InventoryFile == "" || state.VarsFile == "" || state.PlaybookFile == "" {
		return nil, nil, fmt.Errorf("plan file is missing execution artifacts: %s", planPath)
	}
	if len(loadedCfg.Components) == 0 {
		return nil, nil, fmt.Errorf("plan file is missing selected components: %s", planPath)
	}

	for _, path := range []string{state.InventoryFile, state.VarsFile, state.PlaybookFile} {
		if _, err := os.Stat(path); err != nil {
			return nil, nil, fmt.Errorf("planned artifact not found: %s", path)
		}
	}
	if state.AuthFile != "" {
		if _, err := os.Stat(state.AuthFile); err != nil {
			return nil, nil, fmt.Errorf("planned auth file not found: %s", state.AuthFile)
		}
	}

	return &loadedCfg, state, nil
}

func normalizePlannedArtifactPath(planPath, candidate string) string {
	resolved := strings.TrimSpace(candidate)
	if resolved == "" {
		return ""
	}

	if strings.HasPrefix(resolved, "~/") {
		expanded, err := expandHomePath(resolved)
		if err == nil {
			return expanded
		}
	}

	if filepath.IsAbs(resolved) {
		return filepath.Clean(resolved)
	}

	normalizedSlash := filepath.ToSlash(resolved)
	if strings.HasPrefix(normalizedSlash, ".civa/") {
		remainder := strings.TrimPrefix(normalizedSlash, ".civa/")
		return filepath.Join(civaHomeDirectory(), filepath.FromSlash(remainder))
	}

	if planPath != "" {
		return filepath.Join(filepath.Dir(planPath), resolved)
	}

	return filepath.Clean(resolved)
}

func runUninstall(cfg config) error {
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve current executable: %w", err)
	}

	targetPath, err := uninstallTargetPathForPath(executablePath)
	if err != nil {
		return err
	}

	if !cfg.AssumeYes {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("non-interactive uninstall requires --yes")
		}

		confirmed, err := promptUninstallConfirmation(targetPath)
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
		if !confirmed {
			fmt.Fprintln(os.Stderr, "civa uninstall was cancelled by the user.")
			return nil
		}
	}

	if err := os.Remove(targetPath); err != nil {
		return fmt.Errorf("failed to remove %s: %w", targetPath, err)
	}

	fmt.Printf("Removed %s\n", targetPath)
	return nil
}

func runSSHCopyID(cfg config) error {
	server := cfg.Servers[0]
	sshPort := initiationSSHPort(cfg)
	target := fmt.Sprintf("%s@%s", cfg.SSHUser, server.Address)
	if err := rewriteKnownHostEntry(server.Address, sshPort); err != nil {
		return err
	}

	cmd := buildSSHCopyIDCommand(cfg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ssh-copy-id failed for %s: %w", target, err)
	}

	fmt.Printf("Installed %s on %s\n", cfg.SSHPublicKey, target)
	return nil
}

func rewriteKnownHostEntry(host string, port int) error {
	return infssh.RewriteKnownHostEntry(host, port)
}

func buildSSHCopyIDCommand(cfg config) *exec.Cmd {
	target := fmt.Sprintf("%s@%s", cfg.SSHUser, cfg.Servers[0].Address)
	sshPort := initiationSSHPort(cfg)
	args := []string{
		"-i", cfg.SSHPublicKey,
		"-p", strconv.Itoa(sshPort),
		"-o", "StrictHostKeyChecking=accept-new",
		target,
	}

	if strings.TrimSpace(cfg.SSHPassword) == "" {
		return exec.Command("ssh-copy-id", args...)
	}

	cmd := exec.Command("sshpass", append([]string{"-e", "ssh-copy-id"}, args...)...)
	cmd.Env = append(os.Environ(), "SSHPASS="+cfg.SSHPassword)
	return cmd
}

func effectiveSSHPort(cfg config, server serverSpec) int {
	if server.SSHPort >= 1 && server.SSHPort <= 65535 {
		return server.SSHPort
	}
	if cfg.SSHPort >= 1 && cfg.SSHPort <= 65535 {
		return cfg.SSHPort
	}

	return defaultSSHPort
}

func initiationSSHPort(cfg config) int {
	if cfg.SSHPort >= 1 && cfg.SSHPort <= 65535 {
		return cfg.SSHPort
	}

	return defaultSSHPort
}

func uninstallTargetPathForPath(path string) (string, error) {
	path = filepath.Clean(path)
	if filepath.Base(path) != "civa" {
		return "", fmt.Errorf("refusing to uninstall unexpected executable: %s", path)
	}

	return path, nil
}

func materializeAnsibleAssets(ansibleDir string) error {
	return ansible.Materialize(ansibleDir)
}

func runDoctor(cfg config) error {
	printSection("civa Doctor")
	fmt.Printf("Version: %s\n", version)

	requirements := doctorRequirements()
	results := checkDoctorRequirements(requirements)
	failures := printDoctorResults(results)

	if cfg.DoctorAction == doctorActionFix {
		if failures == 0 {
			fmt.Println("Doctor fix summary: nothing to install, local machine is already ready")
			return nil
		}

		fmt.Println("Attempting to install or update missing dependencies...")
		if err := runDoctorFix(requirements); err != nil {
			return fmt.Errorf("doctor fix failed: %w", err)
		}

		fmt.Println("Re-checking dependencies after fix...")
		results = checkDoctorRequirements(requirements)
		failures = printDoctorResults(results)
	}

	if failures > 0 {
		return fmt.Errorf("doctor summary: %d blocker(s) found", failures)
	}

	fmt.Println("Doctor summary: local machine looks ready to run civa")
	return nil
}

type doctorRequirement struct {
	Name           string
	Command        string
	VersionArgs    []string
	MinimumVersion string
}

type doctorResult struct {
	Requirement doctorRequirement
	Found       bool
	Version     string
	VersionOK   bool
	Error       error
}

func doctorRequirements() []doctorRequirement {
	return []doctorRequirement{
		{Name: "go", Command: "go", VersionArgs: []string{"version"}, MinimumVersion: "1.26"},
		{Name: "ansible-playbook", Command: "ansible-playbook", VersionArgs: []string{"--version"}, MinimumVersion: "2.20"},
		{Name: "python", Command: "python3", VersionArgs: []string{"--version"}, MinimumVersion: "3.10"},
	}
}

func checkDoctorRequirements(requirements []doctorRequirement) []doctorResult {
	results := make([]doctorResult, 0, len(requirements))
	for _, req := range requirements {
		result := doctorResult{Requirement: req}

		if _, err := exec.LookPath(req.Command); err != nil {
			result.Error = err
			results = append(results, result)
			continue
		}

		output, err := runCommandOutput(req.Command, req.VersionArgs...)
		if err != nil {
			result.Error = err
			results = append(results, result)
			continue
		}

		version := extractFirstSemver(output)
		if version == "" {
			result.Error = fmt.Errorf("unable to parse version from %s output", req.Command)
			results = append(results, result)
			continue
		}

		result.Found = true
		result.Version = version
		result.VersionOK = versionAtLeast(version, req.MinimumVersion)
		if !result.VersionOK {
			result.Error = fmt.Errorf("version %s is lower than required %s", version, req.MinimumVersion)
		}
		results = append(results, result)
	}

	return results
}

func printDoctorResults(results []doctorResult) int {
	failures := 0
	for _, result := range results {
		if result.Found && result.VersionOK {
			fmt.Printf("[ok] %s %s found (required >= %s)\n", result.Requirement.Name, result.Version, result.Requirement.MinimumVersion)
			continue
		}

		failures++
		if result.Found {
			fmt.Printf("[fail] %s %s is below required >= %s\n", result.Requirement.Name, result.Version, result.Requirement.MinimumVersion)
		} else {
			fmt.Printf("[fail] %s not found (required >= %s)\n", result.Requirement.Name, result.Requirement.MinimumVersion)
		}
	}

	return failures
}

func runDoctorFix(requirements []doctorRequirement) error {
	manager, err := detectPackageManager()
	if err != nil {
		return err
	}

	packages := doctorPackagesForManager(manager)
	if len(packages) == 0 {
		return fmt.Errorf("no packages configured for package manager %s", manager)
	}

	fmt.Printf("Using package manager: %s\n", manager)

	switch manager {
	case "apt-get":
		if err := runInstallCommand("sudo", "apt-get", "update"); err != nil {
			return err
		}
		args := append([]string{"apt-get", "install", "-y"}, packages...)
		if err := runInstallCommand("sudo", args...); err != nil {
			return err
		}
	case "dnf":
		args := append([]string{"dnf", "install", "-y"}, packages...)
		if err := runInstallCommand("sudo", args...); err != nil {
			return err
		}
	case "yum":
		args := append([]string{"yum", "install", "-y"}, packages...)
		if err := runInstallCommand("sudo", args...); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported package manager for doctor fix: %s", manager)
	}

	_ = requirements
	return nil
}

func detectPackageManager() (string, error) {
	for _, manager := range []string{"apt-get", "dnf", "yum"} {
		if _, err := exec.LookPath(manager); err == nil {
			return manager, nil
		}
	}

	return "", fmt.Errorf("doctor fix supports apt-get, dnf, or yum only")
}

func doctorPackagesForManager(manager string) []string {
	switch manager {
	case "apt-get":
		return []string{"golang-go", "ansible", "python3"}
	case "dnf", "yum":
		return []string{"golang", "ansible", "python3"}
	default:
		return nil
	}
}

func runInstallCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed command %s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func runCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed command %s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(output), nil
}

func extractFirstSemver(text string) string {
	re := regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)
	match := re.FindString(text)
	return strings.TrimSpace(match)
}

func versionAtLeast(actual, minimum string) bool {
	actualParts := parseVersionParts(actual)
	minimumParts := parseVersionParts(minimum)
	maxParts := len(actualParts)
	if len(minimumParts) > maxParts {
		maxParts = len(minimumParts)
	}

	for i := 0; i < maxParts; i++ {
		actualPart := 0
		if i < len(actualParts) {
			actualPart = actualParts[i]
		}

		minimumPart := 0
		if i < len(minimumParts) {
			minimumPart = minimumParts[i]
		}

		if actualPart > minimumPart {
			return true
		}
		if actualPart < minimumPart {
			return false
		}
	}

	return true
}

func parseVersionParts(version string) []int {
	trimmed := strings.TrimSpace(version)
	segments := strings.Split(trimmed, ".")
	parts := make([]int, 0, len(segments))
	for _, segment := range segments {
		value, err := strconv.Atoi(segment)
		if err != nil {
			parts = append(parts, 0)
			continue
		}
		parts = append(parts, value)
	}
	return parts
}

func writeInventory(cfg *config, state *runtimeState) error {
	var builder strings.Builder
	initiationPort := cfg.SSHPort
	if initiationPort < 1 || initiationPort > 65535 {
		initiationPort = defaultSSHPort
	}

	usedAliases := make(map[string]int, len(cfg.Servers))
	builder.WriteString("all:\n")
	builder.WriteString("  vars:\n")
	builder.WriteString("    ansible_python_interpreter: auto_silent\n")
	builder.WriteString("  children:\n")
	builder.WriteString("    civa_targets:\n")
	builder.WriteString("      hosts:\n")

	for i, server := range cfg.Servers {
		alias := uniqueInventoryAlias(server, i+1, usedAliases)
		fmt.Fprintf(&builder, "        %s:\n", alias)
		fmt.Fprintf(&builder, "          ansible_host: %q\n", server.Address)
		fmt.Fprintf(&builder, "          ansible_user: %q\n", cfg.SSHUser)
		fmt.Fprintf(&builder, "          ansible_port: %d\n", initiationPort)
		if server.SSHPort >= 1 && server.SSHPort <= 65535 && server.SSHPort != initiationPort {
			fmt.Fprintf(&builder, "          civa_custom_ssh_port: %d\n", server.SSHPort)
		}
		if cfg.SSHAuthMethod == sshAuthMethodKey {
			fmt.Fprintf(&builder, "          ansible_ssh_private_key_file: %q\n", cfg.SSHPrivateKey)
		}
		if server.Hostname != "" {
			fmt.Fprintf(&builder, "          civa_target_hostname: %q\n", server.Hostname)
		}
	}

	return os.WriteFile(state.InventoryFile, []byte(builder.String()), 0o600)
}

func writeAuthFile(cfg *config, state *runtimeState) error {
	if state.AuthFile == "" {
		return nil
	}

	content := fmt.Sprintf("ansible_password: %q\n", cfg.SSHPassword)
	return os.WriteFile(state.AuthFile, []byte(content), 0o600)
}

func writePlanMetadata(cfg *config, state *runtimeState) error {
	metadata := plannedRunMetadata{
		WebServer:     cfg.WebServer,
		SSHAuthMethod: cfg.SSHAuthMethod,
		SSHUser:       cfg.SSHUser,
		SSHPort:       cfg.SSHPort,
		Components:    append([]string(nil), cfg.Components...),
		InventoryFile: state.InventoryFile,
		VarsFile:      state.VarsFile,
		AuthFile:      state.AuthFile,
		PlanFile:      state.PlanFile,
		PlaybookFile:  state.PlaybookFile,
	}

	content, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.WriteFile(state.MetadataFile, content, 0o600); err != nil {
		return err
	}
	if cfg.PlanFile != "" {
		if err := os.WriteFile(planMetadataPath(cfg.PlanFile), content, 0o600); err != nil {
			return err
		}
	}
	return writeLatestPlanPointer(state.PlanFile)
}

func writeVarsFile(cfg *config, state *runtimeState) error {
	content := fmt.Sprintf(
		"civa_deployer_user: %q\n"+
			"civa_public_key_path: %q\n"+
			"civa_web_server: %q\n"+
			"civa_timezone: %q\n"+
			"civa_swap_size: %q\n"+
			"civa_swap_file: %q\n"+
			"civa_traefik_email: %q\n"+
			"civa_traefik_challenge: %q\n"+
			"civa_traefik_dns_provider: %q\n",
		cfg.DeployUser,
		cfg.SSHPublicKey,
		cfg.WebServer,
		cfg.Timezone,
		cfg.SwapSize,
		"/swapfile",
		cfg.TraefikEmail,
		cfg.TraefikChallenge,
		cfg.TraefikDNSProvider,
	)
	return os.WriteFile(state.VarsFile, []byte(content), 0o644)
}

func writePlanFile(cfg *config, state *runtimeState) error {
	serverLines := make([]string, 0, len(cfg.Servers))
	for _, server := range cfg.Servers {
		if server.Hostname != "" {
			serverLines = append(serverLines, fmt.Sprintf("- %s -> %s", server.Address, server.Hostname))
		} else {
			serverLines = append(serverLines, fmt.Sprintf("- %s", server.Address))
		}
	}

	componentLines := make([]string, 0, len(cfg.Components))
	for _, component := range cfg.Components {
		componentLines = append(componentLines, fmt.Sprintf("- %s", componentLabel(component)))
	}

	content := fmt.Sprintf(
		"# civa Run Plan\n\n"+
			"## Mode\n\n"+
			"- Command: %s\n"+
			"- Web server: %s\n"+
			"- SSH auth method: %s\n"+
			"- SSH user: %s\n"+
			"- SSH port: %d\n"+
			"- SSH credential: %s\n"+
			"- SSH public key: %s\n"+
			"- Deployer user: %s\n"+
			"- Timezone: %s\n\n"+
			"## Selected Components\n\n%s\n\n"+
			"## Target Servers\n\n%s\n\n"+
			"## Generated Files\n\n"+
			"- Inventory: %s\n"+
			"- Vars: %s\n"+
			"- SSH auth file: %s\n"+
			"- Metadata: %s\n"+
			"- Plan: %s\n"+
			"- Playbook: %s\n\n"+
			"## Notes\n\n%s\n\n"+
			"## Command\n\n```bash\n%s\n```\n",
		cfg.Command,
		webServerLabel(cfg.WebServer),
		sshAuthMethodLabel(cfg.SSHAuthMethod),
		cfg.SSHUser,
		cfg.SSHPort,
		sshCredentialSummary(*cfg),
		cfg.SSHPublicKey,
		cfg.DeployUser,
		cfg.Timezone,
		strings.Join(componentLines, "\n"),
		strings.Join(serverLines, "\n"),
		state.InventoryFile,
		state.VarsFile,
		sshAuthFileSummary(*state),
		state.MetadataFile,
		state.PlanFile,
		state.PlaybookFile,
		sshAuthNote(*cfg, *state),
		buildAnsibleCommand(cfg, state),
	)

	if err := os.WriteFile(state.PlanFile, []byte(content), 0o644); err != nil {
		return err
	}
	if cfg.PlanFile != "" {
		if err := os.WriteFile(cfg.PlanFile, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func runAnsible(cfg *config, state *runtimeState) error {
	args := []string{
		"-i", state.InventoryFile,
		state.PlaybookFile,
		"-e", "@" + state.VarsFile,
	}
	if state.AuthFile != "" {
		args = append(args, "-e", "@"+state.AuthFile)
	}
	tags := selectedAnsibleTags(*cfg)
	if len(tags) > 0 {
		args = append(args, "--tags", strings.Join(tags, ","))
	}
	if shouldUseAnsibleCheckMode(*cfg) {
		args = append(args, "--check", "--diff")
	}

	cmd := exec.Command("ansible-playbook", args...)
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "ANSIBLE_ROLES_PATH="+filepath.Join(filepath.Dir(state.PlaybookFile), "roles"))

	progressController := newAnsibleProgressBarController(*cfg)
	if progressController == nil {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to capture ansible stdout: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to capture ansible stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	progressController.Start()

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}

	go func() {
		defer wg.Done()
		errCh <- captureCommandOutput(stdoutPipe, stdoutBuffer, progressController.ObserveOutputLine)
	}()
	go func() {
		defer wg.Done()
		errCh <- captureCommandOutput(stderrPipe, stderrBuffer, progressController.ObserveOutputLine)
	}()

	waitErr := cmd.Wait()
	progressController.Stop(waitErr == nil)
	wg.Wait()
	close(errCh)

	for streamErr := range errCh {
		if streamErr != nil {
			return streamErr
		}
	}

	if waitErr != nil {
		reportCapturedCommandOutput(stdoutBuffer, stderrBuffer)
	}

	return waitErr
}

type ansibleProgressBarController struct {
	program         *tea.Program
	doneCh          chan struct{}
	stopped         sync.Once
	desc            string
	mu              sync.Mutex
	discoveredTasks int
	completedTasks  int
	activeTask      bool
	currentTask     string
}

func newAnsibleProgressBarController(cfg config) *ansibleProgressBarController {
	if !shouldUseAnsibleProgressBar(cfg, term.IsTerminal(int(os.Stdout.Fd())), term.IsTerminal(int(os.Stderr.Fd()))) {
		return nil
	}

	return &ansibleProgressBarController{
		doneCh: make(chan struct{}),
		desc:   ansibleProgressDescription(cfg),
	}
}

func (c *ansibleProgressBarController) Start() {
	if c == nil {
		return
	}

	model := newAnsibleProgressModel(c.desc)
	program := tea.NewProgram(model, tea.WithOutput(os.Stderr), tea.WithInput(nil))
	c.program = program

	go func() {
		defer close(c.doneCh)
		_, _ = program.Run()
	}()
}

func (c *ansibleProgressBarController) Stop(success bool) {
	if c == nil {
		return
	}

	c.stopped.Do(func() {
		if success {
			c.finalizeSuccessProgress()
		}

		if c.program != nil {
			c.program.Send(ansibleProgressStopMsg{success: success})
		}
		<-c.doneCh
		if !success {
			_, _ = fmt.Fprint(os.Stderr, "\r\033[K\n")
		}
	})
}

func (c *ansibleProgressBarController) ObserveOutputLine(line string) {
	if c == nil {
		return
	}

	normalized := normalizeAnsibleOutputLine(line)
	if normalized == "" {
		return
	}

	if taskName, ok := parseAnsibleTaskHeader(normalized); ok {
		c.recordTaskStart(taskName)
		return
	}

	if isAnsiblePlayRecapHeader(normalized) {
		c.recordPlayRecap()
	}
}

func (c *ansibleProgressBarController) recordTaskStart(taskName string) {
	c.mu.Lock()
	if c.activeTask {
		c.completedTasks++
	}
	c.discoveredTasks++
	c.activeTask = true
	c.currentTask = taskName
	completed := c.completedTasks
	total := c.discoveredTasks
	c.mu.Unlock()

	c.sendProgressUpdate(completed, total, taskName)
}

func (c *ansibleProgressBarController) recordPlayRecap() {
	c.mu.Lock()
	if c.activeTask {
		c.completedTasks++
		c.activeTask = false
	}
	if c.discoveredTasks == 0 {
		c.discoveredTasks = 1
	}
	if c.completedTasks < c.discoveredTasks {
		c.completedTasks = c.discoveredTasks
	}
	completed := c.completedTasks
	total := c.discoveredTasks
	taskName := c.currentTask
	c.mu.Unlock()

	c.sendProgressUpdate(completed, total, taskName)
}

func (c *ansibleProgressBarController) finalizeSuccessProgress() {
	c.mu.Lock()
	if c.activeTask {
		c.completedTasks++
		c.activeTask = false
	}
	if c.discoveredTasks == 0 {
		c.discoveredTasks = 1
	}
	if c.completedTasks < c.discoveredTasks {
		c.completedTasks = c.discoveredTasks
	}
	completed := c.completedTasks
	total := c.discoveredTasks
	taskName := c.currentTask
	c.mu.Unlock()

	c.sendProgressUpdate(completed, total, taskName)
}

func (c *ansibleProgressBarController) sendProgressUpdate(completed, total int, taskName string) {
	if c.program == nil {
		return
	}

	c.program.Send(ansibleProgressUpdateMsg{
		completed: completed,
		total:     total,
		taskName:  taskName,
	})
}

var (
	ansiEscapePattern     = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	ansibleTaskHeaderLine = regexp.MustCompile(`^TASK \[(.+)\] \*+$`)
	ansiblePlayRecapLine  = regexp.MustCompile(`^PLAY RECAP \*+$`)
)

func normalizeAnsibleOutputLine(line string) string {
	withoutANSI := ansiEscapePattern.ReplaceAllString(line, "")
	withoutCarriage := strings.ReplaceAll(withoutANSI, "\r", "")
	return strings.TrimSpace(withoutCarriage)
}

func parseAnsibleTaskHeader(line string) (string, bool) {
	matches := ansibleTaskHeaderLine.FindStringSubmatch(line)
	if len(matches) != 2 {
		return "", false
	}

	task := strings.TrimSpace(matches[1])
	if task == "" {
		return "", false
	}

	return task, true
}

func isAnsiblePlayRecapHeader(line string) bool {
	return ansiblePlayRecapLine.MatchString(line)
}

type ansibleProgressUpdateMsg struct {
	completed int
	total     int
	taskName  string
}

type ansibleProgressStopMsg struct {
	success bool
}

type ansibleProgressModel struct {
	bar       progress.Model
	desc      string
	completed int
	total     int
	taskName  string
	done      bool
	success   bool
}

func newAnsibleProgressModel(desc string) ansibleProgressModel {
	bar := progress.New(progress.WithWidth(24))

	return ansibleProgressModel{
		bar:       bar,
		desc:      desc,
		completed: 0,
		total:     1,
	}
}

func (m ansibleProgressModel) Init() tea.Cmd {
	return nil
}

func (m ansibleProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ansibleProgressUpdateMsg:
		if m.done {
			return m, nil
		}
		m.completed = max(msg.completed, 0)
		m.total = max(msg.total, 1)
		m.taskName = strings.TrimSpace(msg.taskName)
		return m, nil
	case ansibleProgressStopMsg:
		m.done = true
		m.success = msg.success
		return m, tea.Quit
	}

	return m, nil
}

func (m ansibleProgressModel) View() string {
	if m.done {
		if m.success {
			return fmt.Sprintf("%s %s\n", m.desc, m.bar.ViewAs(1))
		}
		return ""
	}

	percentage := float64(m.completed) / float64(m.total)
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}

	taskSuffix := ""
	if m.taskName != "" {
		taskSuffix = " • " + m.taskName
	}

	return fmt.Sprintf("%s %s%s", m.desc, m.bar.ViewAs(percentage), taskSuffix)
}

func captureCommandOutput(source io.Reader, target io.Writer, onLine func(string)) error {
	scanner := bufio.NewScanner(source)
	buffer := make([]byte, 0, 64*1024)
	scanner.Buffer(buffer, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if _, err := target.Write([]byte(line + "\n")); err != nil {
			return err
		}
		if onLine != nil {
			onLine(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func ansibleProgressDescription(cfg config) string {
	if cfg.ApplyAction == applyActionReview {
		return "Reviewing server state"
	}
	return "Running ansible-playbook"
}

func shouldUseAnsibleProgressBar(cfg config, stdoutTTY, stderrTTY bool) bool {
	return cfg.Command == commandApply && stdoutTTY && stderrTTY
}

func reportCapturedCommandOutput(stdoutBuffer, stderrBuffer *bytes.Buffer) {
	stdoutContent := strings.TrimSpace(stdoutBuffer.String())
	stderrContent := strings.TrimSpace(stderrBuffer.String())

	if stderrContent != "" {
		fmt.Fprintln(os.Stderr, stderrContent)
	}
	if stdoutContent != "" {
		fmt.Fprintln(os.Stderr, stdoutContent)
	}
}

func syncSSHConfigAfterApply(cfg *config, state *runtimeState) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve home directory for SSH config sync: %w", err)
	}
	return syncSSHConfigAfterApplyInHome(cfg, state, homeDir)
}

func syncSSHConfigAfterApplyInHome(_ *config, state *runtimeState, homeDir string) error {
	if _, err := ensureUserCivaDirectoryInHome(homeDir); err != nil {
		return err
	}

	entries, err := parseSSHConfigHostsFromInventory(state.InventoryFile)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		return fmt.Errorf("failed to create SSH directory %s: %w", sshDir, err)
	}

	sshConfigPath := filepath.Join(sshDir, "config")
	existingContent, err := os.ReadFile(sshConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read SSH config %s: %w", sshConfigPath, err)
	}

	updatedContent := string(existingContent)
	for _, entry := range entries {
		updatedContent = upsertManagedSSHConfigEntry(updatedContent, entry)
	}

	if string(existingContent) == updatedContent {
		return nil
	}

	if err := os.WriteFile(sshConfigPath, []byte(updatedContent), 0o600); err != nil {
		return fmt.Errorf("failed to write SSH config %s: %w", sshConfigPath, err)
	}

	fmt.Fprintf(os.Stderr, "Updated SSH config with %d server entrie(s): %s\n", len(entries), sshConfigPath)
	return nil
}

func ensureUserCivaDirectoryInHome(homeDir string) (string, error) {
	return infssh.EnsureUserCivaDirectoryInHome(homeDir)
}

type sshConfigEntry struct {
	Alias        string
	HostName     string
	User         string
	Port         int
	IdentityFile string
}

func parseSSHConfigHostsFromInventory(inventoryPath string) ([]sshConfigEntry, error) {
	content, err := os.ReadFile(inventoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read inventory for SSH config sync %s: %w", inventoryPath, err)
	}

	lines := strings.Split(string(content), "\n")
	entries := []sshConfigEntry{}
	var current *sshConfigEntry

	flushCurrent := func() {
		if current == nil {
			return
		}
		if strings.TrimSpace(current.Alias) != "" && strings.TrimSpace(current.HostName) != "" {
			if current.Port == 0 {
				current.Port = defaultSSHPort
			}
			entries = append(entries, *current)
		}
		current = nil
	}

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r")

		if strings.HasPrefix(line, "        ") && !strings.HasPrefix(line, "          ") {
			trimmed := strings.TrimSpace(line)
			if strings.HasSuffix(trimmed, ":") {
				alias := strings.TrimSuffix(trimmed, ":")
				if alias != "hosts" && alias != "vars" && alias != "children" && alias != "all" && alias != "civa_targets" {
					flushCurrent()
					current = &sshConfigEntry{Alias: alias, Port: defaultSSHPort}
					continue
				}
			}
		}

		if current == nil {
			continue
		}

		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "ansible_host:"):
			current.HostName = parseInventoryScalarValue(strings.TrimPrefix(trimmed, "ansible_host:"))
		case strings.HasPrefix(trimmed, "ansible_user:"):
			current.User = parseInventoryScalarValue(strings.TrimPrefix(trimmed, "ansible_user:"))
		case strings.HasPrefix(trimmed, "ansible_port:"):
			portValue := parseInventoryScalarValue(strings.TrimPrefix(trimmed, "ansible_port:"))
			if parsedPort, convErr := strconv.Atoi(portValue); convErr == nil {
				current.Port = parsedPort
			}
		case strings.HasPrefix(trimmed, "civa_custom_ssh_port:"):
			portValue := parseInventoryScalarValue(strings.TrimPrefix(trimmed, "civa_custom_ssh_port:"))
			if parsedPort, convErr := strconv.Atoi(portValue); convErr == nil && parsedPort >= 1 && parsedPort <= 65535 {
				current.Port = parsedPort
			}
		case strings.HasPrefix(trimmed, "ansible_ssh_private_key_file:"):
			current.IdentityFile = parseInventoryScalarValue(strings.TrimPrefix(trimmed, "ansible_ssh_private_key_file:"))
		}
	}

	flushCurrent()

	if len(entries) == 0 {
		return nil, fmt.Errorf("no hosts found in inventory for SSH config sync: %s", inventoryPath)
	}

	return entries, nil
}

func parseInventoryScalarValue(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if unquoted, err := strconv.Unquote(trimmed); err == nil {
		return unquoted
	}
	return trimmed
}

func upsertManagedSSHConfigEntry(existing string, entry sshConfigEntry) string {
	cleaned := removeManagedSSHConfigEntry(existing, entry.Alias)
	if strings.TrimSpace(cleaned) != "" && !strings.HasSuffix(cleaned, "\n") {
		cleaned += "\n"
	}
	if strings.TrimSpace(cleaned) != "" {
		cleaned += "\n"
	}
	return cleaned + buildManagedSSHConfigEntry(entry)
}

func removeManagedSSHConfigEntry(content, alias string) string {
	startMarker := "# civa-managed-start " + alias
	endMarker := "# civa-managed-end " + alias

	for {
		start := strings.Index(content, startMarker)
		if start == -1 {
			break
		}

		endOffset := strings.Index(content[start:], endMarker)
		if endOffset == -1 {
			break
		}

		end := start + endOffset + len(endMarker)
		for end < len(content) && (content[end] == '\r' || content[end] == '\n') {
			end++
		}
		content = content[:start] + content[end:]
	}

	return strings.TrimRight(content, "\n")
}

func buildManagedSSHConfigEntry(entry sshConfigEntry) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# civa-managed-start %s\n", entry.Alias)
	fmt.Fprintf(&builder, "Host %s\n", entry.Alias)
	fmt.Fprintf(&builder, "  HostName %s\n", entry.HostName)
	if strings.TrimSpace(entry.User) != "" {
		fmt.Fprintf(&builder, "  User %s\n", entry.User)
	}
	if entry.Port > 0 {
		fmt.Fprintf(&builder, "  Port %d\n", entry.Port)
	}
	if strings.TrimSpace(entry.IdentityFile) != "" {
		fmt.Fprintf(&builder, "  IdentityFile %s\n", entry.IdentityFile)
		builder.WriteString("  IdentitiesOnly yes\n")
	}
	fmt.Fprintf(&builder, "# civa-managed-end %s\n", entry.Alias)
	return builder.String()
}

func buildAnsibleCommand(cfg *config, state *runtimeState) string {
	parts := []string{
		"ANSIBLE_ROLES_PATH=" + filepath.Join(filepath.Dir(state.PlaybookFile), "roles"),
		"ansible-playbook",
		"-i", state.InventoryFile,
		state.PlaybookFile,
		"-e", "@" + state.VarsFile,
	}
	if state.AuthFile != "" {
		parts = append(parts, "-e", "@"+state.AuthFile)
	}
	tags := selectedAnsibleTags(*cfg)
	if len(tags) > 0 {
		parts = append(parts, "--tags", strings.Join(tags, ","))
	}
	if shouldUseAnsibleCheckMode(*cfg) {
		parts = append(parts, "--check", "--diff")
	}

	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, shellQuote(part))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}

	needsQuoting := false
	for _, r := range value {
		if r == ' ' || r == '\t' || r == '\n' || r == '\'' || r == '"' || r == '\\' || r == '$' || r == '@' {
			needsQuoting = true
			break
		}
	}
	if !needsQuoting {
		return value
	}

	return strconv.Quote(value)
}

func shouldUseAnsibleCheckMode(cfg config) bool {
	return cfg.Command == commandPreview || cfg.ApplyAction == applyActionReview
}

func uniqueInventoryAlias(server serverSpec, index int, used map[string]int) string {
	baseAlias := sanitizeAlias(server.Hostname)
	if baseAlias == "" {
		baseAlias = fmt.Sprintf("server-%02d", index)
	}

	count := used[baseAlias]
	used[baseAlias] = count + 1
	if count == 0 {
		return baseAlias
	}

	return fmt.Sprintf("%s-%02d", baseAlias, count+1)
}

func detectPythonCommand() (string, error) {
	for _, candidate := range []string{"python", "python3"} {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("python or python3 not found")
}

func sanitizeAlias(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, " ", "-")
	var builder strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.' || r == '_' || r == '-':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}

func printSection(title string) {
	fmt.Fprintln(os.Stderr, renderSectionTitle(title, canStyleStderr()))
}

func logLine(message string) {
	if canStyleStderr() {
		fmt.Fprintln(os.Stderr, renderOutputBlock(outputBlock{Title: "Info", Lines: []string{message}}, true))
		return
	}
	fmt.Fprintln(os.Stderr, message)
}

func printConfigurationSummary(cfg *config) {
	printSection("Run Summary")
	style := canStyleStderr()
	configLines := []string{
		fmt.Sprintf("Command: %s", cfg.Command),
		fmt.Sprintf("Web server: %s", webServerLabel(cfg.WebServer)),
		fmt.Sprintf("SSH auth method: %s", sshAuthMethodLabel(cfg.SSHAuthMethod)),
		fmt.Sprintf("SSH user: %s", cfg.SSHUser),
		fmt.Sprintf("SSH port: %d", cfg.SSHPort),
		fmt.Sprintf("SSH credential: %s", sshCredentialSummary(*cfg)),
		fmt.Sprintf("SSH public key: %s", cfg.SSHPublicKey),
		fmt.Sprintf("Deployer user: %s", cfg.DeployUser),
		fmt.Sprintf("Timezone: %s", cfg.Timezone),
	}
	componentLines := make([]string, 0, len(cfg.Components))
	for _, component := range cfg.Components {
		componentLines = append(componentLines, fmt.Sprintf("- %s", componentLabel(component)))
	}
	serverLines := make([]string, 0, len(cfg.Servers))
	for _, server := range cfg.Servers {
		if server.Hostname != "" {
			serverLines = append(serverLines, fmt.Sprintf("- %s -> %s", server.Address, server.Hostname))
		} else {
			serverLines = append(serverLines, fmt.Sprintf("- %s", server.Address))
		}
	}
	blocks := []outputBlock{
		{Title: "Configuration", Lines: configLines},
		{Title: "Components", Lines: componentLines},
		{Title: "Servers", Lines: serverLines},
	}
	if cfg.WebServer == webServerTraefik {
		traefikLines := []string{
			fmt.Sprintf("Traefik ACME email: %s", cfg.TraefikEmail),
			fmt.Sprintf("Traefik challenge: %s", cfg.TraefikChallenge),
		}
		if cfg.TraefikChallenge == "dns" {
			traefikLines = append(traefikLines, fmt.Sprintf("Traefik DNS provider: %s", cfg.TraefikDNSProvider))
		}
		blocks = append(blocks, outputBlock{Title: "Traefik", Lines: traefikLines})
	}
	fmt.Fprintln(os.Stderr, renderOutputBlocks(blocks, style))
}

func showExecutionSummary(cfg *config, state *runtimeState) {
	printSection("Execution Summary")
	style := canStyleStderr()
	coreLines := []string{
		fmt.Sprintf("Command: %s", cfg.Command),
		fmt.Sprintf("Completed phases: %d/%d", state.ProgressCurrent, state.ProgressTotal),
		fmt.Sprintf("Result: %s", executionResultLabel(cfg)),
	}
	phaseLines := make([]string, 0, len(state.CompletedPhases))
	for _, phase := range state.CompletedPhases {
		phaseLines = append(phaseLines, fmt.Sprintf("- %s", phase))
	}
	artifactLines := []string{
		fmt.Sprintf("Inventory: %s", state.InventoryFile),
		fmt.Sprintf("Vars: %s", state.VarsFile),
		fmt.Sprintf("Plan: %s", state.PlanFile),
	}
	blocks := []outputBlock{
		{Title: "Overview", Lines: coreLines},
		{Title: "Phases Completed", Lines: phaseLines},
		{Title: "Artifacts", Lines: artifactLines},
	}
	if cfg.Command == commandApply && cfg.ApplyAction == applyActionReview {
		blocks = append(blocks, outputBlock{Title: "Review Details", Lines: applyReviewSummaryLines(cfg, state)})
	}
	fmt.Fprintln(os.Stderr, renderOutputBlocks(blocks, style))
}

func executionSummaryLines(cfg *config, state *runtimeState) []string {
	lines := []string{
		fmt.Sprintf("Command: %s", cfg.Command),
		fmt.Sprintf("Completed phases: %d/%d", state.ProgressCurrent, state.ProgressTotal),
		"Phases completed:",
	}

	for _, phase := range state.CompletedPhases {
		lines = append(lines, fmt.Sprintf("- %s", phase))
	}

	lines = append(lines,
		fmt.Sprintf("Inventory: %s", state.InventoryFile),
		fmt.Sprintf("Vars: %s", state.VarsFile),
		fmt.Sprintf("Plan: %s", state.PlanFile),
	)

	if cfg.Command == commandApply && cfg.ApplyAction == applyActionReview {
		lines = append(lines, applyReviewSummaryLines(cfg, state)...)
		return lines
	}

	switch cfg.Command {
	case commandPreview:
		lines = append(lines, "Result: preview completed with ansible check mode")
	case commandApply:
		lines = append(lines, "Result: apply completed")
	case commandPlan:
		lines = append(lines, "Result: plan generated without executing ansible-playbook")
	}

	return lines
}

func executionResultLabel(cfg *config) string {
	switch cfg.Command {
	case commandPreview:
		return "preview completed with ansible check mode"
	case commandApply:
		if cfg.ApplyAction == applyActionReview {
			return "apply review completed and current state was checked against the saved plan"
		}
		return "apply completed"
	case commandPlan:
		return "plan generated without executing ansible-playbook"
	default:
		return "completed"
	}
}

func applyReviewSummaryLines(cfg *config, state *runtimeState) []string {
	targetSummary := reviewTargetSummary(state.InventoryFile)
	tags := selectedAnsibleTags(*cfg)
	tagSummary := "all configured roles"
	if len(tags) > 0 {
		tagSummary = strings.Join(tags, ", ")
	}

	lines := []string{
		"Review mode: ansible-playbook executed with --check --diff (server changes were not applied)",
		fmt.Sprintf("Review scope: %s", reviewScopeSummary(*cfg)),
		fmt.Sprintf("Target hosts: %s", targetSummary),
		fmt.Sprintf("Selected ansible tags: %s", tagSummary),
		fmt.Sprintf("SSH auth mode: %s", reviewSSHAuthMode(*cfg, *state)),
		"Verification meaning: any reported diff or change indicates drift that a normal `civa apply` would try to reconcile.",
		"Result: apply review completed and the current server state was checked against the saved plan.",
	}

	if state.AuthFile != "" {
		lines = append(lines, fmt.Sprintf("Review auth file: %s", state.AuthFile))
	}

	return lines
}

func reviewTargetSummary(inventoryPath string) string {
	entries, err := parseSSHConfigHostsFromInventory(inventoryPath)
	if err != nil || len(entries) == 0 {
		return fmt.Sprintf("unable to summarize inventory targets from %s", inventoryPath)
	}

	aliases := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.TrimSpace(entry.Alias) != "" {
			aliases = append(aliases, entry.Alias)
			continue
		}
		aliases = append(aliases, entry.HostName)
	}

	return fmt.Sprintf("%d host(s): %s", len(entries), strings.Join(aliases, ", "))
}

func reviewScopeSummary(cfg config) string {
	components := make([]string, 0, len(cfg.Components))
	for _, component := range cfg.Components {
		if component == "web_server" {
			if isValidWebServer(cfg.WebServer) && cfg.WebServer != webServerNone {
				components = append(components, fmt.Sprintf("web_server(%s)", cfg.WebServer))
			}
			continue
		}
		components = append(components, component)
	}

	if len(components) == 0 {
		return "all planned components"
	}

	return strings.Join(components, ", ")
}

func reviewSSHAuthMode(cfg config, state runtimeState) string {
	if cfg.SSHAuthMethod == sshAuthMethodPassword && state.AuthFile != "" {
		return fmt.Sprintf("password auth via %s", state.AuthFile)
	}

	if cfg.SSHAuthMethod == sshAuthMethodPassword {
		return "password auth"
	}

	return "SSH key auth"
}

func (state *runtimeState) progressStep(label string) {
	state.ProgressCurrent++
	fmt.Printf("[%d/%d] %s\n", state.ProgressCurrent, state.ProgressTotal, label)
}

func (state *runtimeState) appendCompletedPhase(phase string) {
	state.CompletedPhases = append(state.CompletedPhases, phase)
}

func sshCredentialSummary(cfg config) string {
	if cfg.SSHAuthMethod == sshAuthMethodPassword {
		return "[hidden password]"
	}

	return cfg.SSHPrivateKey
}

func sshAuthFileSummary(state runtimeState) string {
	if state.AuthFile == "" {
		return "not used"
	}

	return state.AuthFile
}

func sshAuthNote(cfg config, state runtimeState) string {
	if cfg.SSHAuthMethod == sshAuthMethodPassword {
		return fmt.Sprintf("- Password auth stores the SSH secret in %s with mode 0600 so Ansible can connect without exposing it in the Markdown plan.", state.AuthFile)
	}

	return "- SSH key auth uses the configured private key path directly from the generated inventory."
}
