package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

func TestResolveComponentsSupportsMixedTokens(t *testing.T) {
	components, err := resolveComponents("1,docker,security")
	if err != nil {
		t.Fatalf("resolveComponents returned error: %v", err)
	}

	want := []string{"system_update", "containerization", "security_firewall"}
	if strings.Join(components, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected components: got %v want %v", components, want)
	}
}

func TestIsValidSSHAuthMethod(t *testing.T) {
	if !isValidSSHAuthMethod(sshAuthMethodKey) {
		t.Fatal("expected key auth method to be valid")
	}
	if !isValidSSHAuthMethod(sshAuthMethodPassword) {
		t.Fatal("expected password auth method to be valid")
	}
	if isValidSSHAuthMethod("token") {
		t.Fatal("expected token auth method to be invalid")
	}
}

func TestNormalizeWebServerSelectionInfersAndAddsWebServerComponent(t *testing.T) {
	cfg := &config{ComponentsInput: "nginx", Components: []string{"web_server"}}
	normalizeWebServerSelection(cfg)
	if cfg.WebServer != webServerNginx {
		t.Fatalf("unexpected inferred web server: %s", cfg.WebServer)
	}

	cfg = &config{WebServer: webServerCaddy, Provided: providedFlags{WebServer: true}, Components: []string{"system_update"}}
	normalizeWebServerSelection(cfg)
	if !selectedComponentsInclude(cfg.Components, "web_server") {
		t.Fatalf("expected explicit web server to add web_server component, got %v", cfg.Components)
	}

	cfg = &config{WebServer: webServerNone, Provided: providedFlags{WebServer: true}, Components: []string{"system_update", "web_server"}}
	normalizeWebServerSelection(cfg)
	if selectedComponentsInclude(cfg.Components, "web_server") {
		t.Fatalf("expected web_server component to be removed for none, got %v", cfg.Components)
	}
}

func TestSelectedAnsibleTagsUsesConcreteWebServerTag(t *testing.T) {
	tags := selectedAnsibleTags(config{Components: []string{"system_update", "web_server"}, WebServer: webServerCaddy})
	if strings.Join(tags, ",") != "system_update,caddy" {
		t.Fatalf("unexpected ansible tags: %v", tags)
	}
}

func TestParseServerSpecSupportsHostname(t *testing.T) {
	server, err := parseServerSpec("203.0.113.10,web-01")
	if err != nil {
		t.Fatalf("parseServerSpec returned error: %v", err)
	}

	if server.Address != "203.0.113.10" || server.Hostname != "web-01" {
		t.Fatalf("unexpected server spec: %#v", server)
	}
}

func TestShouldShowCommandHelpForBarePlanPreviewApply(t *testing.T) {
	for _, args := range [][]string{{"plan"}, {"preview"}, {"apply"}, {"plan", "help"}, {"preview", "--help"}, {"apply", "-h"}} {
		if !shouldShowCommandHelp(args) {
			t.Fatalf("expected help for args %v", args)
		}
	}

	for _, args := range [][]string{{"plan", "start"}, {"preview", "my-plan"}, {"apply", "my-plan", "--yes"}} {
		if shouldShowCommandHelp(args) {
			t.Fatalf("did not expect help for args %v", args)
		}
	}
}

func TestParseArgsSupportsPlanSubcommandsAndPlanNames(t *testing.T) {
	planCfg, err := parseArgs([]string{"plan", "start", "--non-interactive"})
	if err != nil {
		t.Fatalf("parseArgs returned error for plan start: %v", err)
	}
	if planCfg.PlanAction != planActionStart {
		t.Fatalf("unexpected plan action: %s", planCfg.PlanAction)
	}

	listCfg, err := parseArgs([]string{"plan", "list"})
	if err != nil {
		t.Fatalf("parseArgs returned error for plan list: %v", err)
	}
	if listCfg.PlanAction != planActionList {
		t.Fatalf("unexpected plan list action: %s", listCfg.PlanAction)
	}

	removeCfg, err := parseArgs([]string{"plan", "remove", "my-plan", "--yes"})
	if err != nil {
		t.Fatalf("parseArgs returned error for plan remove: %v", err)
	}
	if removeCfg.PlanAction != planActionRemove || removeCfg.PlanName != "my-plan" {
		t.Fatalf("unexpected plan remove config: %#v", removeCfg)
	}

	previewCfg, err := parseArgs([]string{"preview", "my-plan"})
	if err != nil {
		t.Fatalf("parseArgs returned error for preview: %v", err)
	}
	if previewCfg.PlanName != "my-plan" {
		t.Fatalf("unexpected preview plan name: %s", previewCfg.PlanName)
	}

	applyCfg, err := parseArgs([]string{"apply", "my-plan", "--yes"})
	if err != nil {
		t.Fatalf("parseArgs returned error for apply: %v", err)
	}
	if applyCfg.PlanName != "my-plan" {
		t.Fatalf("unexpected apply plan name: %s", applyCfg.PlanName)
	}
}

