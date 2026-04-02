package cli

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
	commandUninstall        = "uninstall"
	commandVersion          = "version"
	commandHelp             = "help"

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
	runRootDirectory          = ".civa/runs"
	latestPlanPointerFile     = ".civa/latest-plan"
	planActionStart           = "start"
	planActionList            = "list"
	planActionRemove          = "remove"
	doctorActionCheck         = "check"
	doctorActionFix           = "fix"
)

type serverSpec struct {
	Address  string
	Hostname string
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
	Command            string
	PlanAction         string
	DoctorAction       string
	PlanName           string
	AssumeYes          bool
	NonInteractive     bool
	SSHUser            string
	SSHPort            int
	SSHAuthMethod      string
	SSHPassword        string
	WebServer          string
	SSHPrivateKey      string
	SSHPublicKey       string
	DeployUser         string
	Timezone           string
	SwapSize           string
	TraefikEmail       string
	TraefikChallenge   string
	TraefikDNSProvider string
	ComponentsInput    string
	Components         []string
	PlanInputFile      string
	PlanFile           string
	Servers            []serverSpec
	Provided           providedFlags
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

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	if args[0] == commandCompleteInternal {
		return runHiddenCompletion(args[1:])
	}

	if args[0] == commandCompletion {
		return runCompletionCommand(args[1:])
	}

	if shouldShowCommandHelp(args) {
		printCommandUsage(args[0])
		return nil
	}

	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}

	switch cfg.Command {
	case commandHelp:
		printUsage()
		return nil
	case commandVersion:
		fmt.Printf("civa %s\n", version)
		return nil
	case commandDoctor:
		return runDoctor(cfg)
	case commandSetup:
		return runSetupFlow(&cfg)
	case commandCompletion:
		return runCompletionCommand(args[1:])
	case commandUninstall:
		return runUninstall(cfg)
	case commandPlan:
		switch cfg.PlanAction {
		case planActionList:
			return runPlanListFlow(&cfg)
		case planActionRemove:
			return runPlanRemoveFlow(&cfg)
		default:
			return runPlanFlow(&cfg)
		}
	case commandPreview:
		return runPreviewFlow(&cfg)
	case commandApply:
		return runApplyFlow(&cfg)
	default:
		return fmt.Errorf("unknown command: %s", cfg.Command)
	}
}

