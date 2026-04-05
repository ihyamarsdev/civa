package infra

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
	glowutils "github.com/charmbracelet/glow/v2/utils"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const version = "1.1.10"

const (
	commandApply            = "apply"
	commandPlan             = "plan"
	commandCompletion       = "completion"
	commandCompleteInternal = "__complete"
	commandDoctor           = "doctor"
	commandSetup            = "setup"
	commandTools            = "tools"
	commandToolsCloudflare  = "tools-cloudflare"
	commandSecret           = "secret"
	commandConfig           = "config"
	commandConfigNginxHelp  = "config-nginx"
	commandConfigCaddyHelp  = "config-caddy"
	commandConfigAllHelp    = "config-all"
	commandUninstall        = "uninstall"
	commandVersion          = "version"
	commandHelp             = "help"
	userCivaHomeDirectory   = "~/.civa"

	defaultSSHUser            = "root"
	defaultSSHPort            = 22
	defaultSSHAuthMethod      = sshAuthMethodKey
	defaultWebServer          = webServerNone
	defaultSSHPrivateKey      = "~/.ssh/id_ed25519"
	defaultSSHPublicKey       = "~/.ssh/id_ed25519.pub"
	defaultDeployUser         = "deployer"
	defaultTimezone           = "Asia/Jakarta"
	defaultSwapSize           = "2G"
	defaultTraefikChallenge   = "http"
	defaultTraefikDNSProvider = "cloudflare"
	planActionInit            = "init"
	planActionReview          = "review"
	planActionEdit            = "edit"
	planActionList            = "list"
	planActionRemove          = "remove"
	configActionInit          = "init"
	configActionEdit          = "edit"
	configActionList          = "list"
	configActionRemove        = "remove"
	configProfileAll          = "all"
	applyActionExecute        = "execute"
	applyActionReview         = "review"
	applyActionDrift          = "drift"
	applyActionRollback       = "rollback"
	secretActionSet           = "set"
	secretActionList          = "list"
	secretActionRemove        = "remove"
	toolsProviderCloudflare   = "cloudflare"
	toolsActionCloudflareZone = "zones"
	doctorActionCheck         = "check"
	doctorActionFix           = "fix"
)

type serverSpec struct {
	Address  string
	Hostname string
	SSHPort  int
}

type webServerSiteSpec struct {
	ServerName   string
	UpstreamHost string
	UpstreamPort int
	EnableHTTPS  bool
}

type webServerProfileConfig struct {
	Sites             []webServerSiteSpec `json:"sites"`
	InstallHostnames  []string            `json:"installHostnames,omitempty"`
	NginxCertbotEmail string              `json:"nginxCertbotEmail,omitempty"`
}

type persistedWebServerConfig struct {
	Version int                    `json:"version"`
	Nginx   webServerProfileConfig `json:"nginx"`
	Caddy   webServerProfileConfig `json:"caddy"`
}

type componentOption struct {
	Number      int
	Value       string
	Label       string
	Description string
}

type providedFlags struct {
	SSHUser            bool
	SSHPort            bool
	SSHAuthMethod      bool
	SSHPassword        bool
	SSHPasswordSecret  bool
	WebServer          bool
	SSHPrivateKey      bool
	SSHPublicKey       bool
	DeployUser         bool
	Timezone           bool
	Components         bool
	PlanInputFile      bool
	PlanFile           bool
	TraefikEmail       bool
	TraefikChallenge   bool
	TraefikDNSProvider bool
	Servers            bool
	NonInteractive     bool
	SecretValue        bool
	SecretValueFile    bool
	CloudflareToken    bool
}

type config struct {
	Command              string
	PlanAction           string
	ConfigAction         string
	ApplyAction          string
	SecretAction         string
	ToolsProvider        string
	ToolsAction          string
	DoctorAction         string
	PlanName             string
	AssumeYes            bool
	NonInteractive       bool
	SSHUser              string
	SSHPort              int
	SSHAuthMethod        string
	SSHPassword          string
	SSHPasswordSecret    string
	WebServer            string
	SSHPrivateKey        string
	SSHPublicKey         string
	DeployUser           string
	Timezone             string
	SwapSize             string
	TraefikEmail         string
	TraefikChallenge     string
	TraefikDNSProvider   string
	ComponentsInput      string
	Components           []string
	PlanInputFile        string
	PlanFile             string
	SecretName           string
	SecretValue          string
	SecretValueFile      string
	CloudflareToken      string
	WebServerSites       []webServerSiteSpec
	WebServerTargetHosts []string
	NginxCertbotEmail    string
	Servers              []serverSpec
	Provided             providedFlags
}

type runtimeState struct {
	RunID           string
	GeneratedDir    string
	InventoryFile   string
	VarsFile        string
	AuthFile        string
	MetadataFile    string
	PlanFile        string
	PlaybookFile    string
	ProgressCurrent int
	ProgressTotal   int
	CompletedPhases []string
}

var componentOptions = []componentOption{
	{Number: 1, Value: "system_update", Label: "System Update & Upgrade", Description: "Update package metadata and upgrade packages on Debian or RHEL family targets"},
	{Number: 2, Value: "user_management", Label: "User Management", Description: "Create deployer user, sudoers, and SSH key"},
	{Number: 3, Value: "ssh_hardening", Label: "SSH Hardening", Description: "Disable root login and password authentication"},
	{Number: 4, Value: "security_firewall", Label: "Security & Firewall", Description: "Set up UFW or firewalld plus Fail2Ban"},
	{Number: 5, Value: "system_config", Label: "System Config", Description: "Apply timezone and swap configuration"},
	{Number: 6, Value: "dependencies", Label: "Dependencies", Description: "Install git, curl, wget, htop, vim, unzip, jq, and net-tools"},
	{Number: 7, Value: "containerization", Label: "Containerization", Description: "Install Docker Engine and Compose plugin"},
	{Number: 8, Value: "web_server", Label: "Web Server", Description: "Prepare Traefik, Nginx, or Caddy"},
}

func defaultConfig(command string) config {
	return config{
		Command:            command,
		PlanAction:         planActionInit,
		ConfigAction:       configActionInit,
		ApplyAction:        applyActionExecute,
		SecretAction:       secretActionList,
		DoctorAction:       doctorActionCheck,
		SSHUser:            defaultSSHUser,
		SSHPort:            defaultSSHPort,
		SSHAuthMethod:      defaultSSHAuthMethod,
		WebServer:          defaultWebServer,
		SSHPrivateKey:      defaultSSHPrivateKey,
		SSHPublicKey:       defaultSSHPublicKey,
		DeployUser:         defaultDeployUser,
		Timezone:           defaultTimezone,
		SwapSize:           defaultSwapSize,
		TraefikChallenge:   defaultTraefikChallenge,
		TraefikDNSProvider: defaultTraefikDNSProvider,
		ComponentsInput:    "all",
	}
}

func runPlanListFlow(cfg *config) error {
	return listPlans(cfg.PlanName)
}

func runPlanRemoveFlow(cfg *config) error {
	if strings.TrimSpace(cfg.PlanName) == "" {
		return fmt.Errorf("plan remove requires a generated plan name")
	}
	return removePlan(cfg)
}

func runSetupFlow(cfg *config) error {
	if shouldPrompt(cfg) {
		if err := collectSetupInputs(cfg); err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
	}

	if err := finalizePaths(cfg); err != nil {
		return err
	}
	if err := resolveSetupSecretPassword(cfg); err != nil {
		return err
	}
	if err := validateSetupConfig(cfg); err != nil {
		return err
	}

	printSection("Setup Summary")
	fmt.Fprintf(os.Stderr, "Server: %s\n", cfg.Servers[0].Address)
	fmt.Fprintf(os.Stderr, "SSH user: %s\n", cfg.SSHUser)
	fmt.Fprintf(os.Stderr, "SSH port: %d\n", cfg.SSHPort)
	fmt.Fprintf(os.Stderr, "SSH public key: %s\n", cfg.SSHPublicKey)
	if strings.TrimSpace(cfg.SSHPassword) == "" {
		fmt.Fprintln(os.Stderr, "Password source: ssh-copy-id will prompt for the server password")
	} else {
		fmt.Fprintln(os.Stderr, "Password source: provided via sshpass")
	}

	return runSSHCopyID(*cfg)
}