func TestUninstallTargetPathForPathValidatesBinaryName(t *testing.T) {
	path, err := uninstallTargetPathForPath("/usr/local/bin/civa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/usr/local/bin/civa" {
		t.Fatalf("unexpected uninstall path: %s", path)
	}

	if _, err := uninstallTargetPathForPath("/usr/local/bin/not-civa"); err == nil {
		t.Fatal("expected uninstall target validation to reject a non-civa binary")
	}
}

func TestSanitizeAlias(t *testing.T) {
	alias := sanitizeAlias("My Host@1")
	if alias != "my-host-1" {
		t.Fatalf("unexpected alias: %s", alias)
	}
}

func TestComponentPromptKeyMapToggleUsesSpace(t *testing.T) {
	keyMap := newComponentPromptKeyMap()
	toggleKeys := keyMap.MultiSelect.Toggle.Keys()
	if len(toggleKeys) != 2 || toggleKeys[0] != " " || toggleKeys[1] != "x" {
		t.Fatalf("unexpected toggle keys: %#v", toggleKeys)
	}
	help := keyMap.MultiSelect.Toggle.Help()
	if help.Key != "space" || help.Desc != "toggle" {
		t.Fatalf("unexpected help text: %#v", help)
	}
}

func TestComponentPromptSpaceToggleDeselectsFocusedOption(t *testing.T) {
	selectedComponents := defaultComponentValues()
	options := make([]huh.Option[string], 0, len(componentOptions))
	for _, option := range componentOptions {
		options = append(options, huh.NewOption(option.Label, option.Value).Selected(true))
	}

	field := huh.NewMultiSelect[string]().
		Options(options...).
		Filterable(false).
		Value(&selectedComponents)
	field.WithKeyMap(newComponentPromptKeyMap())
	field.Focus()

	field.Update(tea.KeyMsg{Type: tea.KeyDown})
	field.Update(tea.KeyMsg{Type: tea.KeyDown})
	field.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})

	if selectedComponentsInclude(selectedComponents, "ssh_hardening") {
		t.Fatalf("expected ssh_hardening to be deselected, got %v", selectedComponents)
	}
}

func TestShouldPromptApplyConfirmationRespectsNonInteractive(t *testing.T) {
	if shouldPromptApplyConfirmation(config{Command: commandApply, NonInteractive: true}) {
		t.Fatal("expected non-interactive apply to skip confirmation prompt")
	}

	if !shouldPromptApplyConfirmation(config{Command: commandApply, NonInteractive: false}) {
		t.Fatal("expected interactive apply to prompt for confirmation")
	}
}

func TestValidateExistingPlanCommandFlagsRejectsPlanGenerationFlags(t *testing.T) {
	if err := validateExistingPlanCommandFlags(config{Provided: providedFlags{Servers: true}}); err == nil {
		t.Fatal("expected preview/apply validation to reject server flags")
	}

	if err := validateExistingPlanCommandFlags(config{Provided: providedFlags{PlanInputFile: true}}); err != nil {
		t.Fatalf("expected --plan-file override to remain valid, got %v", err)
	}

	if err := validateExistingPlanCommandFlags(config{Provided: providedFlags{}}); err != nil {
		t.Fatalf("expected empty provided flags to be valid, got %v", err)
	}
}

func TestUniqueInventoryAliasAvoidsCollisions(t *testing.T) {
	used := map[string]int{}
	first := uniqueInventoryAlias(serverSpec{Hostname: "Web 01"}, 1, used)
	second := uniqueInventoryAlias(serverSpec{Hostname: "Web@01"}, 2, used)

	if first != "web-01" {
		t.Fatalf("unexpected first alias: %s", first)
	}
	if second != "web-01-02" {
		t.Fatalf("unexpected second alias: %s", second)
	}
}

