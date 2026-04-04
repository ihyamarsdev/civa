package infra

import (
	"bytes"
	infssh "civa/internal/cli/infra/ssh"
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

func TestFinalizePathsDerivesPublicKeyFromPrivateKey(t *testing.T) {
	cfg := &config{SSHPrivateKey: "~/example_key"}
	if err := finalizePaths(cfg); err != nil {
		t.Fatalf("finalizePaths returned error: %v", err)
	}
	if !strings.HasSuffix(cfg.SSHPublicKey, "example_key.pub") {
		t.Fatalf("expected derived public key path, got %s", cfg.SSHPublicKey)
	}
}

func TestFinalizePathsKeepsProvidedPublicKey(t *testing.T) {
	cfg := &config{
		SSHPrivateKey: "~/example_key",
		SSHPublicKey:  "~/custom_key.pub",
		Provided:      providedFlags{SSHPublicKey: true},
	}
	if err := finalizePaths(cfg); err != nil {
		t.Fatalf("finalizePaths returned error: %v", err)
	}
	if !strings.HasSuffix(cfg.SSHPublicKey, "custom_key.pub") {
		t.Fatalf("expected custom public key path to be preserved, got %s", cfg.SSHPublicKey)
	}
}

func TestValidateExecutionConfigRejectsPasswordModeForPlan(t *testing.T) {
	tempDir := t.TempDir()
	privateKey := filepath.Join(tempDir, "id_test")
	publicKey := privateKey + ".pub"
	for _, path := range []string{privateKey, publicKey} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	cfg := &config{
		SSHUser:       "root",
		SSHPort:       22,
		WebServer:     webServerNone,
		SSHPrivateKey: privateKey,
		SSHPublicKey:  publicKey,
		SSHPassword:   "secret",
		DeployUser:    "deployer",
		Timezone:      "Asia/Jakarta",
		Components:    []string{"system_update"},
		Servers:       []serverSpec{{Address: "203.0.113.10"}},
		Provided:      providedFlags{SSHPassword: true},
	}

	err := validateExecutionConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "only supports SSH key auth") {
		t.Fatalf("expected key-only plan validation error, got %v", err)
	}
}

func TestBuildSSHCopyIDCommand(t *testing.T) {
	cfg := config{
		SSHUser:      "root",
		SSHPort:      2222,
		SSHPassword:  "secret",
		SSHPublicKey: "/tmp/id_test.pub",
		Servers:      []serverSpec{{Address: "203.0.113.10"}},
	}

	cmd := buildSSHCopyIDCommand(cfg)
	gotArgs := strings.Join(cmd.Args, " ")
	if !strings.Contains(gotArgs, "ssh-copy-id -i /tmp/id_test.pub -p 2222 -o StrictHostKeyChecking=accept-new root@203.0.113.10") {
		t.Fatalf("unexpected ssh-copy-id args: %s", gotArgs)
	}
	found := false
	for _, env := range cmd.Env {
		if env == "SSHPASS=secret" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected SSHPASS environment variable")
	}
}

func TestBuildSSHCopyIDCommandUsesInitiationPortWhenServerCustomPortProvided(t *testing.T) {
	cfg := config{
		SSHUser:      "root",
		SSHPort:      2222,
		SSHPublicKey: "/tmp/id_test.pub",
		Servers:      []serverSpec{{Address: "203.0.113.10", SSHPort: 2201}},
	}

	cmd := buildSSHCopyIDCommand(cfg)
	gotArgs := strings.Join(cmd.Args, " ")
	if !strings.Contains(gotArgs, "ssh-copy-id -i /tmp/id_test.pub -p 2222 -o StrictHostKeyChecking=accept-new root@203.0.113.10") {
		t.Fatalf("expected initiation ssh port in command args, got %s", gotArgs)
	}
}

func TestBuildSSHCopyIDCommandWithoutPasswordUsesSSHCopyIDDirectly(t *testing.T) {
	cfg := config{
		SSHUser:      "root",
		SSHPort:      2222,
		SSHPublicKey: "/tmp/id_test.pub",
		Servers:      []serverSpec{{Address: "203.0.113.10"}},
	}

	cmd := buildSSHCopyIDCommand(cfg)
	if filepath.Base(cmd.Path) != "ssh-copy-id" {
		t.Fatalf("expected direct ssh-copy-id invocation, got %s", cmd.Path)
	}
	if len(cmd.Env) != 0 {
		t.Fatalf("expected no custom env for direct ssh-copy-id, got %v", cmd.Env)
	}
}