func resolveSetupSecretPassword(cfg *config) error {
	secretName := strings.TrimSpace(cfg.SSHPasswordSecret)
	if secretName == "" {
		return nil
	}
	if strings.TrimSpace(cfg.SSHPassword) != "" {
		return fmt.Errorf("--ssh-password and --ssh-password-secret cannot be used together")
	}

	value, err := readSecretValue(secretName)
	if err != nil {
		return fmt.Errorf("resolve setup password from secret %q: %w", secretName, err)
	}
	cfg.SSHPassword = value
	return nil
}

func runSecretFlow(cfg *config) error {
	switch cfg.SecretAction {
	case secretActionSet:
		if strings.TrimSpace(cfg.SecretName) == "" {
			return fmt.Errorf("civa secret set requires a secret name")
		}
		secretValue, err := resolveSecretSetValue(cfg)
		if err != nil {
			return err
		}
		if err := writeSecretValue(cfg.SecretName, secretValue); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "✅ Secret %q stored in encrypted secret store\n", strings.TrimSpace(cfg.SecretName))
		return nil
	case secretActionRemove:
		if strings.TrimSpace(cfg.SecretName) == "" {
			return fmt.Errorf("civa secret remove requires a secret name")
		}
		removed, err := removeSecretValue(cfg.SecretName)
		if err != nil {
			return err
		}
		if !removed {
			fmt.Fprintf(os.Stderr, "Secret %q was not found\n", strings.TrimSpace(cfg.SecretName))
			return nil
		}
		fmt.Fprintf(os.Stderr, "✅ Secret %q removed\n", strings.TrimSpace(cfg.SecretName))
		return nil
	case secretActionList, "":
		names, err := listSecretNames()
		if err != nil {
			return err
		}
		if len(names) == 0 {
			fmt.Fprintln(os.Stderr, "No secrets stored. Use `civa secret set <name> --value-file <path>`.")
			return nil
		}
		printSection("Stored Secrets")
		for _, name := range names {
			fmt.Fprintf(os.Stderr, "- %s\n", name)
		}
		return nil
	default:
		return fmt.Errorf("unknown secret action: %s", cfg.SecretAction)
	}
}

func resolveSecretSetValue(cfg *config) (string, error) {
	if cfg.Provided.SecretValue && cfg.Provided.SecretValueFile {
		return "", fmt.Errorf("civa secret set accepts either --value or --value-file, not both")
	}

	if cfg.Provided.SecretValueFile {
		path := strings.TrimSpace(cfg.SecretValueFile)
		if path == "" {
			return "", fmt.Errorf("civa secret set requires a non-empty --value-file path")
		}
		expandedPath, err := expandHomePath(path)
		if err != nil {
			return "", fmt.Errorf("expand --value-file path: %w", err)
		}
		content, err := os.ReadFile(expandedPath)
		if err != nil {
			return "", fmt.Errorf("read --value-file: %w", err)
		}
		value := strings.TrimRight(string(content), "\r\n")
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("secret value from --value-file must not be empty")
		}
		return value, nil
	}

	if cfg.Provided.SecretValue {
		if strings.TrimSpace(cfg.SecretValue) == "" {
			return "", fmt.Errorf("secret value from --value must not be empty")
		}
		return cfg.SecretValue, nil
	}

	if shouldPrompt(cfg) {
		return promptHiddenSecretValue()
	}

	return "", fmt.Errorf("civa secret set requires --value or --value-file")
}

func promptHiddenSecretValue() (string, error) {
	value, err := promptSecretValue("Secret value")
	if err != nil {
		return "", fmt.Errorf("read secret value: %w", err)
	}
	return value, nil
}

func runConfigFlow(cfg *config) error {
	switch cfg.ConfigAction {
	case configActionList:
		return runConfigListFlow(cfg)
	case configActionRemove:
		return runConfigRemoveFlow(cfg)
	case configActionInit, configActionEdit, "":
		return runConfigInitFlow(cfg)
	default:
		return runConfigInitFlow(cfg)
	}
}

func runConfigInitFlow(cfg *config) error {
	if !shouldPrompt(cfg) {
		return fmt.Errorf("civa config <provider> init currently requires an interactive terminal")
	}

	store, err := loadWebServerConfig()
	if err != nil {
		return err
	}

	printSection("civa Config")
	logLine("Configure persisted web server settings. Plan init will only install web server components.")

	profile, err := normalizeConfigProfileTarget(cfg.WebServer)
	if err != nil {
		return err
	}

	targetWebServer := profile
	if targetWebServer == "" {
		targetWebServer, err = promptConfigWebServerTarget(webServerNginx)
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
	}
	if targetWebServer == configProfileAll {
		return fmt.Errorf("config init requires provider nginx or caddy")
	}

	currentProfile := store.Caddy
	if targetWebServer == webServerNginx {
		currentProfile = store.Nginx
	}

	updatedProfile, err := promptWebServerProfileConfig(targetWebServer, currentProfile)
	if err != nil {
		if errors.Is(err, errUserCancelled) {
			return nil
		}
		return err
	}

	if targetWebServer == webServerNginx {
		store.Nginx = updatedProfile
	} else {
		store.Caddy = updatedProfile
	}

	if err := saveWebServerConfig(store); err != nil {
		return err
	}

	defaultPlanName := ""
	if latestPath, err := readLatestPlanPointer(); err == nil {
		defaultPlanName = filepath.Base(filepath.Dir(latestPath))
	}
	if strings.TrimSpace(cfg.PlanName) == "" {
		planName, err := promptConfigPlanName(defaultPlanName)
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
		cfg.PlanName = strings.TrimSpace(planName)
	}

	planPath, err := resolveConfigPlanInputFile(cfg.PlanName)
	if err != nil {
		return err
	}

	loadedPlanCfg, plannedState, err := loadPlannedRun(planPath)
	if err != nil {
		return err
	}

	runtimeID := fmt.Sprintf("config-%s", generateRunID(time.Now()))
	generatedDir := filepath.Join(runRootDirectoryPath(), runtimeID)
	ansibleDir := filepath.Join(generatedDir, "ansible")
	varsFile := filepath.Join(generatedDir, "vars.yml")
	state := &runtimeState{
		RunID:         runtimeID,
		GeneratedDir:  generatedDir,
		InventoryFile: plannedState.InventoryFile,
		VarsFile:      varsFile,
		AuthFile:      plannedState.AuthFile,
		PlanFile:      planPath,
		PlaybookFile:  filepath.Join(ansibleDir, "config.yml"),
		ProgressTotal: 1,
	}

	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		return err
	}
	if err := materializeAnsibleAssets(ansibleDir); err != nil {
		return err
	}
	if err := writeConfigPlaybook(state.PlaybookFile); err != nil {
		return err
	}

	executionCfg := defaultConfig(commandConfig)
	executionCfg.WebServer = targetWebServer
	executionCfg.Components = []string{"web_server"}
	executionCfg.WebServerSites = append([]webServerSiteSpec(nil), updatedProfile.Sites...)
	executionCfg.WebServerTargetHosts = append([]string(nil), normalizeHostnameList(updatedProfile.InstallHostnames)...)
	executionCfg.NginxCertbotEmail = strings.TrimSpace(updatedProfile.NginxCertbotEmail)
	if loadedPlanCfg.SSHAuthMethod != "" {
		executionCfg.SSHAuthMethod = loadedPlanCfg.SSHAuthMethod
	}
	if strings.TrimSpace(loadedPlanCfg.SSHUser) != "" {
		executionCfg.SSHUser = loadedPlanCfg.SSHUser
	}
	if loadedPlanCfg.SSHPort > 0 {
		executionCfg.SSHPort = loadedPlanCfg.SSHPort
	}
	if strings.TrimSpace(loadedPlanCfg.DeployUser) != "" {
		executionCfg.DeployUser = loadedPlanCfg.DeployUser
	}
	if strings.TrimSpace(loadedPlanCfg.SSHPublicKey) != "" {
		executionCfg.SSHPublicKey = loadedPlanCfg.SSHPublicKey
	}
	if strings.TrimSpace(loadedPlanCfg.Timezone) != "" {
		executionCfg.Timezone = loadedPlanCfg.Timezone
	}

	if err := validateWebServerRuntimeConfig(&executionCfg); err != nil {
		return err
	}
	if err := writeVarsFile(&executionCfg, state); err != nil {
		return err
	}

	printSection("Config Saved")
	fmt.Fprintf(os.Stderr, "Web server profile: %s\n", webServerLabel(targetWebServer))
	fmt.Fprintf(os.Stderr, "Configured sites: %d\n", len(updatedProfile.Sites))
	if len(updatedProfile.InstallHostnames) == 0 {
		fmt.Fprintln(os.Stderr, "Install targets: all hostnames")
	} else {
		fmt.Fprintf(os.Stderr, "Install targets: %s\n", strings.Join(updatedProfile.InstallHostnames, ", "))
	}
	if targetWebServer == webServerNginx {
		if hasHTTPSWebServerSites(updatedProfile.Sites) {
			fmt.Fprintln(os.Stderr, "HTTPS mode: enabled via certbot")
		} else {
			fmt.Fprintln(os.Stderr, "HTTPS mode: disabled")
		}
	}

	printSection("Config Apply")
	fmt.Fprintf(os.Stderr, "Source plan: %s\n", planPath)
	fmt.Fprintf(os.Stderr, "Inventory: %s\n", state.InventoryFile)

	if err := runAnsible(&executionCfg, state); err != nil {
		return err
	}

	showExecutionSummary(&executionCfg, state)

	return nil
}

