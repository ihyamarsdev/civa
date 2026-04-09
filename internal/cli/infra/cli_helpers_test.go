package infra

import "testing"

func TestEffectiveSSHUserPrefersServerOverride(t *testing.T) {
	cfg := config{SSHUser: "root"}
	server := serverSpec{Address: "203.0.113.10", SSHUser: "alice"}
	if got := effectiveSSHUser(cfg, server); got != "alice" {
		t.Fatalf("expected server override, got %q", got)
	}
}

func TestEffectiveSSHUserFallsBackToConfig(t *testing.T) {
	cfg := config{SSHUser: "root"}
	server := serverSpec{Address: "203.0.113.10"}
	if got := effectiveSSHUser(cfg, server); got != "root" {
		t.Fatalf("expected config user fallback, got %q", got)
	}
}

func TestEffectiveSSHUserReturnsEmptyWhenUnset(t *testing.T) {
	cfg := config{}
	server := serverSpec{Address: "203.0.113.10"}
	if got := effectiveSSHUser(cfg, server); got != "" {
		t.Fatalf("expected empty user when unset, got %q", got)
	}
}

func TestEffectiveSSHPortPrefersServerOverride(t *testing.T) {
	cfg := config{SSHPort: 2222}
	server := serverSpec{Address: "203.0.113.10", SSHPort: 2201}
	if got := effectiveSSHPort(cfg, server); got != 2201 {
		t.Fatalf("expected server port override, got %d", got)
	}
}

func TestEffectiveSSHPortFallsBackToConfig(t *testing.T) {
	cfg := config{SSHPort: 2222}
	server := serverSpec{Address: "203.0.113.10"}
	if got := effectiveSSHPort(cfg, server); got != 2222 {
		t.Fatalf("expected config port fallback, got %d", got)
	}
}

func TestEffectiveSSHPortReturnsZeroWhenUnset(t *testing.T) {
	cfg := config{}
	server := serverSpec{Address: "203.0.113.10"}
	if got := effectiveSSHPort(cfg, server); got != 0 {
		t.Fatalf("expected zero port when unset, got %d", got)
	}
}

func TestFormatServerTargetReflectsPerHostOverrides(t *testing.T) {
	cfg := config{SSHUser: "root", SSHPort: 22}
	server := serverSpec{Address: "203.0.113.10", SSHUser: "alice", SSHPort: 2222}
	if got := formatServerTarget(cfg, server); got != "alice@203.0.113.10:2222" {
		t.Fatalf("unexpected target with overrides: %s", got)
	}
}

func TestFormatServerTargetUsesDefaultsWhenNoOverrides(t *testing.T) {
	cfg := config{SSHUser: "root", SSHPort: 22}
	server := serverSpec{Address: "203.0.113.10"}
	if got := formatServerTarget(cfg, server); got != "root@203.0.113.10:22" {
		t.Fatalf("unexpected default target: %s", got)
	}
}

func TestSetupNeedsGlobalSSHUser(t *testing.T) {
	if !setupNeedsGlobalSSHUser([]serverSpec{{Address: "203.0.113.10"}, {Address: "203.0.113.11", SSHUser: "alice"}}) {
		t.Fatal("expected global SSH user requirement when at least one target omits user override")
	}

	if setupNeedsGlobalSSHUser([]serverSpec{{Address: "203.0.113.10", SSHUser: "root"}, {Address: "203.0.113.11", SSHUser: "alice"}}) {
		t.Fatal("did not expect global SSH user requirement when all targets define user override")
	}
}

func TestSetupNeedsGlobalSSHPort(t *testing.T) {
	if !setupNeedsGlobalSSHPort([]serverSpec{{Address: "203.0.113.10", SSHPort: 0}, {Address: "203.0.113.11", SSHPort: 2201}}) {
		t.Fatal("expected global SSH port requirement when at least one target omits custom port")
	}

	if setupNeedsGlobalSSHPort([]serverSpec{{Address: "203.0.113.10", SSHPort: 22}, {Address: "203.0.113.11", SSHPort: 2201}}) {
		t.Fatal("did not expect global SSH port requirement when all targets define custom port")
	}
}

func TestDefaultWizardOptionalSSHPort(t *testing.T) {
	if got := defaultWizardOptionalSSHPort(0); got != "" {
		t.Fatalf("expected empty default port for unset value, got %q", got)
	}
	if got := defaultWizardOptionalSSHPort(2222); got != "2222" {
		t.Fatalf("expected valid port string, got %q", got)
	}
}