func TestRewriteKnownHostEntryInHomeRewritesMatchingHostOnly(t *testing.T) {
	homeDir := t.TempDir()
	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatalf("failed to create ssh dir: %v", err)
	}
	knownHostsPath := filepath.Join(sshDir, "known_hosts")
	content := strings.Join([]string{
		"203.0.113.10,host-a ssh-ed25519 AAAA",
		"[203.0.113.11]:2222 ssh-ed25519 BBBB",
		"203.0.113.12 ssh-ed25519 CCCC",
		"",
	}, "\n")
	if err := os.WriteFile(knownHostsPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write known_hosts: %v", err)
	}

	if err := infssh.RewriteKnownHostEntryInHome(homeDir, "203.0.113.10", 22); err != nil {
		t.Fatalf("RewriteKnownHostEntryInHome returned error: %v", err)
	}
	if err := infssh.RewriteKnownHostEntryInHome(homeDir, "203.0.113.11", 2222); err != nil {
		t.Fatalf("RewriteKnownHostEntryInHome returned error: %v", err)
	}

	rewritten, err := os.ReadFile(knownHostsPath)
	if err != nil {
		t.Fatalf("failed to read rewritten known_hosts: %v", err)
	}
	text := string(rewritten)
	if strings.Contains(text, "203.0.113.10") || strings.Contains(text, "[203.0.113.11]:2222") {
		t.Fatalf("expected matching host entries removed, got: %q", text)
	}
	if !strings.Contains(text, "host-a ssh-ed25519 AAAA") || !strings.Contains(text, "203.0.113.12 ssh-ed25519 CCCC") {
		t.Fatalf("expected non-target host entries retained, got: %q", text)
	}
	if _, err := os.Stat(filepath.Join(sshDir, "known_hosts.old")); !os.IsNotExist(err) {
		t.Fatalf("expected no known_hosts.old backup file, got err=%v", err)
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
	if strings.Join(tags, ",") != "system_update,web_server_caddy" {
		t.Fatalf("unexpected ansible tags: %v", tags)
	}
}

func TestSelectedAnsibleTagsNormalizesNonWebServerComponentTags(t *testing.T) {
	tags := selectedAnsibleTags(config{Components: []string{"dependencies", "containerization"}})
	if strings.Join(tags, ",") != "system_dependencies,system_containerization" {
		t.Fatalf("unexpected normalized ansible tags: %v", tags)
	}
}

func TestParseServerSpecSupportsHostname(t *testing.T) {
	server, err := parseServerSpec("203.0.113.10,web-01")
	if err != nil {
		t.Fatalf("parseServerSpec returned error: %v", err)
	}

	if server.Address != "203.0.113.10" || server.Hostname != "web-01" || server.SSHPort != 0 {
		t.Fatalf("unexpected server spec: %#v", server)
	}
}

func TestParseServerSpecSupportsCustomSSHPort(t *testing.T) {
	server, err := parseServerSpec("203.0.113.10,2202")
	if err != nil {
		t.Fatalf("parseServerSpec returned error: %v", err)
	}
	if server.Address != "203.0.113.10" || server.Hostname != "" || server.SSHPort != 2202 {
		t.Fatalf("unexpected server spec for address+port: %#v", server)
	}

	server, err = parseServerSpec("203.0.113.10,web-01,2203")
	if err != nil {
		t.Fatalf("parseServerSpec returned error: %v", err)
	}
	if server.Address != "203.0.113.10" || server.Hostname != "web-01" || server.SSHPort != 2203 {
		t.Fatalf("unexpected server spec for address+hostname+port: %#v", server)
	}
}

func TestParseServerSpecRejectsInvalidSSHPort(t *testing.T) {
	if _, err := parseServerSpec("203.0.113.10,web-01,abc"); err == nil {
		t.Fatal("expected parseServerSpec to reject non-integer ssh port")
	}
	if _, err := parseServerSpec("203.0.113.10,web-01,70000"); err == nil {
		t.Fatal("expected parseServerSpec to reject out-of-range ssh port")
	}
}

func TestParseOptionalPortInputSupportsBlankAndValidPort(t *testing.T) {
	port, err := parseOptionalPortInput("")
	if err != nil {
		t.Fatalf("expected blank input to be accepted, got error: %v", err)
	}
	if port != 0 {
		t.Fatalf("expected blank input to map to 0, got %d", port)
	}

	port, err = parseOptionalPortInput("2201")
	if err != nil {
		t.Fatalf("expected valid port input, got error: %v", err)
	}
	if port != 2201 {
		t.Fatalf("expected parsed port 2201, got %d", port)
	}
}

func TestParseOptionalPortInputRejectsInvalidPort(t *testing.T) {
	for _, input := range []string{"abc", "0", "70000"} {
		if _, err := parseOptionalPortInput(input); err == nil {
			t.Fatalf("expected invalid port input %q to fail", input)
		}
	}
}

func TestRenderPreviewMarkdownUsesPlainFallbackForNonTTY(t *testing.T) {
	rendered, err := renderPreviewMarkdown("plan.md", []byte("# Hello\n\nBody text\n"), false)
	if err != nil {
		t.Fatalf("renderPreviewMarkdown returned error: %v", err)
	}
	if !strings.Contains(rendered, "Hello") || !strings.Contains(rendered, "Body text") {
		t.Fatalf("unexpected rendered output: %q", rendered)
	}
}

