package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

const version = "1.1.2"

const (
	commandApply   = "apply"
	commandPlan    = "plan"
	commandPreview = "preview"
	commandDoctor  = "doctor"
	commandVersion = "version"
	commandHelp    = "help"

	defaultSSHUser            = "root"
	defaultSSHPort            = 22
	defaultSSHPrivateKey      = "~/.ssh/id_ed25519"
	defaultSSHPublicKey       = "~/.ssh/id_ed25519.pub"
	defaultDeployUser         = "deployer"
	defaultTimezone           = "Asia/Jakarta"
	defaultSwapSize           = "2G"
	defaultTraefikChallenge   = "http"
	defaultTraefikDNSProvider = "cloudflare"
	runRootDirectory          = ".civa/runs"
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
	SSHPrivateKey      bool
	SSHPublicKey       bool
	DeployUser         bool
	Timezone           bool
	Components         bool
	PlanFile           bool
	TraefikEmail       bool
	TraefikChallenge   bool
	TraefikDNSProvider bool
	Servers            bool
	NonInteractive     bool
}

type config struct {
	Command            string
	NonInteractive     bool
	SSHUser            string
	SSHPort            int
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
	PlanFile           string
	Servers            []serverSpec
	Provided           providedFlags
}

type runtimeState struct {
	RunID           string
	GeneratedDir    string
	InventoryFile   string
	VarsFile        string
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
	{Number: 8, Value: "traefik", Label: "Reverse Proxy (Traefik)", Description: "Prepare /opt/traefik, Docker network, compose, and ACME env"},
}

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
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
		if err := finalizePaths(&cfg); err != nil {
			return err
		}
		return runDoctor(cfg)
	case commandApply, commandPlan, commandPreview:
		return runExecutionFlow(&cfg)
	default:
		return fmt.Errorf("unknown command: %s", cfg.Command)
	}
}

func defaultConfig(command string) config {
	return config{
		Command:            command,
		SSHUser:            defaultSSHUser,
		SSHPort:            defaultSSHPort,
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

func parseArgs(args []string) (config, error) {
	command := args[0]
	if command == "--help" || command == "-h" {
		return defaultConfig(commandHelp), nil
	}

	if !isKnownCommand(command) {
		return config{}, fmt.Errorf("unknown command: %s", command)
	}

	cfg := defaultConfig(command)

	for i := 1; i < len(args); i++ {
		arg := args[i]

		switch {
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
		default:
			return config{}, fmt.Errorf("unknown argument: %s", arg)
		}
	}

	return cfg, nil
}

func runExecutionFlow(cfg *config) error {
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

func shouldPromptApplyConfirmation(cfg config) bool {
	return cfg.Command == commandApply && !cfg.NonInteractive
}

func prepareRuntime(cfg *config) (*runtimeState, error) {
	runID := generateRunID(time.Now())
	generatedDir := filepath.Join(runRootDirectory, runID)
	inventoryFile := filepath.Join(generatedDir, "inventory.yml")
	varsFile := filepath.Join(generatedDir, "vars.yml")
	planFile := cfg.PlanFile
	if planFile == "" {
		planFile = filepath.Join(generatedDir, "plan.md")
	}
	playbookFile := filepath.Join(generatedDir, "ansible", "playbook.yml")

	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(planFile), 0o755); err != nil {
		return nil, err
	}
	if err := materializeAnsibleAssets(filepath.Join(generatedDir, "ansible")); err != nil {
		return nil, err
	}

	total := 4
	if cfg.Command == commandPlan {
		total = 3
	}

	return &runtimeState{
		RunID:           runID,
		GeneratedDir:    generatedDir,
		InventoryFile:   inventoryFile,
		VarsFile:        varsFile,
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

	state.progressStep("Writing Markdown execution plan")
	if err := writePlanFile(cfg, state); err != nil {
		return err
	}
	state.appendCompletedPhase("Markdown plan generated")

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
	}

	showExecutionSummary(cfg, state)
	fmt.Printf("Generated plan: %s\n", state.PlanFile)
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
	case commandApply, commandPlan, commandPreview, commandDoctor, commandVersion, commandHelp:
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

func validateExecutionConfig(cfg *config) error {
	if len(cfg.Servers) == 0 {
		return fmt.Errorf("at least one --server entry is required")
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
	if _, err := os.Stat(cfg.SSHPrivateKey); err != nil {
		return fmt.Errorf("SSH private key not found: %s", cfg.SSHPrivateKey)
	}
	if _, err := os.Stat(cfg.SSHPublicKey); err != nil {
		return fmt.Errorf("SSH public key not found: %s", cfg.SSHPublicKey)
	}
	if selectedComponentsInclude(cfg.Components, "traefik") {
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
	case "8", "traefik", "reverse-proxy", "reverse-proxy-traefik":
		return "traefik", nil
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

func printUsage() {
	const usage = `Usage:
  civa <command> [options]

Commands:
  apply                        Run the Ansible playbook against the selected servers
  plan                         Generate inventory, vars, and the execution plan only
  preview                      Run ansible-playbook in check mode with diff output
  doctor                       Check whether the local machine is ready to run civa
  version                      Show the civa version
  help                         Show this help message

Options:
  --non-interactive            Disable prompts and rely on provided flags
  --ssh-user <name>            SSH user used to connect to every target server
  --ssh-port <port>            SSH port used to connect to every target server
  --ssh-private-key <path>     Local private key path used by Ansible for SSH
  --ssh-public-key <path>      Local public key path that will be installed for the deploy user
  --deployer-user <name>       User created and configured on the target servers
  --timezone <tz>              Timezone applied to the target servers
  --components <list>          Components to run: all or a comma list such as 1,2,4 or docker,traefik
  --server <addr[,hostname]>   Add a target server; hostname is optional and becomes the server hostname
  --traefik-email <email>      Email used by Let's Encrypt ACME
  --traefik-challenge <type>   Traefik challenge type: http or dns
  --traefik-dns-provider <id>  DNS provider name used when challenge type is dns
  --output <path>              Markdown summary path (default: .civa/runs/<timestamp>/plan.md)
  --help                       Show this help message

Examples:
  civa apply
  civa preview --server 203.0.113.10,edge-01 --components all
  civa plan --non-interactive --server 203.0.113.10,web-01 --server 203.0.113.11,api-01 --components 1,2,3,4`

	fmt.Println(usage)
}

func componentLabel(value string) string {
	for _, option := range componentOptions {
		if option.Value == value {
			return option.Label
		}
	}
	return value
}