func runConfigListFlow(cfg *config) error {
	store, err := loadWebServerConfig()
	if err != nil {
		return err
	}

	profile, err := normalizeConfigProfileTarget(cfg.WebServer)
	if err != nil {
		return err
	}

	printSection("Configured Web Server Profiles")
	if profile == webServerNginx {
		printConfigProfileSummary(webServerNginx, store.Nginx)
		return nil
	}
	if profile == webServerCaddy {
		printConfigProfileSummary(webServerCaddy, store.Caddy)
		return nil
	}

	printConfigProfileSummary(webServerNginx, store.Nginx)
	printConfigProfileSummary(webServerCaddy, store.Caddy)
	return nil
}

func runConfigRemoveFlow(cfg *config) error {
	store, err := loadWebServerConfig()
	if err != nil {
		return err
	}

	profile, err := normalizeConfigProfileTarget(cfg.WebServer)
	if err != nil {
		return err
	}

	if profile == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("config remove requires provider nginx/caddy and <plan-name>")
		}
		value, err := promptConfigRemoveProfile(webServerNginx)
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
		profile = value
	}

	if profile == configProfileAll {
		return fmt.Errorf("config remove does not support provider all; use nginx or caddy")
	}

	if strings.TrimSpace(cfg.PlanName) == "" {
		if !shouldPrompt(cfg) {
			return fmt.Errorf("config remove requires <plan-name>")
		}
		defaultPlanName := ""
		if latestPath, err := readLatestPlanPointer(); err == nil {
			defaultPlanName = filepath.Base(filepath.Dir(latestPath))
		}
		planName, err := promptConfigPlanName(defaultPlanName)
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
		cfg.PlanName = strings.TrimSpace(planName)
		if cfg.PlanName == "" {
			return fmt.Errorf("config remove requires <plan-name>")
		}
	}

	emptyProfile := webServerProfileConfig{Sites: []webServerSiteSpec{}, InstallHostnames: []string{}}
	switch profile {
	case webServerNginx:
		store.Nginx = emptyProfile
	case webServerCaddy:
		store.Caddy = emptyProfile
	}

	if err := saveWebServerConfig(store); err != nil {
		return err
	}

	printSection("Config Removed")
	fmt.Fprintf(os.Stderr, "Removed profile: %s\n", strings.ToUpper(profile))
	fmt.Fprintf(os.Stderr, "Plan context: %s\n", cfg.PlanName)
	return nil
}

func printConfigProfileSummary(name string, profile webServerProfileConfig) {
	fmt.Fprintf(os.Stderr, "\n%s\n", webServerLabel(name))
	if len(profile.InstallHostnames) == 0 {
		fmt.Fprintln(os.Stderr, "  Targets: all hostnames")
	} else {
		fmt.Fprintf(os.Stderr, "  Targets: %s\n", strings.Join(profile.InstallHostnames, ", "))
	}
	fmt.Fprintf(os.Stderr, "  Sites: %d\n", len(profile.Sites))
	for _, site := range profile.Sites {
		httpsLabel := "http"
		if site.EnableHTTPS {
			httpsLabel = "https"
		}
		fmt.Fprintf(os.Stderr, "  - %s -> %s:%d (%s)\n", site.ServerName, site.UpstreamHost, site.UpstreamPort, httpsLabel)
	}
	if name == webServerNginx && strings.TrimSpace(profile.NginxCertbotEmail) != "" {
		fmt.Fprintf(os.Stderr, "  Certbot email: %s\n", profile.NginxCertbotEmail)
	}
}

func normalizeConfigProfileTarget(value string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	switch trimmed {
	case "", webServerNginx, webServerCaddy, configProfileAll:
		return trimmed, nil
	default:
		return "", fmt.Errorf("unknown config profile %q; expected nginx, caddy, or all", value)
	}
}

func runPlanFlow(cfg *config) error {
	cfg.SSHAuthMethod = sshAuthMethodKey
	cfg.SSHPassword = ""
	if shouldPrompt(cfg) {
		if err := collectInteractiveInputs(cfg); err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
	}

	if err := finalizePaths(cfg); err != nil {
		return err
	}

	if err := resolveConfigComponents(cfg); err != nil {
		return err
	}
	normalizeWebServerSelection(cfg)

	if err := validateExecutionConfig(cfg); err != nil {
		return err
	}

	state, err := prepareRuntime(cfg)
	if err != nil {
		return err
	}

	printConfigurationSummary(cfg)

	if shouldPromptApplyConfirmation(*cfg) {
		confirmed, err := promptApplyConfirmation()
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
		if !confirmed {
			fmt.Fprintln(os.Stderr, "civa apply was cancelled by the user before ansible-playbook started.")
			return nil
		}
	}

	return executeRuntime(cfg, state)
}

func runPlanReviewFlow(cfg *config) error {
	if err := validateExistingPlanCommandFlags(*cfg); err != nil {
		return err
	}

	planPath, err := resolvePlanInputFile(cfg)
	if err != nil {
		return err
	}

	content, err := os.ReadFile(planPath)
	if err != nil {
		return fmt.Errorf("failed to read plan file %s: %w", planPath, err)
	}

	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if header := planReviewHeader(planPath, isTTY); header != "" {
		fmt.Print(header)
	}

	rendered, err := renderPlanReviewMarkdown(planPath, content, isTTY)
	if err != nil {
		return err
	}
	fmt.Print(rendered)
	if rendered == "" || rendered[len(rendered)-1] != '\n' {
		fmt.Println()
	}

	return nil
}

func runPlanEditFlow(cfg *config) error {
	if err := validateExistingPlanCommandFlags(*cfg); err != nil {
		return err
	}

	planPath, err := resolvePlanInputFile(cfg)
	if err != nil {
		return err
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("plan edit requires an interactive terminal")
	}

	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}

	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("failed to resolve editor command")
	}

	cmd := exec.Command(parts[0], append(parts[1:], planPath)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor %q for plan %s: %w", editor, planPath, err)
	}

	fmt.Fprintf(os.Stderr, "Updated plan file: %s\n", planPath)
	return nil
}

func planReviewHeader(planPath string, isTTY bool) string {
	if !isTTY {
		return ""
	}

	return fmt.Sprintf("Plan file: %s\n\n", planPath)
}

func renderPlanReviewMarkdown(planPath string, content []byte, isTTY bool) (string, error) {
	style := styles.AutoStyle
	if !isTTY {
		style = styles.NoTTYStyle
	}

	width := 80
	if isTTY {
		if detectedWidth, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && detectedWidth > 0 {
			width = detectedWidth
			if width > 120 {
				width = 120
			}
		}
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithColorProfile(lipgloss.ColorProfile()),
		glowutils.GlamourStyle(style, false),
		glamour.WithWordWrap(width),
		glamour.WithBaseURL(""),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return "", fmt.Errorf("failed to initialize glow renderer for %s: %w", planPath, err)
	}

	rendered, err := renderer.Render(string(glowutils.RemoveFrontmatter(content)))
	if err != nil {
		return "", fmt.Errorf("failed to render plan review for %s: %w", planPath, err)
	}

	return rendered, nil
}