func TestRenderPreviewMarkdownRemovesFrontmatter(t *testing.T) {
	rendered, err := renderPreviewMarkdown("plan.md", []byte("---\ntitle: Demo\n---\n# Hello\n"), false)
	if err != nil {
		t.Fatalf("renderPreviewMarkdown returned error: %v", err)
	}
	if strings.Contains(rendered, "title: Demo") {
		t.Fatalf("expected frontmatter to be removed, got %q", rendered)
	}
	if !strings.Contains(rendered, "Hello") {
		t.Fatalf("expected heading to remain, got %q", rendered)
	}
}

func TestPreviewHeaderDependsOnTTY(t *testing.T) {
	if header := previewHeader("plan.md", false); header != "" {
		t.Fatalf("expected no header for non-tty, got %q", header)
	}
	if header := previewHeader("plan.md", true); !strings.Contains(header, "Plan file: plan.md") {
		t.Fatalf("expected plan header for tty, got %q", header)
	}
}

func TestRenderSectionTitlePlainFallback(t *testing.T) {
	rendered := renderSectionTitle("Execution Summary", false)
	if !strings.Contains(rendered, "== Execution Summary ==") {
		t.Fatalf("expected plain section fallback, got %q", rendered)
	}
}

func TestRenderOutputBlocksPlainFallback(t *testing.T) {
	rendered := renderOutputBlocks([]outputBlock{
		{Title: "Usage", Lines: []string{"civa apply <plan-name>"}},
		{Title: "Examples", Lines: []string{"civa apply demo --yes"}},
	}, false)
	if !strings.Contains(rendered, "Usage:\nciva apply <plan-name>") {
		t.Fatalf("expected usage block in fallback output, got %q", rendered)
	}
	if !strings.Contains(rendered, "Examples:\nciva apply demo --yes") {
		t.Fatalf("expected examples block in fallback output, got %q", rendered)
	}
}

func TestCompletionSuggestionsTopLevelAndValues(t *testing.T) {
	root := completionSuggestions(nil)
	if !contains(root, commandPlan) || !contains(root, commandCompletion) {
		t.Fatalf("unexpected root completion set: %v", root)
	}

	webServerValues := completionSuggestions([]string{"plan", "start", "--web-server", "c"})
	if len(webServerValues) != 1 || webServerValues[0] != webServerCaddy {
		t.Fatalf("unexpected web server value suggestions: %v", webServerValues)
	}

	doctorSuggestions := completionSuggestions([]string{"doctor", "f"})
	if len(doctorSuggestions) != 1 || doctorSuggestions[0] != doctorActionFix {
		t.Fatalf("unexpected doctor suggestions: %v", doctorSuggestions)
	}

	applySuggestions := completionSuggestions([]string{"apply", "r"})
	if !contains(applySuggestions, applyActionReview) {
		t.Fatalf("expected apply review suggestion, got %v", applySuggestions)
	}
}

func TestCompletionSuggestionsIncludeGeneratedPlanNames(t *testing.T) {
	workingDir := t.TempDir()
	t.Setenv("HOME", workingDir)
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir: %v", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()
	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	for _, planName := range []string{"20260101-010101-000000001", "20260101-010102-000000002"} {
		planPath := planPathForName(planName)
		if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
			t.Fatalf("failed to create plan dir: %v", err)
		}
		if err := os.WriteFile(planPath, []byte("# plan\n"), 0o644); err != nil {
			t.Fatalf("failed to write plan: %v", err)
		}
	}

	suggestions := completionSuggestions([]string{commandPreview, "20260101-010101"})
	if !contains(suggestions, "20260101-010101-000000001") {
		t.Fatalf("expected generated plan name suggestion, got %v", suggestions)
	}
}

func TestDoctorVersionParsingAndComparison(t *testing.T) {
	if got := extractFirstSemver("go version go1.26.1 linux/amd64"); got != "1.26.1" {
		t.Fatalf("unexpected go semver extraction: %s", got)
	}
	if got := extractFirstSemver("ansible-playbook [core 2.20.0]\n"); got != "2.20.0" {
		t.Fatalf("unexpected ansible semver extraction: %s", got)
	}
	if got := extractFirstSemver("Python 3.12.3"); got != "3.12.3" {
		t.Fatalf("unexpected python semver extraction: %s", got)
	}

	if !versionAtLeast("1.26.0", "1.26") {
		t.Fatal("expected 1.26.0 >= 1.26")
	}
	if !versionAtLeast("2.20.1", "2.20") {
		t.Fatal("expected 2.20.1 >= 2.20")
	}
	if versionAtLeast("3.9.18", "3.10") {
		t.Fatal("expected 3.9.18 < 3.10")
	}
}

