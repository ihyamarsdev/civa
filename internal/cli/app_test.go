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

func TestParseServerSpecSupportsHostname(t *testing.T) {
	server, err := parseServerSpec("203.0.113.10,web-01")
	if err != nil {
		t.Fatalf("parseServerSpec returned error: %v", err)
	}

	if server.Address != "203.0.113.10" || server.Hostname != "web-01" {
		t.Fatalf("unexpected server spec: %#v", server)
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
		PlaybookFile:  "/tmp/civa test/ansible/playbook.yml",
		VarsFile:      "/tmp/civa test/vars.yml",
	}

	command := buildAnsibleCommand(cfg, state)
	if !strings.Contains(command, `"/tmp/civa test/inventory.yml"`) {
		t.Fatalf("expected quoted inventory path, got %s", command)
	}
	if !strings.Contains(command, `"/tmp/civa test/ansible/playbook.yml"`) {
		t.Fatalf("expected quoted playbook path, got %s", command)
	}
	if !strings.Contains(command, `@"/tmp/civa test/vars.yml"`) && !strings.Contains(command, `"@/tmp/civa test/vars.yml"`) {
		t.Fatalf("expected quoted vars path, got %s", command)
	}
	if !strings.Contains(command, "--tags system_update,user_management") {
		t.Fatalf("expected tags in command, got %s", command)
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
		filepath.Join(ansibleDir, "playbook.yml"),
		filepath.Join(ansibleDir, "templates", "traefik.env.j2"),
		filepath.Join(ansibleDir, "templates", "traefik-compose.yml.j2"),
		filepath.Join(ansibleDir, "templates", "fail2ban-jail.local.j2"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected embedded asset at %s: %v", path, err)
		}
	}
}
