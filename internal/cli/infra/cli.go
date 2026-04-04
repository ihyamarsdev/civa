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
	commandPreview          = "preview"
	commandCompletion       = "completion"
	commandCompleteInternal = "__complete"
	commandDoctor           = "doctor"
	commandSetup            = "setup"
	commandConfig           = "config"
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
	planActionStart           = "start"
	planActionList            = "list"
	planActionRemove          = "remove"
	applyActionExecute        = "execute"
	applyActionReview         = "review"
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
}

type config struct {
	Command              string
	PlanAction           string
	ApplyAction          string
	DoctorAction         string
	PlanName             string
	AssumeYes            bool
	NonInteractive       bool
	SSHUser              string
	SSHPort              int
	SSHAuthMethod        string
	SSHPassword          string
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
		PlanAction:         planActionStart,
		ApplyAction:        applyActionExecute,
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

func runConfigFlow(cfg *config) error {
	if !shouldPrompt(cfg) {
		return fmt.Errorf("civa config currently requires an interactive terminal")
	}

	store, err := loadWebServerConfig()
	if err != nil {
		return err
	}

	printSection("civa Config")
	logLine("Configure persisted web server settings. Plan start will only install web server components.")

	targetWebServer, err := promptConfigWebServerTarget(webServerNginx)
	if err != nil {
		if errors.Is(err, errUserCancelled) {
			return nil
		}
		return err
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

	return nil
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
	if err := applyPersistedWebServerConfig(cfg); err != nil {
		return err
	}

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

func runPreviewFlow(cfg *config) error {
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
	if header := previewHeader(planPath, isTTY); header != "" {
		fmt.Print(header)
	}

	rendered, err := renderPreviewMarkdown(planPath, content, isTTY)
	if err != nil {
		return err
	}
	fmt.Print(rendered)
	if rendered == "" || rendered[len(rendered)-1] != '\n' {
		fmt.Println()
	}

	return nil
}

func previewHeader(planPath string, isTTY bool) string {
	if !isTTY {
		return ""
	}

	return fmt.Sprintf("Plan file: %s\n\n", planPath)
}

func renderPreviewMarkdown(planPath string, content []byte, isTTY bool) (string, error) {
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
		return "", fmt.Errorf("failed to render preview for %s: %w", planPath, err)
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
		return err
	}
	state.appendCompletedPhase("✅ ansible-playbook execution")
	state.progressStep("🔧 Updating local SSH config from applied inventory")
	if err := syncSSHConfigAfterApply(loadedCfg, state); err != nil {
		return err
	}
	state.appendCompletedPhase("✅ local SSH config synchronized")
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
		if cfg.Command == commandPreview {
			state.appendCompletedPhase("ansible-playbook check run")
		} else {
			state.appendCompletedPhase("ansible-playbook execution")
		}
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
		return fmt.Errorf("civa plan start only supports SSH key auth; use civa setup to install the public key first")
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
			"config                     Configure persistent civa settings (interactive)",
			"apply <plan-name>          Execute an existing generated plan",
			"apply review <plan-name>   Verify an applied plan with ansible check mode",
			"plan start                 Generate inventory, vars, and the execution plan only",
			"plan list                  List generated plans",
			"plan remove <plan-name>    Remove a generated plan and its artifacts",
			"preview <plan-name>        Show an existing generated plan",
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
			"--web-server <name>        Web server to prepare: traefik, nginx, caddy, or none",
			"--ssh-private-key <path>   Local private key path used by Ansible for SSH",
			"--ssh-public-key <path>    Local public key path that will be installed for the deploy user",
			"--deployer-user <name>     User created and configured on the target servers",
			"--timezone <tz>            Timezone applied to the target servers",
			"--components <list>        Components to run: all or a comma list such as 1,2,4 or docker,traefik",
			"--plan-file <path>         Existing plan file override used by preview or apply",
			"--server <addr[,hostname][,port]> Add a target server; hostname and SSH port are optional",
			"--traefik-email <email>    Email used by Let's Encrypt ACME",
			"--traefik-challenge <type> Traefik challenge type: http or dns",
			"--traefik-dns-provider <id> DNS provider name used when challenge type is dns",
			"--output <path>            Extra exported Markdown copy for plan start",
			"--help                     Show this help message",
		}},
		{Title: "Examples", Lines: []string{
			"civa config",
			"civa plan start --non-interactive --server 203.0.113.10,web-01,2201 --server 203.0.113.11,api-01,2202 --components 1,2,3,4",
			"civa plan list",
			"civa preview web-01",
			"civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub",
			"civa doctor",
			"civa doctor fix",
			"civa completion bash",
			"civa apply web-01 --yes",
			"civa apply review web-01",
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
	case commandConfig:
		fmt.Println(renderSectionTitle("civa config", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa config"}},
			{Title: "What it configures", Lines: []string{"Persisted web server profile for nginx/caddy", "Nginx HTTPS mode via certbot"}},
			{Title: "Examples", Lines: []string{"civa config"}},
		}, styled))
	case commandPlan:
		fmt.Println(renderSectionTitle("civa plan", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa plan start [options]", "civa plan list [plan-name]", "civa plan remove <plan-name> [--yes]"}},
			{Title: "Subcommands", Lines: []string{"start                        Generate a new versioned plan under ~/.civa/runs/", "list [plan-name]             List all plans or versions for one plan name", "remove <plan-name>           Remove a generated plan and its artifacts"}},
			{Title: "Examples", Lines: []string{"civa plan start --non-interactive --server 203.0.113.10,web-01,2201 --components all", "civa plan list", "civa plan list web-01", "civa plan web-01 list", "civa plan remove web-01-v2 --yes"}},
		}, styled))
	case commandPreview:
		fmt.Println(renderSectionTitle("civa preview", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa preview <plan-name>", "civa preview --plan-file <path>"}},
			{Title: "Examples", Lines: []string{"civa preview web-01", "civa preview --plan-file ~/.civa/runs/web-01/plan.md"}},
		}, styled))
	case commandApply:
		fmt.Println(renderSectionTitle("civa apply", styled))
		fmt.Println(renderOutputBlocks([]outputBlock{
			{Title: "Usage", Lines: []string{"civa apply <plan-name> [--yes]", "civa apply review <plan-name>", "civa apply --plan-file <path> [--yes]", "civa apply review --plan-file <path>"}},
			{Title: "Examples", Lines: []string{"civa apply web-01 --yes", "civa apply review web-01", "civa apply --plan-file ~/.civa/runs/web-01/plan.md --yes"}},
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
			{Title: "Optional", Lines: []string{"--ssh-password <value>"}},
			{Title: "Examples", Lines: []string{"civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub", "civa setup --server 203.0.113.10 --ssh-user root --ssh-public-key ~/.ssh/id_ed25519.pub", "civa setup --server 203.0.113.10 --ssh-user ubuntu --ssh-port 2222 --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub"}},
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
	if cfg.Provided.SSHUser || cfg.Provided.SSHPort || cfg.Provided.SSHAuthMethod || cfg.Provided.SSHPassword || cfg.Provided.WebServer || cfg.Provided.SSHPrivateKey || cfg.Provided.SSHPublicKey || cfg.Provided.DeployUser || cfg.Provided.Timezone || cfg.Provided.Components || cfg.Provided.PlanFile || cfg.Provided.TraefikEmail || cfg.Provided.TraefikChallenge || cfg.Provided.TraefikDNSProvider || cfg.Provided.Servers {
		return fmt.Errorf("preview/apply only accept --plan-file, --yes, --non-interactive, and --help")
	}
	return nil
}

func sshAuthMethodLabel(value string) string {
	if value == sshAuthMethodPassword {
		return "Password"
	}

	return "SSH Key"
}
