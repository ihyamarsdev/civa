package cli

import (
	ansible "civa/ansible"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

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

	if ansible.HasEmbeddedTemplate("templates/traefik-compose.yml.j2") {
		fmt.Println("[ok] embedded Traefik compose template available")
	} else {
		fmt.Println("[fail] embedded Traefik compose template missing")
		failures++
	}

	if ansible.HasEmbeddedTemplate("templates/fail2ban-jail.local.j2") {
		fmt.Println("[ok] embedded Fail2Ban template available")
	} else {
		fmt.Println("[fail] embedded Fail2Ban template missing")
		failures++
	}

	if _, err := os.Stat(cfg.SSHPrivateKey); err == nil {
		fmt.Printf("[ok] SSH private key found: %s\n", cfg.SSHPrivateKey)
	} else {
		fmt.Printf("[fail] SSH private key not found: %s\n", cfg.SSHPrivateKey)
		failures++
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
	builder.WriteString("  children:\n")
	builder.WriteString("    civa_targets:\n")
	builder.WriteString("      hosts:\n")

	for i, server := range cfg.Servers {
		alias := uniqueInventoryAlias(server, i+1, usedAliases)
		fmt.Fprintf(&builder, "        %s:\n", alias)
		fmt.Fprintf(&builder, "          ansible_host: %q\n", server.Address)
		fmt.Fprintf(&builder, "          ansible_user: %q\n", cfg.SSHUser)
		fmt.Fprintf(&builder, "          ansible_port: %d\n", cfg.SSHPort)
		fmt.Fprintf(&builder, "          ansible_ssh_private_key_file: %q\n", cfg.SSHPrivateKey)
		if server.Hostname != "" {
			fmt.Fprintf(&builder, "          civa_target_hostname: %q\n", server.Hostname)
		}
	}

	return os.WriteFile(state.InventoryFile, []byte(builder.String()), 0o644)
}

func writeVarsFile(cfg *config, state *runtimeState) error {
	content := fmt.Sprintf(
		"civa_deployer_user: %q\n"+
			"civa_public_key_path: %q\n"+
			"civa_timezone: %q\n"+
			"civa_swap_size: %q\n"+
			"civa_swap_file: %q\n"+
			"civa_traefik_email: %q\n"+
			"civa_traefik_challenge: %q\n"+
			"civa_traefik_dns_provider: %q\n",
		cfg.DeployUser,
		cfg.SSHPublicKey,
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
			"- SSH user: %s\n"+
			"- SSH port: %d\n"+
			"- SSH private key: %s\n"+
			"- SSH public key: %s\n"+
			"- Deployer user: %s\n"+
			"- Timezone: %s\n\n"+
			"## Selected Components\n\n%s\n\n"+
			"## Target Servers\n\n%s\n\n"+
			"## Generated Files\n\n"+
			"- Inventory: %s\n"+
			"- Vars: %s\n"+
			"- Plan: %s\n"+
			"- Playbook: %s\n\n"+
			"## Command\n\n```bash\n%s\n```\n",
		cfg.Command,
		cfg.SSHUser,
		cfg.SSHPort,
		cfg.SSHPrivateKey,
		cfg.SSHPublicKey,
		cfg.DeployUser,
		cfg.Timezone,
		strings.Join(componentLines, "\n"),
		strings.Join(serverLines, "\n"),
		state.InventoryFile,
		state.VarsFile,
		state.PlanFile,
		state.PlaybookFile,
		buildAnsibleCommand(cfg, state),
	)

	return os.WriteFile(state.PlanFile, []byte(content), 0o644)
}

func runAnsible(cfg *config, state *runtimeState) error {
	args := []string{
		"-i", state.InventoryFile,
		state.PlaybookFile,
		"-e", "@" + state.VarsFile,
	}
	if len(cfg.Components) > 0 {
		args = append(args, "--tags", strings.Join(cfg.Components, ","))
	}
	if cfg.Command == commandPreview {
		args = append(args, "--check", "--diff")
	}

	cmd := exec.Command("ansible-playbook", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func buildAnsibleCommand(cfg *config, state *runtimeState) string {
	parts := []string{
		"ansible-playbook",
		"-i", state.InventoryFile,
		state.PlaybookFile,
		"-e", "@" + state.VarsFile,
	}
	if len(cfg.Components) > 0 {
		parts = append(parts, "--tags", strings.Join(cfg.Components, ","))
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
	fmt.Fprintf(os.Stderr, "SSH user: %s\n", cfg.SSHUser)
	fmt.Fprintf(os.Stderr, "SSH port: %d\n", cfg.SSHPort)
	fmt.Fprintf(os.Stderr, "SSH private key: %s\n", cfg.SSHPrivateKey)
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
	if selectedComponentsInclude(cfg.Components, "traefik") {
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
