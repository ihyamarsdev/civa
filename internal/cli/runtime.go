package cli

import (
	"bufio"
	ansible "civa/ansible"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

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

func resolvePlanInputFile(cfg *config) (string, error) {
	if err := finalizePaths(cfg); err != nil {
		return "", err
	}

	if strings.TrimSpace(cfg.PlanName) != "" {
		planPath := planPathForName(cfg.PlanName)
		if _, err := os.Stat(planPath); err != nil {
			return "", fmt.Errorf("generated plan not found: %s", cfg.PlanName)
		}
		cfg.PlanInputFile = planPath
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
	content, err := os.ReadFile(latestPlanPointerFile)
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
	if err := os.MkdirAll(filepath.Dir(latestPlanPointerFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(latestPlanPointerFile, []byte(planPath+"\n"), 0o644)
}

func planPathForName(name string) string {
	return filepath.Join(runRootDirectory, name, "plan.md")
}

func planDirForName(name string) string {
	return filepath.Join(runRootDirectory, name)
}

func listPlans() error {
	entries, err := os.ReadDir(runRootDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No generated plans found.")
			return nil
		}
		return err
	}

	latestPlan, _ := readLatestPlanPointer()
	printed := false
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsDir() {
			continue
		}
		planName := entry.Name()
		planPath := planPathForName(planName)
		if _, err := os.Stat(planPath); err != nil {
			continue
		}

		marker := ""
		if latestPlan == planPath {
			marker = " (latest)"
		}
		fmt.Printf("- %s%s\n  %s\n", planName, marker, planPath)
		printed = true
	}

	if !printed {
		fmt.Println("No generated plans found.")
	}
	return nil
}

func removePlan(cfg *config) error {
	targetDir := planDirForName(cfg.PlanName)
	planPath := planPathForName(cfg.PlanName)
	if _, err := os.Stat(planPath); err != nil {
		return fmt.Errorf("generated plan not found: %s", cfg.PlanName)
	}

	if !cfg.AssumeYes {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("non-interactive plan removal requires --yes")
		}

		confirmed, err := promptPlanRemovalConfirmation(cfg.PlanName)
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
		return fmt.Errorf("failed to remove plan %s: %w", cfg.PlanName, err)
	}

	if latestPlan, err := readLatestPlanPointer(); err == nil && latestPlan == planPath {
		if nextLatest, nextErr := latestGeneratedPlanFile(); nextErr == nil {
			if writeErr := writeLatestPlanPointer(nextLatest); writeErr != nil {
				return writeErr
			}
		} else {
			_ = os.Remove(latestPlanPointerFile)
		}
	}

	fmt.Printf("Removed generated plan %s\n", cfg.PlanName)
	return nil
}

func latestGeneratedPlanFile() (string, error) {
	entries, err := os.ReadDir(runRootDirectory)
	if err != nil {
		return "", err
	}

	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(runRootDirectory, entry.Name(), "plan.md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("no generated plan found under %s", runRootDirectory)
}

func loadPlannedRun(planPath string) (*config, *runtimeState, error) {
	metadata, err := loadPlannedRunMetadata(planPath)
	if err == nil {
		return metadataToPlannedRun(metadata)
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

func metadataToPlannedRun(metadata *plannedRunMetadata) (*config, *runtimeState, error) {
	loadedCfg := defaultConfig(commandApply)
	loadedCfg.WebServer = metadata.WebServer
	loadedCfg.SSHAuthMethod = metadata.SSHAuthMethod
	loadedCfg.SSHUser = metadata.SSHUser
	loadedCfg.SSHPort = metadata.SSHPort
	loadedCfg.Components = append([]string(nil), metadata.Components...)

	state := &runtimeState{
		InventoryFile: metadata.InventoryFile,
		VarsFile:      metadata.VarsFile,
		AuthFile:      metadata.AuthFile,
		MetadataFile:  planMetadataPath(metadata.PlanFile),
		PlanFile:      metadata.PlanFile,
		PlaybookFile:  metadata.PlaybookFile,
	}

	if state.InventoryFile == "" || state.VarsFile == "" || state.PlaybookFile == "" || state.PlanFile == "" {
		return nil, nil, fmt.Errorf("plan metadata is missing execution artifacts: %s", metadata.PlanFile)
	}
	if len(loadedCfg.Components) == 0 {
		return nil, nil, fmt.Errorf("plan metadata is missing selected components: %s", metadata.PlanFile)
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
	target := fmt.Sprintf("%s@%s", cfg.SSHUser, cfg.Servers[0].Address)
	if err := rotateKnownHostsFile(); err != nil {
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

func rotateKnownHostsFile() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to resolve home directory for known_hosts rotation: %w", err)
	}
	return rotateKnownHostsFileInHome(homeDir)
}

func rotateKnownHostsFileInHome(homeDir string) error {
	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")
	knownHostsOldPath := filepath.Join(homeDir, ".ssh", "known_hosts.old")

	if _, err := os.Stat(knownHostsPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to inspect %s: %w", knownHostsPath, err)
	}

	if err := os.Rename(knownHostsPath, knownHostsOldPath); err != nil {
		return fmt.Errorf("failed to move %s to %s: %w", knownHostsPath, knownHostsOldPath, err)
	}

	fmt.Fprintf(os.Stderr, "Moved %s to %s before running ssh-copy-id\n", knownHostsPath, knownHostsOldPath)
	return nil
}

func buildSSHCopyIDCommand(cfg config) *exec.Cmd {
	target := fmt.Sprintf("%s@%s", cfg.SSHUser, cfg.Servers[0].Address)
	args := []string{
		"-i", cfg.SSHPublicKey,
		"-p", strconv.Itoa(cfg.SSHPort),
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
	failures := 0

	printSection("civa Doctor")
	fmt.Printf("Version: %s\n", version)

	if _, err := exec.LookPath("ansible-playbook"); err == nil {
		fmt.Println("[ok] ansible-playbook found")
	} else {
		fmt.Println("[fail] ansible-playbook not found")
		failures++
	}

	if pythonCommand, err := detectPythonCommand(); err == nil {
		fmt.Printf("[ok] %s found\n", pythonCommand)
	} else {
		fmt.Println("[fail] python or python3 not found")
		failures++
	}

	if ansible.HasEmbeddedPlaybook() {
		fmt.Println("[ok] embedded Ansible playbook available")
	} else {
		fmt.Println("[fail] embedded Ansible playbook missing")
		failures++
	}

	if ansible.HasEmbeddedTemplate("collections/ansible_collections/civa/traefik/roles/traefik/templates/traefik-compose.yml.j2") {
		fmt.Println("[ok] embedded Traefik compose template available")
	} else {
		fmt.Println("[fail] embedded Traefik compose template missing")
		failures++
	}

	if ansible.HasEmbeddedTemplate("collections/ansible_collections/civa/security_firewall/roles/security_firewall/templates/fail2ban-jail.local.j2") {
		fmt.Println("[ok] embedded Fail2Ban template available")
	} else {
		fmt.Println("[fail] embedded Fail2Ban template missing")
		failures++
	}

	if cfg.SSHAuthMethod == sshAuthMethodKey {
		if _, err := os.Stat(cfg.SSHPrivateKey); err == nil {
			fmt.Printf("[ok] SSH private key found: %s\n", cfg.SSHPrivateKey)
		} else {
			fmt.Printf("[fail] SSH private key not found: %s\n", cfg.SSHPrivateKey)
			failures++
		}
	} else {
		if _, err := exec.LookPath("sshpass"); err == nil {
			fmt.Println("[ok] sshpass found for password-based SSH")
		} else {
			fmt.Println("[fail] sshpass not found for password-based SSH")
			failures++
		}
		if strings.TrimSpace(cfg.SSHPassword) != "" {
			fmt.Println("[ok] SSH password provided")
		} else {
			fmt.Println("[fail] SSH password not provided")
			failures++
		}
	}

	if _, err := os.Stat(cfg.SSHPublicKey); err == nil {
		fmt.Printf("[ok] SSH public key found: %s\n", cfg.SSHPublicKey)
	} else {
		fmt.Printf("[fail] SSH public key not found: %s\n", cfg.SSHPublicKey)
		failures++
	}

	if failures > 0 {
		return fmt.Errorf("doctor summary: %d blocker(s) found", failures)
	}

	fmt.Println("Doctor summary: local machine looks ready to run civa")
	return nil
}

func writeInventory(cfg *config, state *runtimeState) error {
	var builder strings.Builder
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
		fmt.Fprintf(&builder, "          ansible_port: %d\n", cfg.SSHPort)
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
	if cfg.Command == commandPreview {
		args = append(args, "--check", "--diff")
	}

	cmd := exec.Command("ansible-playbook", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(os.Environ(), "ANSIBLE_COLLECTIONS_PATH="+filepath.Join(filepath.Dir(state.PlaybookFile), "collections"))
	return cmd.Run()
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
	userCivaDir := filepath.Join(homeDir, ".civa")
	if err := os.MkdirAll(userCivaDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create user civa directory %s: %w", userCivaDir, err)
	}
	return userCivaDir, nil
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
		"ANSIBLE_COLLECTIONS_PATH=" + filepath.Join(filepath.Dir(state.PlaybookFile), "collections"),
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
	if cfg.Command == commandPreview {
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
	fmt.Fprintf(os.Stderr, "\n== %s ==\n", title)
	fmt.Fprintln(os.Stderr, "----------------------------------------")
}

func logLine(message string) {
	fmt.Fprintln(os.Stderr, message)
}

func printConfigurationSummary(cfg *config) {
	printSection("Run Summary")
	fmt.Fprintf(os.Stderr, "Command: %s\n", cfg.Command)
	fmt.Fprintf(os.Stderr, "Web server: %s\n", webServerLabel(cfg.WebServer))
	fmt.Fprintf(os.Stderr, "SSH auth method: %s\n", sshAuthMethodLabel(cfg.SSHAuthMethod))
	fmt.Fprintf(os.Stderr, "SSH user: %s\n", cfg.SSHUser)
	fmt.Fprintf(os.Stderr, "SSH port: %d\n", cfg.SSHPort)
	fmt.Fprintf(os.Stderr, "SSH credential: %s\n", sshCredentialSummary(*cfg))
	fmt.Fprintf(os.Stderr, "SSH public key: %s\n", cfg.SSHPublicKey)
	fmt.Fprintf(os.Stderr, "Deployer user: %s\n", cfg.DeployUser)
	fmt.Fprintf(os.Stderr, "Timezone: %s\n", cfg.Timezone)
	fmt.Fprintln(os.Stderr, "Components:")
	for _, component := range cfg.Components {
		fmt.Fprintf(os.Stderr, "- %s\n", componentLabel(component))
	}
	fmt.Fprintln(os.Stderr, "Servers:")
	for _, server := range cfg.Servers {
		if server.Hostname != "" {
			fmt.Fprintf(os.Stderr, "- %s -> %s\n", server.Address, server.Hostname)
		} else {
			fmt.Fprintf(os.Stderr, "- %s\n", server.Address)
		}
	}
	if cfg.WebServer == webServerTraefik {
		fmt.Fprintf(os.Stderr, "Traefik ACME email: %s\n", cfg.TraefikEmail)
		fmt.Fprintf(os.Stderr, "Traefik challenge: %s\n", cfg.TraefikChallenge)
		if cfg.TraefikChallenge == "dns" {
			fmt.Fprintf(os.Stderr, "Traefik DNS provider: %s\n", cfg.TraefikDNSProvider)
		}
	}
}

func showExecutionSummary(cfg *config, state *runtimeState) {
	printSection("Execution Summary")
	fmt.Fprintf(os.Stderr, "Command: %s\n", cfg.Command)
	fmt.Fprintf(os.Stderr, "Completed phases: %d/%d\n", state.ProgressCurrent, state.ProgressTotal)
	fmt.Fprintln(os.Stderr, "Phases completed:")
	for _, phase := range state.CompletedPhases {
		fmt.Fprintf(os.Stderr, "- %s\n", phase)
	}
	fmt.Fprintf(os.Stderr, "Inventory: %s\n", state.InventoryFile)
	fmt.Fprintf(os.Stderr, "Vars: %s\n", state.VarsFile)
	fmt.Fprintf(os.Stderr, "Plan: %s\n", state.PlanFile)
	switch cfg.Command {
	case commandPreview:
		fmt.Fprintln(os.Stderr, "Result: preview completed with ansible check mode")
	case commandApply:
		fmt.Fprintln(os.Stderr, "Result: apply completed")
	case commandPlan:
		fmt.Fprintln(os.Stderr, "Result: plan generated without executing ansible-playbook")
	}
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