func runApplyFlow(cfg *config) error {
	if err := validateExistingPlanCommandFlags(*cfg); err != nil {
		return err
	}

	planPath, err := resolvePlanInputFile(cfg)
	if err != nil {
		return err
	}

	loadedCfg, state, err := loadPlannedRun(planPath)
	if err != nil {
		return err
	}

	if !cfg.AssumeYes {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("non-interactive apply requires --yes when executing an existing plan")
		}

		confirmed, err := promptApplyExistingPlanConfirmation(planPath)
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
		if !confirmed {
			fmt.Fprintln(os.Stderr, "🛑 civa apply was cancelled by the user before ansible-playbook started.")
			return nil
		}
	}

	loadedCfg.Command = commandApply
	loadedCfg.ApplyAction = applyActionExecute
	state.ProgressCurrent = 0
	state.ProgressTotal = 2
	state.CompletedPhases = nil

	printSection("🚀 Apply Existing Plan")
	for _, line := range applyArtifactLines(planPath, state) {
		fmt.Fprintln(os.Stderr, line)
	}

	state.progressStep("⚙️ Running ansible-playbook from existing plan")
	if err := runAnsible(loadedCfg, state); err != nil {
		_ = writeRollbackFailure(planPath, err)
		return err
	}
	state.appendCompletedPhase("✅ ansible-playbook execution")
	state.progressStep("🔧 Updating local SSH config from applied inventory")
	if err := syncSSHConfigAfterApply(loadedCfg, state); err != nil {
		_ = writeRollbackFailure(planPath, err)
		return err
	}
	state.appendCompletedPhase("✅ local SSH config synchronized")
	if err := writeRollbackSuccess(planPath); err != nil {
		return err
	}
	if snapshot, err := newDriftSnapshot(planPath, state); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ Unable to capture drift snapshot after apply: %v\n", err)
	} else if err := saveDriftSnapshot(planPath, snapshot); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️ Unable to persist drift snapshot after apply: %v\n", err)
	}
	showExecutionSummary(loadedCfg, state)
	return nil
}

func runApplyReviewFlow(cfg *config) error {
	if err := validateExistingPlanCommandFlags(*cfg); err != nil {
		return err
	}

	planPath, err := resolvePlanInputFile(cfg)
	if err != nil {
		return err
	}

	loadedCfg, state, err := loadPlannedRun(planPath)
	if err != nil {
		return err
	}

	loadedCfg.Command = commandApply
	loadedCfg.ApplyAction = applyActionReview
	state.ProgressCurrent = 0
	state.ProgressTotal = 4
	state.CompletedPhases = nil

	printSection("🔍 Apply Review")
	state.progressStep("📦 Loading planned review artifacts")
	for _, line := range applyArtifactLines(planPath, state) {
		fmt.Fprintln(os.Stderr, line)
	}
	state.appendCompletedPhase("✅ Loaded plan metadata and resolved inventory, vars, auth, and playbook artifacts")

	state.progressStep("🧭 Inspecting review scope from the generated plan")
	state.appendCompletedPhase(fmt.Sprintf("✅ Prepared review scope for %s", reviewScopeSummary(*loadedCfg)))

	state.progressStep("🧪 Running Ansible review in check mode (--check --diff)")
	if err := runAnsible(loadedCfg, state); err != nil {
		return err
	}
	state.appendCompletedPhase("✅ Completed ansible review run without applying changes to the server")

	state.progressStep("📝 Rendering detailed review summary")
	state.appendCompletedPhase("✅ Prepared detailed review summary for server state verification")
	showExecutionSummary(loadedCfg, state)
	return nil
}

func runApplyDriftFlow(cfg *config) error {
	if err := validateExistingPlanCommandFlags(*cfg); err != nil {
		return err
	}

	planPath, err := resolvePlanInputFile(cfg)
	if err != nil {
		return err
	}

	loadedCfg, state, err := loadPlannedRun(planPath)
	if err != nil {
		return err
	}

	loadedCfg.Command = commandApply
	loadedCfg.ApplyAction = applyActionDrift
	state.ProgressCurrent = 0
	state.ProgressTotal = 3
	state.CompletedPhases = nil

	printSection("🧭 Drift Detection")
	state.progressStep("📦 Loading planned artifacts for drift analysis")
	for _, line := range applyArtifactLines(planPath, state) {
		fmt.Fprintln(os.Stderr, line)
	}
	state.appendCompletedPhase("Loaded plan artifacts for drift detection")

	state.progressStep("🧪 Running ansible check mode to detect server drift")
	ansibleDrift, err := detectAnsibleDrift(loadedCfg, state)
	if err != nil {
		return err
	}
	if ansibleDrift {
		state.appendCompletedPhase("Detected drift from ansible check-mode output")
	} else {
		state.appendCompletedPhase("No drift detected from ansible check-mode output")
	}

	state.progressStep("🗃️ Comparing local plan artifacts against baseline snapshot")
	_, hasBaseline, err := loadDriftSnapshot(planPath)
	if err != nil {
		return err
	}
	snapshot, localDrift, err := compareDriftSnapshot(planPath, state)
	if err != nil {
		return err
	}
	if err := saveDriftSnapshot(planPath, snapshot); err != nil {
		return err
	}
	if !hasBaseline {
		state.appendCompletedPhase("Initialized local artifact drift baseline snapshot (first drift check)")
	} else if localDrift {
		state.appendCompletedPhase("Detected local artifact drift since previous drift snapshot")
	} else {
		state.appendCompletedPhase("No local artifact drift detected")
	}

	fmt.Fprintln(os.Stderr)
	if ansibleDrift || localDrift {
		fmt.Fprintln(os.Stderr, "⚠️ Drift detected. Review `civa apply review` output and reconcile before production apply.")
	} else if !hasBaseline {
		fmt.Fprintln(os.Stderr, "ℹ️ Drift baseline initialized. Re-run `civa apply drift` to compare local artifacts against this baseline.")
	} else {
		fmt.Fprintln(os.Stderr, "✅ No drift detected against current plan artifacts and server check-mode output.")
	}

	showExecutionSummary(loadedCfg, state)
	return nil
}

func runApplyRollbackFlow(cfg *config) error {
	if err := validateExistingPlanCommandFlags(*cfg); err != nil {
		return err
	}

	planPath, err := resolveRollbackPlanPath(cfg)
	if err != nil {
		return err
	}

	loadedCfg, state, err := loadPlannedRun(planPath)
	if err != nil {
		return err
	}

	if !cfg.AssumeYes {
		if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf("non-interactive apply rollback requires --yes")
		}

		confirmed, err := promptApplyExistingPlanConfirmation(planPath)
		if err != nil {
			if errors.Is(err, errUserCancelled) {
				return nil
			}
			return err
		}
		if !confirmed {
			fmt.Fprintln(os.Stderr, "🛑 civa apply rollback was cancelled by the user before ansible-playbook started.")
			return nil
		}
	}

	loadedCfg.Command = commandApply
	loadedCfg.ApplyAction = applyActionRollback
	state.ProgressCurrent = 0
	state.ProgressTotal = 4
	state.CompletedPhases = nil

	printSection("⏪ Apply Rollback")
	state.progressStep("📦 Loading rollback plan artifacts")
	for _, line := range applyArtifactLines(planPath, state) {
		fmt.Fprintln(os.Stderr, line)
	}
	state.appendCompletedPhase("Loaded rollback plan artifacts")

	state.progressStep("🧪 Running rollback preflight in check mode")
	if err := runRollbackPreflight(loadedCfg, state); err != nil {
		_ = writeRollbackFailure(planPath, err)
		return fmt.Errorf("rollback preflight failed: %w", err)
	}
	state.appendCompletedPhase("Rollback preflight completed in check mode")

	state.progressStep("⚙️ Executing rollback apply")
	if err := runAnsible(loadedCfg, state); err != nil {
		_ = writeRollbackFailure(planPath, err)
		return err
	}
	state.appendCompletedPhase("Rollback ansible apply completed")

	state.progressStep("🔧 Updating local SSH config from rollback inventory")
	if err := syncSSHConfigAfterApply(loadedCfg, state); err != nil {
		_ = writeRollbackFailure(planPath, err)
		return err
	}
	state.appendCompletedPhase("Local SSH config synchronized")

	if err := writeRollbackSuccess(planPath); err != nil {
		return err
	}

	showExecutionSummary(loadedCfg, state)
	return nil
}