func TestDoctorPackagesForManager(t *testing.T) {
	aptPackages := strings.Join(doctorPackagesForManager("apt-get"), ",")
	if aptPackages != "golang-go,ansible,python3" {
		t.Fatalf("unexpected apt packages: %s", aptPackages)
	}

	dnfPackages := strings.Join(doctorPackagesForManager("dnf"), ",")
	if dnfPackages != "golang,ansible,python3" {
		t.Fatalf("unexpected dnf packages: %s", dnfPackages)
	}

	if len(doctorPackagesForManager("apk")) != 0 {
		t.Fatal("expected unsupported package manager to return no packages")
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

	if shouldPromptApplyConfirmation(config{Command: commandApply, ApplyAction: applyActionReview, NonInteractive: false}) {
		t.Fatal("expected apply review to skip confirmation prompt")
	}
}

func TestValidateExistingPlanCommandFlagsRejectsPlanGenerationFlags(t *testing.T) {
	if err := validateExistingPlanCommandFlags(config{Provided: providedFlags{Servers: true}}); err == nil {
		t.Fatal("expected preview/apply validation to reject server flags")
	}

	if err := validateExistingPlanCommandFlags(config{Provided: providedFlags{PlanFile: true}}); err == nil {
		t.Fatal("expected preview/apply validation to reject --output flag")
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
	if !strings.Contains(command, `"ANSIBLE_ROLES_PATH=/tmp/civa test/ansible/roles"`) {
		t.Fatalf("expected quoted roles path, got %s", command)
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

func TestBuildAnsibleCommandIncludesCheckModeForApplyReview(t *testing.T) {
	cfg := &config{
		Command:     commandApply,
		ApplyAction: applyActionReview,
	}
	state := &runtimeState{
		InventoryFile: "/tmp/inventory.yml",
		PlaybookFile:  "/tmp/ansible/main.yml",
		VarsFile:      "/tmp/vars.yml",
	}

	command := buildAnsibleCommand(cfg, state)
	if !strings.Contains(command, "--check --diff") {
		t.Fatalf("expected check mode flags in apply review command, got %s", command)
	}
}

func TestAnsibleProgressDescription(t *testing.T) {
	if got := ansibleProgressDescription(config{Command: commandApply, ApplyAction: applyActionExecute}); got != "Running ansible-playbook" {
		t.Fatalf("unexpected apply progress description: %q", got)
	}

	if got := ansibleProgressDescription(config{Command: commandApply, ApplyAction: applyActionReview}); got != "Reviewing server state" {
		t.Fatalf("unexpected apply review progress description: %q", got)
	}
}

func TestShouldUseAnsibleProgressBar(t *testing.T) {
	if !shouldUseAnsibleProgressBar(config{Command: commandApply}, true, true) {
		t.Fatal("expected interactive apply to use progress bar")
	}

	if !shouldUseAnsibleProgressBar(config{Command: commandApply, ApplyAction: applyActionReview}, true, true) {
		t.Fatal("expected interactive apply review to use progress bar")
	}

	if shouldUseAnsibleProgressBar(config{Command: commandPreview}, true, true) {
		t.Fatal("expected preview to skip progress bar")
	}

	if shouldUseAnsibleProgressBar(config{Command: commandApply}, false, true) {
		t.Fatal("expected non-tty stdout to skip progress bar")
	}

	if shouldUseAnsibleProgressBar(config{Command: commandApply}, true, false) {
		t.Fatal("expected non-tty stderr to skip progress bar")
	}
}

func TestObserveOutputLineTracksAnsibleTasks(t *testing.T) {
	controller := &ansibleProgressBarController{}

	controller.ObserveOutputLine("TASK [Gathering Facts] *********************************************************")

	if controller.discoveredTasks != 1 {
		t.Fatalf("expected discovered tasks to be 1, got %d", controller.discoveredTasks)
	}
	if controller.completedTasks != 0 {
		t.Fatalf("expected completed tasks to be 0, got %d", controller.completedTasks)
	}
	if !controller.activeTask {
		t.Fatal("expected active task to be true")
	}
	if controller.currentTask != "Gathering Facts" {
		t.Fatalf("unexpected current task: %s", controller.currentTask)
	}

	controller.ObserveOutputLine("TASK [Install Docker] **********************************************************")

	if controller.discoveredTasks != 2 {
		t.Fatalf("expected discovered tasks to be 2, got %d", controller.discoveredTasks)
	}
	if controller.completedTasks != 1 {
		t.Fatalf("expected completed tasks to be 1, got %d", controller.completedTasks)
	}
	if controller.currentTask != "Install Docker" {
		t.Fatalf("unexpected current task after second task: %s", controller.currentTask)
	}

	controller.ObserveOutputLine("PLAY RECAP *********************************************************************")

	if controller.discoveredTasks != 2 {
		t.Fatalf("expected discovered tasks to remain 2 after recap, got %d", controller.discoveredTasks)
	}
	if controller.completedTasks != 2 {
		t.Fatalf("expected completed tasks to be 2 after recap, got %d", controller.completedTasks)
	}
	if controller.activeTask {
		t.Fatal("expected active task to be false after recap")
	}
}

func TestObserveOutputLineSupportsANSIWrappedTaskHeader(t *testing.T) {
	controller := &ansibleProgressBarController{}

	controller.ObserveOutputLine("\x1b[0;32mTASK [Configure firewall] **************************************\x1b[0m")

	if controller.discoveredTasks != 1 {
		t.Fatalf("expected ansi task line to be parsed, discovered=%d", controller.discoveredTasks)
	}
	if controller.currentTask != "Configure firewall" {
		t.Fatalf("unexpected parsed task name from ansi line: %s", controller.currentTask)
	}
}

func TestCaptureCommandOutputWritesAndReportsLines(t *testing.T) {
	var target bytes.Buffer
	var observed []string

	err := captureCommandOutput(strings.NewReader("line-1\nline-2\n"), &target, func(line string) {
		observed = append(observed, line)
	})
	if err != nil {
		t.Fatalf("captureCommandOutput returned error: %v", err)
	}

	if got := target.String(); got != "line-1\nline-2\n" {
		t.Fatalf("unexpected captured output: %q", got)
	}

	expected := []string{"line-1", "line-2"}
	if len(observed) != len(expected) {
		t.Fatalf("unexpected observed line count: got=%d expected=%d", len(observed), len(expected))
	}
	for i := range expected {
		if observed[i] != expected[i] {
			t.Fatalf("unexpected observed line at %d: got=%q expected=%q", i, observed[i], expected[i])
		}
	}
}

func TestReviewScopeSummary(t *testing.T) {
	cfg := config{Components: []string{"system_update", "web_server"}, WebServer: webServerTraefik}
	if got := reviewScopeSummary(cfg); got != "system_update, web_server(traefik)" {
		t.Fatalf("unexpected review scope summary: %s", got)
	}
}

func TestExecutionSummaryLinesForApplyReview(t *testing.T) {
	tempDir := t.TempDir()
	inventoryPath := filepath.Join(tempDir, "inventory.yml")
	inventory := strings.Join([]string{
		"all:",
		"  children:",
		"    civa_targets:",
		"      hosts:",
		"        web-01:",
		"          ansible_host: \"203.0.113.50\"",
		"          ansible_user: \"root\"",
		"          ansible_port: 22",
	}, "\n") + "\n"
	if err := os.WriteFile(inventoryPath, []byte(inventory), 0o600); err != nil {
		t.Fatalf("failed to write inventory: %v", err)
	}

	cfg := &config{
		Command:       commandApply,
		ApplyAction:   applyActionReview,
		SSHAuthMethod: sshAuthMethodKey,
		Components:    []string{"system_update", "web_server"},
		WebServer:     webServerTraefik,
	}
	state := &runtimeState{
		InventoryFile:   inventoryPath,
		VarsFile:        filepath.Join(tempDir, "vars.yml"),
		PlanFile:        filepath.Join(tempDir, "plan.md"),
		PlaybookFile:    filepath.Join(tempDir, "ansible", "main.yml"),
		ProgressCurrent: 4,
		ProgressTotal:   4,
		CompletedPhases: []string{"Loaded plan metadata", "Prepared review scope", "Completed ansible review run", "Prepared detailed review summary"},
	}

	lines := executionSummaryLines(cfg, state)
	joined := strings.Join(lines, "\n")
	checks := []string{
		"Completed phases: 4/4",
		"Review mode: ansible-playbook executed with --check --diff (server changes were not applied)",
		"Review scope: system_update, web_server(traefik)",
		"Target hosts: 1 host(s): web-01",
		"Selected ansible tags: system_update, web_server_traefik",
		"SSH auth mode: SSH key auth",
		"Result: apply review completed and the current server state was checked against the saved plan.",
	}

	for _, check := range checks {
		if !strings.Contains(joined, check) {
			t.Fatalf("expected summary to contain %q, got:\n%s", check, joined)
		}
	}
}

func TestApplyArtifactLinesUseNeatIconSpacing(t *testing.T) {
	state := &runtimeState{
		InventoryFile: "/tmp/inventory.yml",
		VarsFile:      "/tmp/vars.yml",
		AuthFile:      "/tmp/auth.yml",
		PlaybookFile:  "/tmp/ansible/main.yml",
	}

	lines := applyArtifactLines("/tmp/plan.md", state)
	if len(lines) != 5 {
		t.Fatalf("expected 5 artifact lines with auth file, got %d", len(lines))
	}

	expectedPrefixes := []string{
		"📄  Plan file:",
		"🗂️  Inventory:",
		"🧩  Vars:",
		"🔐  SSH auth file:",
		"📜  Playbook:",
	}
	for i, prefix := range expectedPrefixes {
		if !strings.HasPrefix(lines[i], prefix) {
			t.Fatalf("expected line %d to start with %q, got %q", i, prefix, lines[i])
		}
	}
}

func TestApplyArtifactLinesSkipAuthWhenEmpty(t *testing.T) {
	state := &runtimeState{
		InventoryFile: "/tmp/inventory.yml",
		VarsFile:      "/tmp/vars.yml",
		PlaybookFile:  "/tmp/ansible/main.yml",
	}

	lines := applyArtifactLines("/tmp/plan.md", state)
	if len(lines) != 4 {
		t.Fatalf("expected 4 artifact lines without auth file, got %d", len(lines))
	}
	for _, line := range lines {
		if strings.Contains(line, "SSH auth file") {
			t.Fatalf("did not expect SSH auth file line when auth is empty, got %q", line)
		}
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

func TestResolveGeneratedPlanNameUsesPrimaryHostname(t *testing.T) {
	cfg := &config{
		NonInteractive: true,
		Servers:        []serverSpec{{Address: "203.0.113.10", Hostname: "Web-01"}},
	}

	planName, err := resolveGeneratedPlanName(cfg)
	if err != nil {
		t.Fatalf("resolveGeneratedPlanName returned error: %v", err)
	}
	if planName != "web-01" {
		t.Fatalf("expected hostname-based plan name web-01, got %s", planName)
	}
}

func TestResolveGeneratedPlanNameIncrementsVersionOnDuplicate(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	existingPlanName := "web-01"
	existingPlanPath := planPathForName(existingPlanName)
	if err := os.MkdirAll(filepath.Dir(existingPlanPath), 0o755); err != nil {
		t.Fatalf("failed to create existing plan dir: %v", err)
	}
	if err := os.WriteFile(existingPlanPath, []byte("# existing plan\n"), 0o644); err != nil {
		t.Fatalf("failed to create existing plan file: %v", err)
	}

	cfg := &config{
		NonInteractive: true,
		Servers:        []serverSpec{{Address: "203.0.113.10", Hostname: "web-01"}},
	}

	planName, err := resolveGeneratedPlanName(cfg)
	if err != nil {
		t.Fatalf("resolveGeneratedPlanName returned error: %v", err)
	}
	if planName != "web-01-v2" {
		t.Fatalf("expected duplicate hostname to generate versioned plan name web-01-v2, got %s", planName)
	}
}

func TestLatestPlanVersionNameFindsNewestVersion(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	plans := []string{"web-01", "web-01-v2", "web-01-v3", "api-01"}
	for _, planName := range plans {
		planPath := planPathForName(planName)
		if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
			t.Fatalf("failed to create plan dir: %v", err)
		}
		if err := os.WriteFile(planPath, []byte("# plan\n"), 0o644); err != nil {
			t.Fatalf("failed to write plan file: %v", err)
		}
	}

	latestName, found, err := latestPlanVersionName("web-01")
	if err != nil {
		t.Fatalf("latestPlanVersionName returned error: %v", err)
	}
	if !found {
		t.Fatal("expected to find latest version for web-01")
	}
	if latestName != "web-01-v3" {
		t.Fatalf("expected latest version web-01-v3, got %s", latestName)
	}
}

func TestResolvePlanInputFileUsesLatestVersionForBasePlanName(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	for _, planName := range []string{"web-01-v2", "web-01-v3"} {
		planPath := planPathForName(planName)
		if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
			t.Fatalf("failed to create plan dir: %v", err)
		}
		if err := os.WriteFile(planPath, []byte("# plan\n"), 0o644); err != nil {
			t.Fatalf("failed to write plan file: %v", err)
		}
	}

	cfg := &config{Command: commandPreview, PlanName: "web-01"}
	resolved, err := resolvePlanInputFile(cfg)
	if err != nil {
		t.Fatalf("resolvePlanInputFile returned error: %v", err)
	}

	expected := planPathForName("web-01-v3")
	if resolved != expected {
		t.Fatalf("expected latest plan path %s, got %s", expected, resolved)
	}
	if cfg.PlanName != "web-01-v3" {
		t.Fatalf("expected cfg.PlanName updated to latest version, got %s", cfg.PlanName)
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
		filepath.Join(ansibleDir, "roles", "web_server_traefik", "templates", "traefik.env.j2"),
		filepath.Join(ansibleDir, "roles", "web_server_traefik", "templates", "traefik-compose.yml.j2"),
		filepath.Join(ansibleDir, "roles", "security_firewall", "templates", "fail2ban-jail.local.j2"),
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
	if !strings.Contains(inventory, "ansible_python_interpreter: auto_silent") {
		t.Fatalf("expected auto_silent python interpreter discovery in inventory, got %s", inventory)
	}
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

func TestWriteInventoryUsesPrivateKeyForKeyAuth(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config{
		SSHAuthMethod: sshAuthMethodKey,
		SSHPrivateKey: "/tmp/id_ed25519",
		SSHUser:       "ubuntu",
		SSHPort:       2222,
		Servers: []serverSpec{{
			Address:  "203.0.113.20",
			Hostname: "app-01",
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
	if !strings.Contains(inventory, "ansible_python_interpreter: auto_silent") {
		t.Fatalf("expected auto_silent python interpreter discovery in inventory, got %s", inventory)
	}
	if !strings.Contains(inventory, `ansible_ssh_private_key_file: "/tmp/id_ed25519"`) {
		t.Fatalf("expected private key in key-auth inventory, got %s", inventory)
	}
}

func TestWriteInventoryUsesInitiationPortAndStoresCustomSSHPortForSSHConfig(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config{
		SSHAuthMethod: sshAuthMethodKey,
		SSHPrivateKey: "/tmp/id_ed25519",
		SSHUser:       "ubuntu",
		SSHPort:       2200,
		Servers: []serverSpec{
			{Address: "203.0.113.20", Hostname: "app-01", SSHPort: 2222},
			{Address: "203.0.113.21", Hostname: "app-02"},
		},
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
	if strings.Count(inventory, "ansible_port: 2200") != 2 {
		t.Fatalf("expected initiation ssh port for all inventory targets, got %s", inventory)
	}
	if !strings.Contains(inventory, "civa_custom_ssh_port: 2222") {
		t.Fatalf("expected custom ssh port metadata for ssh config sync, got %s", inventory)
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

func TestLoadPlannedRunMetadataNormalizesLegacyDotCivaPaths(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	runID := "20260402-153329-602964730"
	planDir := filepath.Join(homeDir, ".civa", "runs", runID)
	inventoryFile := filepath.Join(planDir, "inventory.yml")
	varsFile := filepath.Join(planDir, "vars.yml")
	playbookFile := filepath.Join(planDir, "ansible", "main.yml")
	planFile := filepath.Join(planDir, "plan.md")
	metadataFile := planMetadataPath(planFile)

	if err := os.MkdirAll(filepath.Dir(playbookFile), 0o755); err != nil {
		t.Fatalf("failed to create playbook dir: %v", err)
	}
	for _, file := range []string{inventoryFile, varsFile, playbookFile, planFile} {
		if err := os.WriteFile(file, []byte("test"), 0o600); err != nil {
			t.Fatalf("failed to write %s: %v", file, err)
		}
	}

	metadata := strings.Join([]string{
		"{",
		`  "webServer": "none",`,
		`  "sshAuthMethod": "key",`,
		`  "sshUser": "root",`,
		`  "sshPort": 22,`,
		`  "components": ["system_update"],`,
		`  "inventoryFile": ".civa/runs/` + runID + `/inventory.yml",`,
		`  "varsFile": ".civa/runs/` + runID + `/vars.yml",`,
		`  "planFile": ".civa/runs/` + runID + `/plan.md",`,
		`  "playbookFile": ".civa/runs/` + runID + `/ansible/main.yml"`,
		"}",
	}, "\n") + "\n"
	if err := os.WriteFile(metadataFile, []byte(metadata), 0o600); err != nil {
		t.Fatalf("failed to write metadata: %v", err)
	}

	loadedCfg, state, err := loadPlannedRun(planFile)
	if err != nil {
		t.Fatalf("loadPlannedRun returned error: %v", err)
	}
	if len(loadedCfg.Components) != 1 || loadedCfg.Components[0] != "system_update" {
		t.Fatalf("unexpected components: %v", loadedCfg.Components)
	}
	if state.InventoryFile != inventoryFile || state.VarsFile != varsFile || state.PlaybookFile != playbookFile || state.PlanFile != planFile {
		t.Fatalf("expected normalized artifact paths, got %#v", state)
	}
}

func TestLoadPlannedRunFromMarkdownNormalizesLegacyDotCivaPaths(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	runID := "20260403-000000-000000001"
	planDir := filepath.Join(homeDir, ".civa", "runs", runID)
	inventoryFile := filepath.Join(planDir, "inventory.yml")
	varsFile := filepath.Join(planDir, "vars.yml")
	playbookFile := filepath.Join(planDir, "ansible", "main.yml")
	planFile := filepath.Join(planDir, "plan.md")

	if err := os.MkdirAll(filepath.Dir(playbookFile), 0o755); err != nil {
		t.Fatalf("failed to create playbook dir: %v", err)
	}
	for _, file := range []string{inventoryFile, varsFile, playbookFile} {
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
		"- SSH auth method: SSH Key",
		"- SSH user: root",
		"- SSH port: 22",
		"",
		"## Selected Components",
		"",
		"- System Update & Upgrade",
		"",
		"## Generated Files",
		"",
		"- Inventory: .civa/runs/" + runID + "/inventory.yml",
		"- Vars: .civa/runs/" + runID + "/vars.yml",
		"- SSH auth file: not used",
		"- Playbook: .civa/runs/" + runID + "/ansible/main.yml",
	}, "\n") + "\n"
	if err := os.WriteFile(planFile, []byte(planContent), 0o600); err != nil {
		t.Fatalf("failed to write plan file: %v", err)
	}

	_, state, err := loadPlannedRun(planFile)
	if err != nil {
		t.Fatalf("loadPlannedRun returned error: %v", err)
	}
	if state.InventoryFile != inventoryFile || state.VarsFile != varsFile || state.PlaybookFile != playbookFile || state.PlanFile != planFile {
		t.Fatalf("expected normalized markdown artifact paths, got %#v", state)
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
	t.Setenv("HOME", workingDir)
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

func TestEnsureUserCivaDirectoryInHomeCreatesDotCiva(t *testing.T) {
	homeDir := t.TempDir()
	created, err := ensureUserCivaDirectoryInHome(homeDir)
	if err != nil {
		t.Fatalf("ensureUserCivaDirectoryInHome returned error: %v", err)
	}

	if created != filepath.Join(homeDir, strings.TrimPrefix(userCivaHomeDirectory, "~/")) {
		t.Fatalf("unexpected civa directory path: %s", created)
	}
	if info, statErr := os.Stat(created); statErr != nil {
		t.Fatalf("expected created civa directory, got error: %v", statErr)
	} else if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", created)
	}
}

func TestParseSSHConfigHostsFromInventory(t *testing.T) {
	tempDir := t.TempDir()
	inventoryPath := filepath.Join(tempDir, "inventory.yml")
	inventory := strings.Join([]string{
		"all:",
		"  vars:",
		"    ansible_python_interpreter: auto_silent",
		"  children:",
		"    civa_targets:",
		"      hosts:",
		"        app-01:",
		"          ansible_host: \"203.0.113.10\"",
		"          ansible_user: \"deployer\"",
		"          ansible_port: 2222",
		"          ansible_ssh_private_key_file: \"/tmp/id_ed25519\"",
		"        db-01:",
		"          ansible_host: \"203.0.113.20\"",
		"          ansible_user: \"deployer\"",
		"          ansible_port: 22",
	}, "\n") + "\n"
	if err := os.WriteFile(inventoryPath, []byte(inventory), 0o600); err != nil {
		t.Fatalf("failed to write inventory: %v", err)
	}

	entries, err := parseSSHConfigHostsFromInventory(inventoryPath)
	if err != nil {
		t.Fatalf("parseSSHConfigHostsFromInventory returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Alias != "app-01" || entries[0].HostName != "203.0.113.10" || entries[0].User != "deployer" || entries[0].Port != 2222 || entries[0].IdentityFile != "/tmp/id_ed25519" {
		t.Fatalf("unexpected first entry: %#v", entries[0])
	}
	if entries[1].Alias != "db-01" || entries[1].HostName != "203.0.113.20" || entries[1].Port != 22 {
		t.Fatalf("unexpected second entry: %#v", entries[1])
	}
}

func TestParseSSHConfigHostsFromInventoryPrefersCustomSSHPort(t *testing.T) {
	inventoryPath := filepath.Join(t.TempDir(), "inventory.yml")
	inventory := strings.Join([]string{
		"all:",
		"  children:",
		"    civa_targets:",
		"      hosts:",
		"        app-01:",
		"          ansible_host: \"203.0.113.10\"",
		"          ansible_user: \"deployer\"",
		"          ansible_port: 22",
		"          civa_custom_ssh_port: 2201",
	}, "\n") + "\n"
	if err := os.WriteFile(inventoryPath, []byte(inventory), 0o600); err != nil {
		t.Fatalf("failed to write inventory: %v", err)
	}

	entries, err := parseSSHConfigHostsFromInventory(inventoryPath)
	if err != nil {
		t.Fatalf("parseSSHConfigHostsFromInventory returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Port != 2201 {
		t.Fatalf("expected ssh config to use custom port 2201, got %d", entries[0].Port)
	}
}

func TestSyncSSHConfigAfterApplyInHomeUpdatesManagedEntries(t *testing.T) {
	homeDir := t.TempDir()
	inventoryPath := filepath.Join(t.TempDir(), "inventory.yml")
	inventory := strings.Join([]string{
		"all:",
		"  children:",
		"    civa_targets:",
		"      hosts:",
		"        web-01:",
		"          ansible_host: \"203.0.113.50\"",
		"          ansible_user: \"root\"",
		"          ansible_port: 22",
	}, "\n") + "\n"
	if err := os.WriteFile(inventoryPath, []byte(inventory), 0o600); err != nil {
		t.Fatalf("failed to write inventory: %v", err)
	}

	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatalf("failed to create ssh dir: %v", err)
	}
	sshConfigPath := filepath.Join(sshDir, "config")
	initial := strings.Join([]string{
		"Host github.com",
		"  User git",
		"",
		"# civa-managed-start web-01",
		"Host web-01",
		"  HostName 198.51.100.10",
		"  User root",
		"  Port 22",
		"# civa-managed-end web-01",
	}, "\n") + "\n"
	if err := os.WriteFile(sshConfigPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("failed to write initial ssh config: %v", err)
	}

	state := &runtimeState{InventoryFile: inventoryPath}
	if err := syncSSHConfigAfterApplyInHome(&config{}, state, homeDir); err != nil {
		t.Fatalf("syncSSHConfigAfterApplyInHome returned error: %v", err)
	}

	content, err := os.ReadFile(sshConfigPath)
	if err != nil {
		t.Fatalf("failed to read ssh config: %v", err)
	}
	updated := string(content)
	if strings.Count(updated, "# civa-managed-start web-01") != 1 {
		t.Fatalf("expected exactly one managed web-01 block, got:\n%s", updated)
	}
	if !strings.Contains(updated, "HostName 203.0.113.50") {
		t.Fatalf("expected updated host address in ssh config, got:\n%s", updated)
	}
	if _, err := os.Stat(filepath.Join(homeDir, strings.TrimPrefix(userCivaHomeDirectory, "~/"))); err != nil {
		t.Fatalf("expected ~/.civa to be created, got error: %v", err)
	}
}
