package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
)

var errUserCancelled = errors.New("user cancelled prompt")

func collectInteractiveInputs(cfg *config) error {
	fmt.Fprintln(os.Stderr, "")
	printSection("Interactive Setup")
	logLine("civa will ask for the minimum data needed to generate inventory, vars, and the Ansible run plan.")

	if !cfg.Provided.Components {
		components, err := promptComponents()
		if err != nil {
			return err
		}
		cfg.Components = components
	}

	if len(cfg.Servers) == 0 {
		printSection("Step 2/6 - Server Count")
		serverCount, err := promptPositiveInt("How many servers will be targeted?", 1)
		if err != nil {
			return err
		}
		for i := 1; i <= serverCount; i++ {
			printSection(fmt.Sprintf("Server %d", i))
			address, err := promptNonEmptyString("Server IP or address", "")
			if err != nil {
				return err
			}
			hostname, err := promptString("Hostname to apply on the server (optional)", "", false)
			if err != nil {
				return err
			}
			cfg.Servers = append(cfg.Servers, serverSpec{Address: address, Hostname: hostname})
		}
	}

	if !cfg.Provided.SSHUser || !cfg.Provided.SSHPort || !cfg.Provided.SSHAuthMethod || !cfg.Provided.SSHPassword || !cfg.Provided.SSHPrivateKey || !cfg.Provided.SSHPublicKey {
		printSection("Step 3/6 - SSH Access")
		if !cfg.Provided.SSHUser {
			value, err := promptNonEmptyString("SSH user for the initial connection", cfg.SSHUser)
			if err != nil {
				return err
			}
			cfg.SSHUser = value
		}
		if !cfg.Provided.SSHPort {
			value, err := promptPort("SSH port for the initial connection", cfg.SSHPort)
			if err != nil {
				return err
			}
			cfg.SSHPort = value
		}
		if !cfg.Provided.SSHAuthMethod {
			value, err := promptSSHAuthMethod(cfg.SSHAuthMethod)
			if err != nil {
				return err
			}
			cfg.SSHAuthMethod = value
		}
		if cfg.SSHAuthMethod == sshAuthMethodPassword && !cfg.Provided.SSHPassword {
			value, err := promptSSHPassword()
			if err != nil {
				return err
			}
			cfg.SSHPassword = value
		}
		if cfg.SSHAuthMethod == sshAuthMethodKey && !cfg.Provided.SSHPrivateKey {
			value, err := promptNonEmptyString("Local SSH private key path used by Ansible", cfg.SSHPrivateKey)
			if err != nil {
				return err
			}
			cfg.SSHPrivateKey = value
		}
		if !cfg.Provided.SSHPublicKey {
			value, err := promptNonEmptyString("Local SSH public key path to install for the deployer user", cfg.SSHPublicKey)
			if err != nil {
				return err
			}
			cfg.SSHPublicKey = value
		}
	}

	if !cfg.Provided.DeployUser {
		printSection("Step 4/6 - Target User")
		value, err := promptNonEmptyString("Deployer username to create on the servers", cfg.DeployUser)
		if err != nil {
			return err
		}
		cfg.DeployUser = value
	}

	if !cfg.Provided.Timezone {
		printSection("Step 5/6 - System Defaults")
		value, err := promptNonEmptyString("Timezone to apply on all servers", cfg.Timezone)
		if err != nil {
			return err
		}
		cfg.Timezone = value
	}

	if err := resolveConfigComponents(cfg); err != nil {
		return err
	}
	normalizeWebServerSelection(cfg)

	if selectedComponentsInclude(cfg.Components, "web_server") && !cfg.Provided.WebServer {
		printSection("Step 6/6 - Web Server")
		value, err := promptWebServer(cfg.WebServer)
		if err != nil {
			return err
		}
		cfg.WebServer = value
		normalizeWebServerSelection(cfg)
	}

	if cfg.WebServer == webServerTraefik {
		printSection("Step 7/7 - Traefik")
		if !cfg.Provided.TraefikEmail {
			value, err := promptNonEmptyString("ACME email for Traefik", cfg.TraefikEmail)
			if err != nil {
				return err
			}
			cfg.TraefikEmail = value
		}
		if !cfg.Provided.TraefikChallenge {
			value, err := promptChallengeType(cfg.TraefikChallenge)
			if err != nil {
				return err
			}
			cfg.TraefikChallenge = value
		}
		if cfg.TraefikChallenge == "dns" && !cfg.Provided.TraefikDNSProvider {
			value, err := promptNonEmptyString("Traefik DNS provider name", cfg.TraefikDNSProvider)
			if err != nil {
				return err
			}
			cfg.TraefikDNSProvider = value
		}
	}

	if !cfg.Provided.PlanFile {
		value, err := promptString("Optional Markdown summary path (blank for default run directory)", cfg.PlanFile, false)
		if err != nil {
			return err
		}
		cfg.PlanFile = value
	}

	return nil
}

func promptComponents() ([]string, error) {
	selectedComponents := defaultComponentValues()
	options := make([]huh.Option[string], 0, len(componentOptions))
	for _, option := range componentOptions {
		label := fmt.Sprintf("%s - %s", option.Label, option.Description)
		options = append(options, huh.NewOption(label, option.Value).Selected(true))
	}

	field := huh.NewMultiSelect[string]().
		Title("Choose which components to run").
		Description("Use the arrow keys to move, press Space to toggle, then Enter to confirm.").
		Options(options...).
		Filterable(false).
		Validate(func(values []string) error {
			if len(values) == 0 {
				return fmt.Errorf("at least one component must be selected")
			}
			return nil
		}).
		Value(&selectedComponents).
		WithKeyMap(newComponentPromptKeyMap())

	if err := field.Run(); err != nil {
		return nil, normalizePromptError(err)
	}
	return selectedComponents, nil
}