func shouldPromptApplyConfirmation(cfg config) bool {
	action := cfg.ApplyAction
	if action == "" {
		action = applyActionExecute
	}
	return cfg.Command == commandApply && action == applyActionExecute && !cfg.NonInteractive
}

func applyArtifactLines(planPath string, state *runtimeState) []string {
	lines := []string{
		formatApplyArtifactLine("📄", "Plan file", planPath),
		formatApplyArtifactLine("🗂️", "Inventory", state.InventoryFile),
		formatApplyArtifactLine("🧩", "Vars", state.VarsFile),
	}
	if state.AuthFile != "" {
		lines = append(lines, formatApplyArtifactLine("🔐", "SSH auth file", state.AuthFile))
	}
	lines = append(lines, formatApplyArtifactLine("📜", "Playbook", state.PlaybookFile))
	return lines
}

func formatApplyArtifactLine(icon, label, value string) string {
	return fmt.Sprintf("%s  %-15s %s", icon, label+":", value)
}

func prepareRuntime(cfg *config) (*runtimeState, error) {
	runID, err := resolveGeneratedPlanName(cfg)
	if err != nil {
		return nil, err
	}
	generatedDir := filepath.Join(runRootDirectoryPath(), runID)
	inventoryFile := filepath.Join(generatedDir, "inventory.yml")
	varsFile := filepath.Join(generatedDir, "vars.yml")
	authFile := ""
	if cfg.SSHAuthMethod == sshAuthMethodPassword {
		authFile = filepath.Join(generatedDir, "auth.yml")
	}
	planFile := filepath.Join(generatedDir, "plan.md")
	metadataFile := planMetadataPath(planFile)
	playbookFile := filepath.Join(generatedDir, "ansible", "main.yml")

	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(planFile), 0o755); err != nil {
		return nil, err
	}
	if cfg.PlanFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.PlanFile), 0o755); err != nil {
			return nil, err
		}
	}
	if err := materializeAnsibleAssets(filepath.Join(generatedDir, "ansible")); err != nil {
		return nil, err
	}

	total := 3
	if cfg.Command != commandPlan {
		total = 4
	}
	if cfg.Command == commandPlan {
		total++
	}
	if cfg.Command == commandPlan && cfg.SSHAuthMethod == sshAuthMethodPassword {
		total++
	}

	return &runtimeState{
		RunID:           runID,
		GeneratedDir:    generatedDir,
		InventoryFile:   inventoryFile,
		VarsFile:        varsFile,
		AuthFile:        authFile,
		MetadataFile:    metadataFile,
		PlanFile:        planFile,
		PlaybookFile:    playbookFile,
		ProgressTotal:   total,
		CompletedPhases: []string{},
	}, nil
}

func resolveGeneratedPlanName(cfg *config) (string, error) {
	candidate := strings.TrimSpace(cfg.PlanName)
	if candidate == "" && len(cfg.Servers) > 0 {
		candidate = strings.TrimSpace(cfg.Servers[0].Hostname)
		if candidate == "" {
			candidate = strings.TrimSpace(cfg.Servers[0].Address)
		}
	}

	baseName := sanitizePlanName(candidate)
	if baseName == "" {
		planName := generateRunID(time.Now())
		cfg.PlanName = planName
		return planName, nil
	}

	planName, err := nextPlanVersionName(baseName)
	if err != nil {
		return "", err
	}

	cfg.PlanName = planName
	return planName, nil
}

func sanitizePlanName(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(raw))
	lastDash := false

	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || r == '.':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(builder.String(), "-._")
}

func generateRunID(now time.Time) string {
	return fmt.Sprintf("%s-%09d", now.Format("20060102-150405"), now.Nanosecond())
}

func executeRuntime(cfg *config, state *runtimeState) error {
	state.progressStep("Generating Ansible inventory")
	if err := writeInventory(cfg, state); err != nil {
		return err
	}
	state.appendCompletedPhase("Inventory generated")

	state.progressStep("Generating Ansible vars file")
	if err := writeVarsFile(cfg, state); err != nil {
		return err
	}
	state.appendCompletedPhase("Vars file generated")

	if cfg.SSHAuthMethod == sshAuthMethodPassword {
		state.progressStep("Generating SSH auth file")
		if err := writeAuthFile(cfg, state); err != nil {
			return err
		}
		state.appendCompletedPhase("SSH auth file generated")
	}

	state.progressStep("Writing Markdown execution plan")
	if err := writePlanFile(cfg, state); err != nil {
		return err
	}
	state.appendCompletedPhase("Markdown plan generated")

	if cfg.Command == commandPlan {
		state.progressStep("Writing structured plan metadata")
		if err := writePlanMetadata(cfg, state); err != nil {
			return err
		}
		state.appendCompletedPhase("Structured plan metadata generated")
	}

	if cfg.Command != commandPlan {
		state.progressStep("Running ansible-playbook")
		state.appendCompletedPhase("ansible-playbook execution")
		if err := runAnsible(cfg, state); err != nil {
			return err
		}
		if cfg.Command == commandApply {
			if err := syncSSHConfigAfterApply(cfg, state); err != nil {
				return err
			}
		}
	}

	showExecutionSummary(cfg, state)
	fmt.Printf("Generated plan name: %s\n", state.RunID)
	fmt.Printf("Generated plan: %s\n", state.PlanFile)
	if cfg.PlanFile != "" {
		fmt.Printf("Exported plan copy: %s\n", cfg.PlanFile)
	}
	fmt.Printf("Inventory: %s\n", state.InventoryFile)
	fmt.Printf("Vars: %s\n", state.VarsFile)
	return nil
}