func defaultConfig(command string) config {
	return config{
		Command:            command,
		PlanAction:         planActionStart,
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

func shouldShowCommandHelp(args []string) bool {
	if len(args) == 1 {
		switch args[0] {
		case commandPlan, commandPreview, commandApply, commandCompletion:
			return true
		}
	}

	if len(args) == 2 && (args[1] == "help" || args[1] == "--help" || args[1] == "-h") {
		switch args[0] {
		case commandPlan, commandPreview, commandApply, commandCompletion, commandSetup, commandDoctor:
			return true
		}
	}

	return false
}

func parseArgs(args []string) (config, error) {
	command := args[0]
	if command == "--help" || command == "-h" {
		return defaultConfig(commandHelp), nil
	}

	if !isKnownCommand(command) {
		return config{}, fmt.Errorf("unknown command: %s", command)
	}

	cfg := defaultConfig(command)
	startIndex := 1

	if command == commandPlan {
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			switch args[1] {
			case planActionStart, planActionList, planActionRemove:
				cfg.PlanAction = args[1]
				startIndex = 2
			default:
				return config{}, fmt.Errorf("unknown plan subcommand: %s", args[1])
			}
		}
	}

	if command == commandDoctor {
		if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
			switch args[1] {
			case doctorActionFix:
				cfg.DoctorAction = doctorActionFix
				startIndex = 2
			case doctorActionCheck:
				cfg.DoctorAction = doctorActionCheck
				startIndex = 2
			default:
				return config{}, fmt.Errorf("unknown doctor subcommand: %s", args[1])
			}
		}
	}

	for i := startIndex; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--yes" || arg == "-y":
			cfg.AssumeYes = true
		case arg == "--non-interactive":
			cfg.NonInteractive = true
			cfg.Provided.NonInteractive = true
		case arg == "--help" || arg == "-h":
			cfg.Command = commandHelp
			return cfg, nil
		case arg == "--ssh-user":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.SSHUser = value
			cfg.Provided.SSHUser = true
			i = nextIndex
		case strings.HasPrefix(arg, "--ssh-user="):
			cfg.SSHUser = strings.TrimPrefix(arg, "--ssh-user=")
			cfg.Provided.SSHUser = true
		case arg == "--ssh-port":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			port, err := strconv.Atoi(value)
			if err != nil {
				return config{}, fmt.Errorf("--ssh-port must be an integer")
			}
			cfg.SSHPort = port
			cfg.Provided.SSHPort = true
			i = nextIndex
		case strings.HasPrefix(arg, "--ssh-port="):
			port, err := strconv.Atoi(strings.TrimPrefix(arg, "--ssh-port="))
			if err != nil {
				return config{}, fmt.Errorf("--ssh-port must be an integer")
			}
			cfg.SSHPort = port
			cfg.Provided.SSHPort = true
		case arg == "--ssh-private-key":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.SSHPrivateKey = value
			cfg.Provided.SSHPrivateKey = true
			i = nextIndex
		case strings.HasPrefix(arg, "--ssh-private-key="):
			cfg.SSHPrivateKey = strings.TrimPrefix(arg, "--ssh-private-key=")
			cfg.Provided.SSHPrivateKey = true
		case arg == "--ssh-auth-method":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.SSHAuthMethod = strings.ToLower(value)
			cfg.Provided.SSHAuthMethod = true
			i = nextIndex
		case strings.HasPrefix(arg, "--ssh-auth-method="):
			cfg.SSHAuthMethod = strings.ToLower(strings.TrimPrefix(arg, "--ssh-auth-method="))
			cfg.Provided.SSHAuthMethod = true
		case arg == "--ssh-password":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.SSHPassword = value
			cfg.Provided.SSHPassword = true
			i = nextIndex
		case strings.HasPrefix(arg, "--ssh-password="):
			cfg.SSHPassword = strings.TrimPrefix(arg, "--ssh-password=")
			cfg.Provided.SSHPassword = true
		case arg == "--web-server":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.WebServer = strings.ToLower(value)
			cfg.Provided.WebServer = true
			i = nextIndex
		case strings.HasPrefix(arg, "--web-server="):
			cfg.WebServer = strings.ToLower(strings.TrimPrefix(arg, "--web-server="))
			cfg.Provided.WebServer = true
		case arg == "--ssh-public-key":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.SSHPublicKey = value
			cfg.Provided.SSHPublicKey = true
			i = nextIndex
		case strings.HasPrefix(arg, "--ssh-public-key="):
			cfg.SSHPublicKey = strings.TrimPrefix(arg, "--ssh-public-key=")
			cfg.Provided.SSHPublicKey = true
		case arg == "--deployer-user":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.DeployUser = value
			cfg.Provided.DeployUser = true
			i = nextIndex
		case strings.HasPrefix(arg, "--deployer-user="):
			cfg.DeployUser = strings.TrimPrefix(arg, "--deployer-user=")
			cfg.Provided.DeployUser = true
		case arg == "--timezone":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.Timezone = value
			cfg.Provided.Timezone = true
			i = nextIndex
		case strings.HasPrefix(arg, "--timezone="):
			cfg.Timezone = strings.TrimPrefix(arg, "--timezone=")
			cfg.Provided.Timezone = true
		case arg == "--components":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.ComponentsInput = value
			cfg.Provided.Components = true
			i = nextIndex
		case strings.HasPrefix(arg, "--components="):
			cfg.ComponentsInput = strings.TrimPrefix(arg, "--components=")
			cfg.Provided.Components = true
		case arg == "--plan-file":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.PlanInputFile = value
			cfg.Provided.PlanInputFile = true
			i = nextIndex
		case strings.HasPrefix(arg, "--plan-file="):
			cfg.PlanInputFile = strings.TrimPrefix(arg, "--plan-file=")
			cfg.Provided.PlanInputFile = true
		case arg == "--server":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			server, err := parseServerSpec(value)
			if err != nil {
				return config{}, err
			}
			cfg.Servers = append(cfg.Servers, server)
			cfg.Provided.Servers = true
			i = nextIndex
		case strings.HasPrefix(arg, "--server="):
			server, err := parseServerSpec(strings.TrimPrefix(arg, "--server="))
			if err != nil {
				return config{}, err
			}
			cfg.Servers = append(cfg.Servers, server)
			cfg.Provided.Servers = true
		case arg == "--traefik-email":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.TraefikEmail = value
			cfg.Provided.TraefikEmail = true
			i = nextIndex
		case strings.HasPrefix(arg, "--traefik-email="):
			cfg.TraefikEmail = strings.TrimPrefix(arg, "--traefik-email=")
			cfg.Provided.TraefikEmail = true
		case arg == "--traefik-challenge":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.TraefikChallenge = strings.ToLower(value)
			cfg.Provided.TraefikChallenge = true
			i = nextIndex
		case strings.HasPrefix(arg, "--traefik-challenge="):
			cfg.TraefikChallenge = strings.ToLower(strings.TrimPrefix(arg, "--traefik-challenge="))
			cfg.Provided.TraefikChallenge = true
		case arg == "--traefik-dns-provider":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.TraefikDNSProvider = value
			cfg.Provided.TraefikDNSProvider = true
			i = nextIndex
		case strings.HasPrefix(arg, "--traefik-dns-provider="):
			cfg.TraefikDNSProvider = strings.TrimPrefix(arg, "--traefik-dns-provider=")
			cfg.Provided.TraefikDNSProvider = true
		case arg == "--output":
			value, nextIndex, err := nextArgValue(args, i)
			if err != nil {
				return config{}, err
			}
			cfg.PlanFile = value
			cfg.Provided.PlanFile = true
			i = nextIndex
		case strings.HasPrefix(arg, "--output="):
			cfg.PlanFile = strings.TrimPrefix(arg, "--output=")
			cfg.Provided.PlanFile = true
		case command == commandPlan && cfg.PlanAction == planActionRemove && cfg.PlanName == "" && !strings.HasPrefix(arg, "-"):
			cfg.PlanName = arg
		case (command == commandPreview || command == commandApply) && cfg.PlanName == "" && !strings.HasPrefix(arg, "-"):
			cfg.PlanName = arg
		default:
			return config{}, fmt.Errorf("unknown argument: %s", arg)
		}
	}

	return cfg, nil
}