func promptApplyConfirmation() (bool, error) {
	confirmed := true
	field := huh.NewConfirm().
		Title("Continue with ansible apply after reviewing the summary?").
		Value(&confirmed)
	if err := field.Run(); err != nil {
		return false, normalizePromptError(err)
	}
	return confirmed, nil
}

func promptApplyExistingPlanConfirmation(planPath string) (bool, error) {
	confirmed := true
	field := huh.NewConfirm().
		Title(fmt.Sprintf("Execute the generated plan at %s?", planPath)).
		Description("This runs ansible-playbook with the artifacts recorded in the existing plan.").
		Value(&confirmed)
	if err := field.Run(); err != nil {
		return false, normalizePromptError(err)
	}
	return confirmed, nil
}

func promptPlanFilePath(defaultValue string) (string, error) {
	value := defaultValue
	field := huh.NewInput().
		Title("Existing plan file path").
		Value(&value).
		Validate(func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("this value cannot be empty")
			}
			return nil
		})
	if err := field.Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return strings.TrimSpace(value), nil
}

func promptPlanRemovalConfirmation(planName string) (bool, error) {
	confirmed := false
	field := huh.NewConfirm().
		Title(fmt.Sprintf("Remove generated plan %s?", planName)).
		Description("This deletes the stored run directory and its generated artifacts.").
		Value(&confirmed)
	if err := field.Run(); err != nil {
		return false, normalizePromptError(err)
	}
	return confirmed, nil
}

func promptUninstallConfirmation(targetPath string) (bool, error) {
	confirmed := false
	field := huh.NewConfirm().
		Title(fmt.Sprintf("Remove the civa binary at %s?", targetPath)).
		Description("This deletes the currently running civa executable from disk.").
		Value(&confirmed)
	if err := field.Run(); err != nil {
		return false, normalizePromptError(err)
	}
	return confirmed, nil
}

func promptString(title, defaultValue string, requireNonEmpty bool) (string, error) {
	value := defaultValue
	field := huh.NewInput().
		Title(title).
		Value(&value)
	if requireNonEmpty {
		field = field.Validate(func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("this value cannot be empty")
			}
			return nil
		})
	}
	if err := field.Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return strings.TrimSpace(value), nil
}

func promptNonEmptyString(title, defaultValue string) (string, error) {
	return promptString(title, defaultValue, true)
}

func promptPositiveInt(title string, defaultValue int) (int, error) {
	value := strconv.Itoa(defaultValue)
	field := huh.NewInput().
		Title(title).
		Value(&value).
		Validate(func(input string) error {
			parsed, err := strconv.Atoi(strings.TrimSpace(input))
			if err != nil || parsed <= 0 {
				return fmt.Errorf("please enter a positive integer")
			}
			return nil
		})
	if err := field.Run(); err != nil {
		return 0, normalizePromptError(err)
	}
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed, nil
}

func promptPort(title string, defaultValue int) (int, error) {
	value := strconv.Itoa(defaultValue)
	field := huh.NewInput().
		Title(title).
		Value(&value).
		Validate(func(input string) error {
			parsed, err := strconv.Atoi(strings.TrimSpace(input))
			if err != nil || parsed < 1 || parsed > 65535 {
				return fmt.Errorf("port must be an integer between 1 and 65535")
			}
			return nil
		})
	if err := field.Run(); err != nil {
		return 0, normalizePromptError(err)
	}
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed, nil
}

func promptChallengeType(defaultValue string) (string, error) {
	value := defaultValue
	field := huh.NewSelect[string]().
		Title("Traefik challenge type").
		Options(
			huh.NewOption("http", "http"),
			huh.NewOption("dns", "dns"),
		).
		Value(&value)
	if err := field.Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return value, nil
}

func promptWebServer(defaultValue string) (string, error) {
	value := defaultValue
	field := huh.NewSelect[string]().
		Title("Web server to prepare").
		Options(
			huh.NewOption("None", webServerNone),
			huh.NewOption("Traefik", webServerTraefik),
			huh.NewOption("Nginx", webServerNginx),
			huh.NewOption("Caddy", webServerCaddy),
		).
		Value(&value)
	if err := field.Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return value, nil
}

func promptSSHAuthMethod(defaultValue string) (string, error) {
	value := defaultValue
	field := huh.NewSelect[string]().
		Title("SSH authentication method").
		Options(
			huh.NewOption("SSH key", sshAuthMethodKey),
			huh.NewOption("Password", sshAuthMethodPassword),
		).
		Value(&value)
	if err := field.Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return value, nil
}

func promptSSHPassword() (string, error) {
	value := ""
	field := huh.NewInput().
		Title("SSH password for the initial connection").
		EchoMode(huh.EchoModePassword).
		Value(&value).
		Validate(func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("this value cannot be empty")
			}
			return nil
		})
	if err := field.Run(); err != nil {
		return "", normalizePromptError(err)
	}
	return value, nil
}

func newComponentPromptKeyMap() *huh.KeyMap {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.MultiSelect.Toggle = key.NewBinding(
		key.WithKeys(" ", "x"),
		key.WithHelp("space", "toggle"),
	)
	return keyMap
}

func normalizePromptError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "interrupt") || strings.Contains(message, "ctrl+c") || strings.Contains(message, "user aborted") {
		return errUserCancelled
	}
	return err
}