func TestBuildAnsibleCommandQuotesPathsWithSpaces(t *testing.T) {
	cfg := &config{
		Command:    commandPlan,
		Components: []string{"system_update", "user_management"},
	}
	state := &runtimeState{
		InventoryFile: "/tmp/civa test/inventory.yml",
		PlaybookFile:  "/tmp/civa test/ansible/main.yml",
		VarsFile:      "/tmp/civa test/vars.yml",
		AuthFile:      "/tmp/civa test/auth.yml",
	}

	command := buildAnsibleCommand(cfg, state)
	if !strings.Contains(command, `"/tmp/civa test/inventory.yml"`) {
		t.Fatalf("expected quoted inventory path, got %s", command)
	}
	if !strings.Contains(command, `"/tmp/civa test/ansible/main.yml"`) {
		t.Fatalf("expected quoted playbook path, got %s", command)
	}
	if !strings.Contains(command, `"ANSIBLE_COLLECTIONS_PATH=/tmp/civa test/ansible/collections"`) {
		t.Fatalf("expected quoted collections path, got %s", command)
	}
	if !strings.Contains(command, `@"/tmp/civa test/vars.yml"`) && !strings.Contains(command, `"@/tmp/civa test/vars.yml"`) {
		t.Fatalf("expected quoted vars path, got %s", command)
	}
	if !strings.Contains(command, `@"/tmp/civa test/auth.yml"`) && !strings.Contains(command, `"@/tmp/civa test/auth.yml"`) {
		t.Fatalf("expected quoted auth path, got %s", command)
	}
	if !strings.Contains(command, "--tags system_update,user_management") {
		t.Fatalf("expected tags in command, got %s", command)
	}
}

func TestSSHCredentialSummaryHidesPassword(t *testing.T) {
	passwordSummary := sshCredentialSummary(config{SSHAuthMethod: sshAuthMethodPassword, SSHPassword: "super-secret"})
	if passwordSummary != "[hidden password]" {
		t.Fatalf("unexpected password summary: %s", passwordSummary)
	}

	keySummary := sshCredentialSummary(config{SSHAuthMethod: sshAuthMethodKey, SSHPrivateKey: "/tmp/id_test"})
	if keySummary != "/tmp/id_test" {
		t.Fatalf("unexpected key summary: %s", keySummary)
	}
}

func TestGenerateRunIDIncludesNanoseconds(t *testing.T) {
	runID := generateRunID(time.Date(2026, time.April, 1, 11, 14, 29, 123456789, time.UTC))
	if runID != "20260401-111429-123456789" {
		t.Fatalf("unexpected run ID: %s", runID)
	}
}

func TestMaterializeAnsibleAssetsWritesEmbeddedFiles(t *testing.T) {
	tempDir := t.TempDir()
	ansibleDir := filepath.Join(tempDir, "ansible")

	if err := materializeAnsibleAssets(ansibleDir); err != nil {
		t.Fatalf("materializeAnsibleAssets returned error: %v", err)
	}

	paths := []string{
		filepath.Join(ansibleDir, "main.yml"),
		filepath.Join(ansibleDir, "collections", "ansible_collections", "civa", "traefik", "roles", "traefik", "templates", "traefik.env.j2"),
		filepath.Join(ansibleDir, "collections", "ansible_collections", "civa", "traefik", "roles", "traefik", "templates", "traefik-compose.yml.j2"),
		filepath.Join(ansibleDir, "collections", "ansible_collections", "civa", "security_firewall", "roles", "security_firewall", "templates", "fail2ban-jail.local.j2"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected embedded asset at %s: %v", path, err)
		}
	}
}

func TestWriteInventoryUsesPasswordForPasswordAuth(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config{
		SSHAuthMethod: sshAuthMethodPassword,
		SSHPassword:   "super-secret",
		SSHUser:       "root",
		SSHPort:       22,
		Servers: []serverSpec{{
			Address:  "203.0.113.10",
			Hostname: "web-01",
		}},
	}
	state := &runtimeState{InventoryFile: filepath.Join(tempDir, "inventory.yml")}

	if err := writeInventory(cfg, state); err != nil {
		t.Fatalf("writeInventory returned error: %v", err)
	}

	content, err := os.ReadFile(state.InventoryFile)
	if err != nil {
		t.Fatalf("failed to read inventory: %v", err)
	}

	inventory := string(content)
	if strings.Contains(inventory, "ansible_ssh_private_key_file") {
		t.Fatalf("did not expect key auth in password inventory, got %s", inventory)
	}
	if strings.Contains(inventory, "ansible_password") {
		t.Fatalf("did not expect plaintext password in inventory, got %s", inventory)
	}

	state.AuthFile = filepath.Join(tempDir, "auth.yml")
	if err := writeAuthFile(cfg, state); err != nil {
		t.Fatalf("writeAuthFile returned error: %v", err)
	}

	authContent, err := os.ReadFile(state.AuthFile)
	if err != nil {
		t.Fatalf("failed to read auth file: %v", err)
	}

	authVars := string(authContent)
	if !strings.Contains(authVars, `ansible_password: "super-secret"`) {
		t.Fatalf("expected password auth in auth file, got %s", authVars)
	}
}

func TestLoadPlannedRunParsesGeneratedArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	inventoryFile := filepath.Join(tempDir, "inventory.yml")
	varsFile := filepath.Join(tempDir, "vars.yml")
	authFile := filepath.Join(tempDir, "auth.yml")
	playbookFile := filepath.Join(tempDir, "ansible", "main.yml")
	planFile := filepath.Join(tempDir, "plan.md")

	if err := os.MkdirAll(filepath.Dir(playbookFile), 0o755); err != nil {
		t.Fatalf("failed to create playbook dir: %v", err)
	}
	for _, file := range []string{inventoryFile, varsFile, authFile, playbookFile} {
		if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
			t.Fatalf("failed to write %s: %v", file, err)
		}
	}

	planContent := strings.Join([]string{
		"# civa Run Plan",
		"",
		"## Mode",
		"",
		"- Command: plan",
		"- SSH auth method: Password",
		"- SSH user: root",
		"- SSH port: 22",
		"",
		"## Selected Components",
		"",
		"- System Update & Upgrade",
		"- User Management",
		"",
		"## Generated Files",
		"",
		"- Inventory: " + inventoryFile,
		"- Vars: " + varsFile,
		"- SSH auth file: " + authFile,
		"- Plan: " + planFile,
		"- Playbook: " + playbookFile,
	}, "\n")
	if err := os.WriteFile(planFile, []byte(planContent), 0o644); err != nil {
		t.Fatalf("failed to write plan file: %v", err)
	}

	loadedCfg, state, err := loadPlannedRun(planFile)
	if err != nil {
		t.Fatalf("loadPlannedRun returned error: %v", err)
	}
	if loadedCfg.SSHAuthMethod != sshAuthMethodPassword {
		t.Fatalf("unexpected auth method: %s", loadedCfg.SSHAuthMethod)
	}
	if strings.Join(loadedCfg.Components, ",") != "system_update,user_management" {
		t.Fatalf("unexpected components: %v", loadedCfg.Components)
	}
	if state.InventoryFile != inventoryFile || state.VarsFile != varsFile || state.AuthFile != authFile || state.PlaybookFile != playbookFile {
		t.Fatalf("unexpected planned state: %#v", state)
	}
}

func TestResolvePlanInputFileRequiresNameOrPlanFile(t *testing.T) {
	cfg := &config{Command: commandPreview, NonInteractive: true}
	_, err := resolvePlanInputFile(cfg)
	if err == nil || !strings.Contains(err.Error(), "require a generated plan name or --plan-file") {
		t.Fatalf("expected name-or-plan-file error, got %v", err)
	}
}

func TestResolvePlanInputFileUsesPlanName(t *testing.T) {
	workingDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()
	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	planName := "example-plan"
	planPath := planPathForName(planName)
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatalf("failed to create plan dir: %v", err)
	}
	if err := os.WriteFile(planPath, []byte("# plan\n"), 0o644); err != nil {
		t.Fatalf("failed to write plan: %v", err)
	}

	cfg := &config{Command: commandPreview, PlanName: planName}
	resolved, err := resolvePlanInputFile(cfg)
	if err != nil {
		t.Fatalf("resolvePlanInputFile returned error: %v", err)
	}
	if resolved != planPath {
		t.Fatalf("unexpected resolved plan path: %s", resolved)
	}
}

func TestRunApplyFlowRequiresYesWhenNonInteractive(t *testing.T) {
	tempDir := t.TempDir()
	planPath := filepath.Join(tempDir, "plan.md")
	metadataPath := planMetadataPath(planPath)
	inventoryFile := filepath.Join(tempDir, "inventory.yml")
	varsFile := filepath.Join(tempDir, "vars.yml")
	playbookFile := filepath.Join(tempDir, "ansible", "main.yml")

	if err := os.MkdirAll(filepath.Dir(playbookFile), 0o755); err != nil {
		t.Fatalf("failed to create playbook dir: %v", err)
	}
	for _, file := range []string{planPath, inventoryFile, varsFile, playbookFile} {
		if err := os.WriteFile(file, []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", file, err)
		}
	}

	metadata := `{
  "sshAuthMethod": "key",
  "sshUser": "root",
  "sshPort": 22,
  "components": ["system_update"],
  "inventoryFile": "` + inventoryFile + `",
  "varsFile": "` + varsFile + `",
  "planFile": "` + planPath + `",
  "playbookFile": "` + playbookFile + `"
}` + "\n"
	if err := os.WriteFile(metadataPath, []byte(metadata), 0o644); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	err := runApplyFlow(&config{Command: commandApply, NonInteractive: true, PlanInputFile: planPath})
	if err == nil || !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("expected --yes gate error, got %v", err)
	}
}