func shouldPrompt(cfg *config) bool {
	if cfg.NonInteractive {
		return false
	}

	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func finalizePaths(cfg *config) error {
	var err error
	cfg.SSHPrivateKey, err = expandHomePath(cfg.SSHPrivateKey)
	if err != nil {
		return err
	}
	if !cfg.Provided.SSHPublicKey && cfg.SSHPrivateKey != "" {
		cfg.SSHPublicKey = cfg.SSHPrivateKey + ".pub"
	}
	cfg.SSHPublicKey, err = expandHomePath(cfg.SSHPublicKey)
	if err != nil {
		return err
	}
	if cfg.PlanFile != "" {
		cfg.PlanFile, err = expandHomePath(cfg.PlanFile)
		if err != nil {
			return err
		}
	}
	if cfg.PlanInputFile != "" {
		cfg.PlanInputFile, err = expandHomePath(cfg.PlanInputFile)
		if err != nil {
			return err
		}
	}
	return nil
}

func resolveConfigComponents(cfg *config) error {
	if len(cfg.Components) > 0 {
		return nil
	}

	components, err := resolveComponents(cfg.ComponentsInput)
	if err != nil {
		return err
	}
	cfg.Components = components
	return nil
}

func normalizeWebServerSelection(cfg *config) {
	inferredWebServer := inferWebServerFromComponentsInput(cfg.ComponentsInput)
	hasWebServer := selectedComponentsInclude(cfg.Components, "web_server")

	if !cfg.Provided.WebServer {
		switch {
		case inferredWebServer != "":
			cfg.WebServer = inferredWebServer
		case hasWebServer:
			cfg.WebServer = webServerTraefik
		default:
			cfg.WebServer = webServerNone
		}
	}

	if cfg.WebServer != webServerNone && !hasWebServer {
		cfg.Components = append(cfg.Components, "web_server")
		hasWebServer = true
	}

	if cfg.WebServer == webServerNone && hasWebServer {
		filtered := cfg.Components[:0]
		for _, component := range cfg.Components {
			if component != "web_server" {
				filtered = append(filtered, component)
			}
		}
		cfg.Components = filtered
	}
}

func applyPersistedWebServerConfig(cfg *config) error {
	cfg.WebServerSites = nil
	cfg.WebServerTargetHosts = nil
	cfg.NginxCertbotEmail = ""

	if !supportsCustomWebServerSites(cfg.WebServer) {
		return nil
	}

	stored, err := loadWebServerConfig()
	if err != nil {
		return err
	}

	profile := stored.Caddy
	if cfg.WebServer == webServerNginx {
		profile = stored.Nginx
	}

	cfg.WebServerSites = append([]webServerSiteSpec(nil), profile.Sites...)
	cfg.WebServerTargetHosts = append([]string(nil), normalizeHostnameList(profile.InstallHostnames)...)
	if cfg.WebServer == webServerNginx {
		cfg.NginxCertbotEmail = strings.TrimSpace(profile.NginxCertbotEmail)
	}

	return nil
}

func inferWebServerFromComponentsInput(raw string) string {
	tokens := strings.Split(strings.NewReplacer(";", ",", " ", ",").Replace(strings.TrimSpace(raw)), ",")
	for _, token := range tokens {
		switch normalizeComponentToken(token) {
		case webServerTraefik:
			return webServerTraefik
		case webServerNginx:
			return webServerNginx
		case webServerCaddy:
			return webServerCaddy
		}
	}
	return ""
}

func validateExecutionConfig(cfg *config) error {
	if len(cfg.Servers) == 0 {
		return fmt.Errorf("at least one --server entry is required")
	}
	cfg.SSHAuthMethod = sshAuthMethodKey
	if !isValidWebServer(cfg.WebServer) {
		return fmt.Errorf("--web-server must be none, traefik, nginx, or caddy")
	}
	if cfg.SSHPort < 1 || cfg.SSHPort > 65535 {
		return fmt.Errorf("--ssh-port must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.SSHUser) == "" {
		return fmt.Errorf("--ssh-user must not be empty")
	}
	if strings.TrimSpace(cfg.DeployUser) == "" {
		return fmt.Errorf("--deployer-user must not be empty")
	}
	if strings.TrimSpace(cfg.Timezone) == "" {
		return fmt.Errorf("--timezone must not be empty")
	}
	if len(cfg.Components) == 0 {
		return fmt.Errorf("at least one component must be selected")
	}
	if cfg.Provided.SSHAuthMethod || cfg.Provided.SSHPassword {
		return fmt.Errorf("civa plan init only supports SSH key auth; use civa setup to install the public key first")
	}
	if _, err := os.Stat(cfg.SSHPrivateKey); err != nil {
		return fmt.Errorf("SSH private key not found: %s", cfg.SSHPrivateKey)
	}
	if _, err := os.Stat(cfg.SSHPublicKey); err != nil {
		return fmt.Errorf("SSH public key not found: %s", cfg.SSHPublicKey)
	}
	if selectedComponentsInclude(cfg.Components, "web_server") && cfg.WebServer == webServerNone {
		return fmt.Errorf("web server component requires --web-server to be set")
	}
	if cfg.WebServer == webServerTraefik {
		if strings.TrimSpace(cfg.TraefikEmail) == "" {
			return fmt.Errorf("Traefik requires a non-empty ACME email")
		}
		if cfg.TraefikChallenge != "http" && cfg.TraefikChallenge != "dns" {
			return fmt.Errorf("Traefik challenge must be http or dns")
		}
		if cfg.TraefikChallenge == "dns" && strings.TrimSpace(cfg.TraefikDNSProvider) == "" {
			return fmt.Errorf("Traefik DNS challenge requires --traefik-dns-provider")
		}
	}
	if len(cfg.WebServerSites) > 0 {
		if !supportsCustomWebServerSites(cfg.WebServer) {
			return fmt.Errorf("custom web server sites are only supported for nginx or caddy")
		}
		for idx, site := range cfg.WebServerSites {
			if strings.TrimSpace(site.ServerName) == "" {
				return fmt.Errorf("web server site %d requires a server name", idx+1)
			}
			if strings.TrimSpace(site.UpstreamHost) == "" {
				return fmt.Errorf("web server site %d requires an upstream host", idx+1)
			}
			if site.UpstreamPort < 1 || site.UpstreamPort > 65535 {
				return fmt.Errorf("web server site %d upstream port must be between 1 and 65535", idx+1)
			}
			if site.EnableHTTPS && cfg.WebServer != webServerNginx {
				return fmt.Errorf("web server site %d enables HTTPS, but only nginx HTTPS via certbot is supported", idx+1)
			}
		}
	}
	if cfg.WebServer == webServerNginx && hasHTTPSWebServerSites(cfg.WebServerSites) && strings.TrimSpace(cfg.NginxCertbotEmail) == "" {
		return fmt.Errorf("nginx HTTPS configuration requires a certbot email")
	}
	for idx, host := range cfg.WebServerTargetHosts {
		if strings.TrimSpace(host) == "" {
			return fmt.Errorf("web server install target host %d must not be empty", idx+1)
		}
	}
	return nil
}

func validateWebServerRuntimeConfig(cfg *config) error {
	if !supportsCustomWebServerSites(cfg.WebServer) {
		return fmt.Errorf("config playbook only supports nginx or caddy web server profiles")
	}
	for idx, site := range cfg.WebServerSites {
		if strings.TrimSpace(site.ServerName) == "" {
			return fmt.Errorf("web server site %d requires a server name", idx+1)
		}
		if strings.TrimSpace(site.UpstreamHost) == "" {
			return fmt.Errorf("web server site %d requires an upstream host", idx+1)
		}
		if site.UpstreamPort < 1 || site.UpstreamPort > 65535 {
			return fmt.Errorf("web server site %d upstream port must be between 1 and 65535", idx+1)
		}
		if site.EnableHTTPS && cfg.WebServer != webServerNginx {
			return fmt.Errorf("web server site %d enables HTTPS, but only nginx HTTPS via certbot is supported", idx+1)
		}
	}
	if cfg.WebServer == webServerNginx && hasHTTPSWebServerSites(cfg.WebServerSites) && strings.TrimSpace(cfg.NginxCertbotEmail) == "" {
		return fmt.Errorf("nginx HTTPS configuration requires a certbot email")
	}
	for idx, host := range cfg.WebServerTargetHosts {
		if strings.TrimSpace(host) == "" {
			return fmt.Errorf("web server install target host %d must not be empty", idx+1)
		}
	}
	return nil
}

func validateSetupConfig(cfg *config) error {
	if len(cfg.Servers) != 1 {
		return fmt.Errorf("civa setup requires exactly one --server target")
	}
	if strings.TrimSpace(cfg.SSHUser) == "" {
		return fmt.Errorf("--ssh-user must not be empty")
	}
	if cfg.SSHPort < 1 || cfg.SSHPort > 65535 {
		return fmt.Errorf("--ssh-port must be between 1 and 65535")
	}
	if _, err := os.Stat(cfg.SSHPublicKey); err != nil {
		return fmt.Errorf("SSH public key not found: %s", cfg.SSHPublicKey)
	}
	if _, err := exec.LookPath("ssh-copy-id"); err != nil {
		return fmt.Errorf("ssh-copy-id is required for civa setup")
	}
	if strings.TrimSpace(cfg.SSHPassword) != "" {
		if _, err := exec.LookPath("sshpass"); err != nil {
			return fmt.Errorf("sshpass is required when --ssh-password is provided to civa setup")
		}
	}
	return nil
}

func parseServerSpec(raw string) (serverSpec, error) {
	parts := strings.Split(raw, ",")
	address := strings.TrimSpace(parts[0])
	if address == "" {
		return serverSpec{}, fmt.Errorf("--server requires an address or IP")
	}
	server := serverSpec{Address: address}

	if len(parts) >= 2 {
		second := strings.TrimSpace(parts[1])
		if second != "" {
			if port, err := strconv.Atoi(second); err == nil {
				if port < 1 || port > 65535 {
					return serverSpec{}, fmt.Errorf("--server ssh port must be between 1 and 65535")
				}
				server.SSHPort = port
			} else {
				server.Hostname = second
			}
		}
	}

	if len(parts) >= 3 {
		portToken := strings.TrimSpace(parts[2])
		if portToken != "" {
			port, err := strconv.Atoi(portToken)
			if err != nil {
				return serverSpec{}, fmt.Errorf("--server ssh port must be an integer")
			}
			if port < 1 || port > 65535 {
				return serverSpec{}, fmt.Errorf("--server ssh port must be between 1 and 65535")
			}
			server.SSHPort = port
		}
	}

	return server, nil
}

func resolveComponents(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" || strings.EqualFold(strings.TrimSpace(raw), "all") {
		return defaultComponentValues(), nil
	}

	normalized := strings.NewReplacer(";", ",", " ", ",").Replace(raw)
	tokens := strings.Split(normalized, ",")
	resolved := make([]string, 0, len(componentOptions))
	seen := map[string]struct{}{}

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		component, err := componentTokenToValue(token)
		if err != nil {
			return nil, err
		}
		if component == "all" {
			return defaultComponentValues(), nil
		}
		if _, exists := seen[component]; exists {
			continue
		}
		seen[component] = struct{}{}
		resolved = append(resolved, component)
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("at least one component must be selected")
	}

	return resolved, nil
}