func runPlanListFlow(_ *config) error {
	return listPlans()
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
			fmt.Fprintln(os.Stderr, "civa apply was cancelled by the user before ansible-playbook started.")
			return nil
		}
	}

	loadedCfg.Command = commandApply
	state.ProgressCurrent = 0
	state.ProgressTotal = 1
	state.CompletedPhases = nil

	printSection("Apply Existing Plan")
	fmt.Fprintf(os.Stderr, "Plan file: %s\n", planPath)
	fmt.Fprintf(os.Stderr, "Inventory: %s\n", state.InventoryFile)
	fmt.Fprintf(os.Stderr, "Vars: %s\n", state.VarsFile)
	if state.AuthFile != "" {
		fmt.Fprintf(os.Stderr, "SSH auth file: %s\n", state.AuthFile)
	}
	fmt.Fprintf(os.Stderr, "Playbook: %s\n", state.PlaybookFile)

	state.progressStep("Running ansible-playbook from existing plan")
	if err := runAnsible(loadedCfg, state); err != nil {
		return err
	}
	state.appendCompletedPhase("ansible-playbook execution")
	if err := syncSSHConfigAfterApply(loadedCfg, state); err != nil {
		return err
	}
	showExecutionSummary(loadedCfg, state)
	return nil
}

func shouldPromptApplyConfirmation(cfg config) bool {
	return cfg.Command == commandApply && !cfg.NonInteractive
}

