package infra

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSetupConfigRequiresUser(t *testing.T) {
	tmp := t.TempDir()
	keyPath := filepath.Join(tmp, "id_rsa.pub")
	if err := os.WriteFile(keyPath, []byte("pubkey"), 0o600); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}
	cfg := config{
		SSHPort:      2224,
		SSHPublicKey: keyPath,
		Servers: []serverSpec{{
			Address: "203.0.113.10",
		}},
	}
	if err := validateSetupConfig(&cfg); err == nil || !strings.Contains(err.Error(), "requires an SSH user") {
		t.Fatalf("expected SSH user validation error, got %v", err)
	}
}

func TestValidateSetupConfigRequiresPort(t *testing.T) {
	tmp := t.TempDir()
	keyPath := filepath.Join(tmp, "id_rsa.pub")
	if err := os.WriteFile(keyPath, []byte("pubkey"), 0o600); err != nil {
		t.Fatalf("failed to write public key: %v", err)
	}
	cfg := config{
		SSHUser:      "root",
		SSHPublicKey: keyPath,
		Servers: []serverSpec{{
			Address: "203.0.113.11",
		}},
	}
	if err := validateSetupConfig(&cfg); err == nil || !strings.Contains(err.Error(), "requires an SSH port") {
		t.Fatalf("expected SSH port validation error, got %v", err)
	}
}