func componentTokenToValue(token string) (string, error) {
	normalized := normalizeComponentToken(token)
	switch normalized {
	case "all":
		return "all", nil
	case "1", "system-update", "system-update-upgrade", "system-update---upgrade", "system-update-upgrade:":
		return "system_update", nil
	case "2", "user-management":
		return "user_management", nil
	case "3", "ssh-hardening":
		return "ssh_hardening", nil
	case "4", "security", "security-firewall", "firewall", "fail2ban":
		return "security_firewall", nil
	case "5", "system-config", "system-configuration", "swap", "timezone":
		return "system_config", nil
	case "6", "dependencies", "packages":
		return "dependencies", nil
	case "7", "containerization", "docker":
		return "containerization", nil
	case "8", "web-server", "webserver", "reverse-proxy", "reverse-proxy-traefik", "traefik", "nginx", "caddy":
		return "web_server", nil
	default:
		return "", fmt.Errorf("unknown component selection: %s", token)
	}
}

func normalizeComponentToken(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	token = strings.ReplaceAll(token, "_", "-")
	token = strings.ReplaceAll(token, " ", "-")
	return token
}

func defaultComponentValues() []string {
	values := make([]string, 0, len(componentOptions))
	for _, option := range componentOptions {
		values = append(values, option.Value)
	}
	return values
}

func selectedComponentsInclude(components []string, needle string) bool {
	return slices.Contains(components, needle)
}

func selectedAnsibleTags(cfg config) []string {
	tags := make([]string, 0, len(cfg.Components)+1)
	for _, component := range cfg.Components {
		if component == "web_server" {
			continue
		}
		tags = append(tags, normalizedComponentTag(component))
	}
	if webServerTag := normalizedWebServerTag(cfg.WebServer); webServerTag != "" {
		tags = append(tags, webServerTag)
	}
	return tags
}

func normalizedComponentTag(component string) string {
	switch component {
	case "dependencies":
		return "system_dependencies"
	case "containerization":
		return "system_containerization"
	default:
		return component
	}
}

func normalizedWebServerTag(server string) string {
	switch server {
	case webServerTraefik:
		return "web_server_traefik"
	case webServerNginx:
		return "web_server_nginx"
	case webServerCaddy:
		return "web_server_caddy"
	default:
		return ""
	}
}

func expandHomePath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func planMetadataPath(planPath string) string {
	ext := filepath.Ext(planPath)
	if ext == "" {
		return planPath + ".json"
	}

	return strings.TrimSuffix(planPath, ext) + ".json"
}

func printUsage() {
	styled := canStyleStdout()
	blocks := []outputBlock{
		{Title: "Usage", Lines: []string{"civa <command> [options]"}},
		{Title: "Commands", Lines: []string{
			"tools                     Run interactive external provider tools",
			"tools cloudflare zones    List Cloudflare zones",
			"config <provider> init     Initialize or update persisted config profile (provider: nginx or caddy)",
			"config <provider> list     List persisted config profile (provider: nginx, caddy, or all)",
			"config <provider> remove   Remove persisted config profile (provider: nginx or caddy, requires <plan-name>)",
			"secret list                List stored encrypted secret names",
			"secret set <name>          Store or update an encrypted secret",
			"secret remove <name>       Remove an encrypted secret",
			"apply <plan-name>          Execute an existing generated plan",
			"apply review <plan-name>   Verify an applied plan with ansible check mode",
			"apply drift <plan-name>    Detect server and artifact drift from an existing plan",
			"apply rollback [plan-name] Roll back to last successful plan or a specific plan",
			"plan init                  Generate inventory, vars, and the execution plan only",
			"plan review <plan-name>    Render an existing generated plan",
			"plan edit <plan-name>      Edit an existing generated plan in your editor",
			"plan list                  List generated plans",
			"plan remove <plan-name>    Remove a generated plan and its artifacts",
			"setup                      Install a public SSH key on a server with ssh-copy-id",
			"completion <shell>         Print shell completion for bash, zsh, or fish",
			"doctor [fix]               Check or install local dependencies for civa",
			"uninstall                  Remove the currently installed civa binary",
			"version                    Show the civa version",
			"help                       Show this help message",
		}},
		{Title: "Options", Lines: []string{
			"--non-interactive          Disable prompts and rely on provided flags",
			"--yes, -y                  Skip confirmation prompts for destructive commands",
			"--ssh-user <name>          SSH user used to connect to every target server",
			"--ssh-port <port>          SSH port used to connect to every target server",
			"--ssh-password <value>     SSH password used by civa setup",
			"--ssh-password-secret <name> Secret name used by civa setup for SSH password",
			"--web-server <name>        Web server to prepare: traefik, nginx, caddy, or none",
			"--ssh-private-key <path>   Local private key path used by Ansible for SSH",
			"--ssh-public-key <path>    Local public key path that will be installed for the deploy user",
			"--deployer-user <name>     User created and configured on the target servers",
			"--timezone <tz>            Timezone applied to the target servers",
			"--components <list>        Components to run: all or a comma list such as 1,2,4 or docker,traefik",
			"--plan-file <path>         Existing plan file override used by plan review/edit or apply",
			"--server <addr[,hostname][,port]> Add a target server; hostname and SSH port are optional",
			"--traefik-email <email>    Email used by Let's Encrypt ACME",
			"--traefik-challenge <type> Traefik challenge type: http or dns",
			"--traefik-dns-provider <id> DNS provider name used when challenge type is dns",
			"--output <path>            Extra exported Markdown copy for plan init",
			"--value-file <path>        Path to secret value file for `civa secret set`",
			"--token <value>            Cloudflare API token used by `civa tools cloudflare`",
			"--help                     Show this help message",
		}},
		{Title: "Examples", Lines: []string{
			"civa tools",
			"civa tools cloudflare zones",
			"civa tools cloudflare zones --token $CLOUDFLARE_API_TOKEN",
			"civa config nginx init web-01-v2",
			"civa config nginx list",
			"civa config all list",
			"civa config caddy remove web-01-v2",
			"civa plan init --non-interactive --server 203.0.113.10,web-01,2201 --server 203.0.113.11,api-01,2202 --components 1,2,3,4",
			"civa plan list",
			"civa plan review web-01",
			"civa plan edit web-01",
			"civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub",
			"civa setup --server 203.0.113.10 --ssh-password-secret vps-root-password --ssh-public-key ~/.ssh/id_ed25519.pub",
			"civa secret set vps-root-password --value-file ~/.secrets/vps-root-password.txt",
			"civa secret list",
			"civa doctor",
			"civa doctor fix",
			"civa completion bash",
			"civa apply web-01 --yes",
			"civa apply review web-01",
			"civa apply drift web-01",
			"civa apply rollback --yes",
			"civa plan remove web-01 --yes",
			"civa uninstall --yes",
		}},
	}

	fmt.Println(renderSectionTitle("civa", styled))
	fmt.Println(renderOutputBlocks(blocks, styled))
}