func prepareRuntime(cfg *config) (*runtimeState, error) {
	runID := generateRunID(time.Now())
	generatedDir := filepath.Join(runRootDirectory, runID)
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

func nextArgValue(args []string, index int) (string, int, error) {
	nextIndex := index + 1
	if nextIndex >= len(args) {
		return "", index, fmt.Errorf("missing value for %s", args[index])
	}
	return args[nextIndex], nextIndex, nil
}

func isKnownCommand(command string) bool {
	switch command {
	case commandApply, commandPlan, commandPreview, commandDoctor, commandSetup, commandCompletion, commandUninstall, commandVersion, commandHelp:
		return true
	default:
		return false
	}
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
	parts := strings.SplitN(raw, ",", 2)
	address := strings.TrimSpace(parts[0])
	if address == "" {
		return serverSpec{}, fmt.Errorf("--server requires an address or IP")
	}
	server := serverSpec{Address: address}
	if len(parts) == 2 {
		server.Hostname = strings.TrimSpace(parts[1])
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
		tags = append(tags, component)
	}
	if isValidWebServer(cfg.WebServer) && cfg.WebServer != webServerNone {
		tags = append(tags, cfg.WebServer)
	}
	return tags
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
	lines := []string{
		"Usage:",
		"  civa <command> [options]",
		"",
		"Commands:",
		"  apply <plan-name>          Execute an existing generated plan",
		"  plan start                 Generate inventory, vars, and the execution plan only",
		"  plan list                  List generated plans",
		"  plan remove <plan-name>    Remove a generated plan and its artifacts",
		"  preview <plan-name>        Show an existing generated plan",
		"  setup                      Install a public SSH key on a server with ssh-copy-id",
		"  completion <shell>         Print shell completion for bash, zsh, or fish",
		"  doctor [fix]               Check or install local dependencies for civa",
		"  uninstall                  Remove the currently installed civa binary",
		"  version                    Show the civa version",
		"  help                       Show this help message",
		"",
		"Options:",
		"  --non-interactive          Disable prompts and rely on provided flags",
		"  --yes, -y                  Skip confirmation prompts for destructive commands",
		"  --ssh-user <name>          SSH user used to connect to every target server",
		"  --ssh-port <port>          SSH port used to connect to every target server",
		"  --ssh-password <value>     SSH password used by civa setup",
		"  --web-server <name>        Web server to prepare: traefik, nginx, caddy, or none",
		"  --ssh-private-key <path>   Local private key path used by Ansible for SSH",
		"  --ssh-public-key <path>    Local public key path that will be installed for the deploy user",
		"  --deployer-user <name>     User created and configured on the target servers",
		"  --timezone <tz>            Timezone applied to the target servers",
		"  --components <list>        Components to run: all or a comma list such as 1,2,4 or docker,traefik",
		"  --plan-file <path>         Existing plan file override used by preview or apply",
		"  --server <addr[,hostname]> Add a target server; hostname is optional and becomes the server hostname",
		"  --traefik-email <email>    Email used by Let's Encrypt ACME",
		"  --traefik-challenge <type> Traefik challenge type: http or dns",
		"  --traefik-dns-provider <id> DNS provider name used when challenge type is dns",
		"  --output <path>            Extra exported Markdown copy for plan start",
		"  --help                     Show this help message",
		"",
		"Examples:",
		"  civa plan start --non-interactive --server 203.0.113.10,web-01 --server 203.0.113.11,api-01 --components 1,2,3,4",
		"  civa plan list",
		"  civa preview 20260401-152334-210329559",
		"  civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub",
		"  civa doctor",
		"  civa doctor fix",
		"  civa completion bash",
		"  civa apply 20260401-152334-210329559 --yes",
		"  civa plan remove 20260401-152334-210329559 --yes",
		"  civa uninstall --yes",
	}

	fmt.Println(strings.Join(lines, "\n"))
}

func printCommandUsage(command string) {
	switch command {
	case commandPlan:
		fmt.Println(`Usage:
  civa plan start [options]
  civa plan list
  civa plan remove <plan-name> [--yes]

Subcommands:
  start                        Generate a new named plan under .civa/runs/
  list                         List generated plans
  remove <plan-name>           Remove a generated plan and its artifacts

Examples:
  civa plan start --non-interactive --server 203.0.113.10,web-01 --components all
  civa plan list
  civa plan remove 20260401-160900-040074544 --yes`)
	case commandPreview:
		fmt.Println(`Usage:
  civa preview <plan-name>
  civa preview --plan-file <path>

Examples:
  civa preview 20260401-160900-040074544
  civa preview --plan-file .civa/runs/20260401-160900-040074544/plan.md`)
	case commandApply:
		fmt.Println(`Usage:
  civa apply <plan-name> [--yes]
  civa apply --plan-file <path> [--yes]

Examples:
  civa apply 20260401-160900-040074544 --yes
  civa apply --plan-file .civa/runs/20260401-160900-040074544/plan.md --yes`)
	case commandCompletion:
		fmt.Println(`Usage:
  civa completion <shell>

Supported shells:
  bash
  zsh
  fish

Examples:
  civa completion bash
  civa completion zsh
  civa completion fish`)
	case commandDoctor:
		fmt.Println(`Usage:
  civa doctor
  civa doctor fix

Subcommands:
  fix                          Install or update required local dependencies

Required minimum versions:
  go >= 1.26
  ansible-playbook >= 2.20
  python3 (or python) >= 3.10

Examples:
  civa doctor
  civa doctor fix`)
	case commandSetup:
		fmt.Println(`Usage:
  civa setup [options]

Required options:
  --server <addr>
  --ssh-user <name>
  --ssh-public-key <path>

Optional:
  --ssh-password <value>

Examples:
  civa setup --server 203.0.113.10 --ssh-user root --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub
  civa setup --server 203.0.113.10 --ssh-user root --ssh-public-key ~/.ssh/id_ed25519.pub
  civa setup --server 203.0.113.10 --ssh-user ubuntu --ssh-port 2222 --ssh-password 'secret' --ssh-public-key ~/.ssh/id_ed25519.pub`)
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

func validateExistingPlanCommandFlags(cfg config) error {
	if cfg.Provided.SSHUser || cfg.Provided.SSHPort || cfg.Provided.SSHAuthMethod || cfg.Provided.SSHPassword || cfg.Provided.WebServer || cfg.Provided.SSHPrivateKey || cfg.Provided.SSHPublicKey || cfg.Provided.DeployUser || cfg.Provided.Timezone || cfg.Provided.Components || cfg.Provided.TraefikEmail || cfg.Provided.TraefikChallenge || cfg.Provided.TraefikDNSProvider || cfg.Provided.Servers {
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