func printCommandUsage(command string) {
	styled := canStyleStdout()
	switch command {
	case commandTools:
		fmt.Println(renderSectionTitle("civa tools", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa tools", "civa tools cloudflare", "civa tools cloudflare zones [--token <value>]"}},
			{Title: "Providers", Lines: []string{"cloudflare                Cloudflare DNS/account helper utilities"}},
			{Title: "Examples", Lines: []string{"civa tools", "civa tools cloudflare", "civa tools cloudflare zones", "civa tools cloudflare zones --token $CLOUDFLARE_API_TOKEN"}},
		}, styled))
	case commandToolsCloudflare:
		fmt.Println(renderSectionTitle("civa tools cloudflare", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa tools cloudflare", "civa tools cloudflare zones [--token <value>]"}},
			{Title: "Actions", Lines: []string{"zones                     List zones available to the API token"}},
			{Title: "Token Resolution", Lines: []string{"1) --token flag", "2) CLOUDFLARE_API_TOKEN env", "3) interactive hidden prompt (TTY only)"}},
			{Title: "Examples", Lines: []string{"civa tools cloudflare", "civa tools cloudflare zones", "civa tools cloudflare zones --token $CLOUDFLARE_API_TOKEN"}},
		}, styled))
	case commandConfig:
		fmt.Println(renderSectionTitle("civa config", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa config <nginx|caddy> init [plan-name]", "civa config <nginx|caddy|all> list", "civa config <nginx|caddy> remove <plan-name>"}},
			{Title: "What it configures", Lines: []string{"Persisted web server profile for nginx/caddy", "Nginx HTTPS mode via certbot", "Apply configured profile using inventory from existing generated plan"}},
			{Title: "Examples", Lines: []string{"civa config nginx init web-01-v2", "civa config nginx list", "civa config all list", "civa config caddy remove web-01-v2"}},
		}, styled))
	case commandConfigNginxHelp:
		fmt.Println(renderSectionTitle("civa config nginx", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa config nginx init [plan-name]", "civa config nginx list", "civa config nginx remove <plan-name>"}},
			{Title: "Examples", Lines: []string{"civa config nginx init web-01-v2", "civa config nginx list", "civa config nginx remove web-01-v2"}},
		}, styled))
	case commandConfigCaddyHelp:
		fmt.Println(renderSectionTitle("civa config caddy", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa config caddy init [plan-name]", "civa config caddy list", "civa config caddy remove <plan-name>"}},
			{Title: "Examples", Lines: []string{"civa config caddy init web-01-v2", "civa config caddy list", "civa config caddy remove web-01-v2"}},
		}, styled))
	case commandConfigAllHelp:
		fmt.Println(renderSectionTitle("civa config all", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa config all list"}},
			{Title: "Examples", Lines: []string{"civa config all list"}},
		}, styled))
	case commandPlan:
		fmt.Println(renderSectionTitle("civa plan", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa plan init [options]", "civa plan review <plan-name>", "civa plan edit <plan-name>", "civa plan list [plan-name]", "civa plan remove <plan-name> [--yes]", "civa plan review --plan-file <path>", "civa plan edit --plan-file <path>"}},
			{Title: "Subcommands", Lines: []string{"init                         Generate a new versioned plan under ~/.civa/runs/", "review <plan-name>           Render an existing Markdown plan", "edit <plan-name>             Edit an existing Markdown plan in your editor", "list [plan-name]             List all plans or versions for one plan name", "remove <plan-name>           Remove a generated plan and its artifacts"}},
			{Title: "Examples", Lines: []string{"civa plan init --non-interactive --server 203.0.113.10,web-01,2201 --components all", "civa plan review web-01", "civa plan edit web-01", "civa plan list", "civa plan list web-01", "civa plan web-01 list", "civa plan remove web-01-v2 --yes"}},
		}, styled))
	case commandApply:
		fmt.Println(renderSectionTitle("civa apply", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa apply <plan-name> [--yes]", "civa apply review <plan-name>", "civa apply drift <plan-name>", "civa apply rollback [plan-name] [--yes]", "civa apply --plan-file <path> [--yes]", "civa apply review --plan-file <path>", "civa apply drift --plan-file <path>", "civa apply rollback --plan-file <path> [--yes]"}},
			{Title: "Examples", Lines: []string{"civa apply web-01 --yes", "civa apply review web-01", "civa apply drift web-01", "civa apply rollback --yes", "civa apply --plan-file ~/.civa/runs/web-01/plan.md --yes"}},
		}, styled))
	case commandSecret:
		fmt.Println(renderSectionTitle("civa secret", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa secret list", "civa secret set <name> --value-file <path>", "civa secret set <name> --value <secret>", "civa secret set <name>  # prompts for hidden input", "civa secret remove <name>"}},
			{Title: "Examples", Lines: []string{"civa secret set vps-root-password --value-file ~/.secrets/vps-root-password.txt", "civa secret set vps-root-password", "civa secret list", "civa secret remove vps-root-password"}},
		}, styled))
	case commandCompletion:
		fmt.Println(renderSectionTitle("civa completion", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa completion <shell>"}},
			{Title: "Supported shells", Lines: []string{"bash", "zsh", "fish"}},
			{Title: "Examples", Lines: []string{"civa completion bash", "civa completion zsh", "civa completion fish"}},
		}, styled))
	case commandDoctor:
		fmt.Println(renderSectionTitle("civa doctor", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa doctor", "civa doctor fix"}},
			{Title: "Subcommands", Lines: []string{"fix                          Install or update required local dependencies"}},
			{Title: "Required minimum versions", Lines: []string{"go >= 1.26", "ansible-playbook >= 2.20", "python3 (or python) >= 3.10"}},
			{Title: "Examples", Lines: []string{"civa doctor", "civa doctor fix"}},
		}, styled))
	case commandSetup:
		fmt.Println(renderSectionTitle("civa setup", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa setup [options]"}},
			{Title: "Required options", Lines: []string{"--server <addr>", "--ssh-user <name>", "--ssh-public-key <path>"}},
			{Title: "Optional", Lines: []string{"--ssh-password <value>", "--ssh-password-secret <name>"}},
			{Title: "Examples", Lines: []string{"civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub", "civa setup --server 203.0.113.10 --ssh-user root --ssh-password-secret vps-root-password --ssh-public-key ~/.ssh/id_ed25519.pub", "civa setup --server 203.0.113.10 --ssh-user root --ssh-public-key ~/.ssh/id_ed25519.pub", "civa setup --server 203.0.113.10 --ssh-user ubuntu --ssh-port 2222 --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub"}},
		}, styled))
	default:
		printUsage()
	}
}

func componentLabel(value string) string {
	for _, option := range componentOptions {
		if option.Value == value {
			return option.Label
		}
	}
	return value
}

const (
	sshAuthMethodKey      = "key"
	sshAuthMethodPassword = "password"
)

func isValidSSHAuthMethod(value string) bool {
	return value == sshAuthMethodKey || value == sshAuthMethodPassword
}

const (
	webServerNone    = "none"
	webServerTraefik = "traefik"
	webServerNginx   = "nginx"
	webServerCaddy   = "caddy"
)

func isValidWebServer(value string) bool {
	return value == webServerNone || value == webServerTraefik || value == webServerNginx || value == webServerCaddy
}

func webServerLabel(value string) string {
	switch value {
	case webServerTraefik:
		return "Traefik"
	case webServerNginx:
		return "Nginx"
	case webServerCaddy:
		return "Caddy"
	default:
		return "None"
	}
}

func supportsCustomWebServerSites(value string) bool {
	return value == webServerNginx || value == webServerCaddy
}

func hasHTTPSWebServerSites(sites []webServerSiteSpec) bool {
	for _, site := range sites {
		if site.EnableHTTPS {
			return true
		}
	}
	return false
}

func validateExistingPlanCommandFlags(cfg config) error {
	if cfg.Provided.SSHUser || cfg.Provided.SSHPort || cfg.Provided.SSHAuthMethod || cfg.Provided.SSHPassword || cfg.Provided.SSHPasswordSecret || cfg.Provided.WebServer || cfg.Provided.SSHPrivateKey || cfg.Provided.SSHPublicKey || cfg.Provided.DeployUser || cfg.Provided.Timezone || cfg.Provided.Components || cfg.Provided.PlanFile || cfg.Provided.TraefikEmail || cfg.Provided.TraefikChallenge || cfg.Provided.TraefikDNSProvider || cfg.Provided.Servers || cfg.Provided.SecretValue || cfg.Provided.SecretValueFile {
		return fmt.Errorf("plan review/edit and apply only accept --plan-file, --yes, --non-interactive, and --help")
	}
	return nil
}

func sshAuthMethodLabel(value string) string {
	if value == sshAuthMethodPassword {
		return "Password"
	}

	return "SSH Key"
}
