package infra

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretStoreWriteReadListRemove(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := writeSecretValue("API.Token", "super-secret"); err != nil {
		t.Fatalf("writeSecretValue returned error: %v", err)
	}

	value, err := readSecretValue("api.token")
	if err != nil {
		t.Fatalf("readSecretValue returned error: %v", err)
	}
	if value != "super-secret" {
		t.Fatalf("unexpected secret value: %q", value)
	}

	names, err := listSecretNames()
	if err != nil {
		t.Fatalf("listSecretNames returned error: %v", err)
	}
	if len(names) != 1 || names[0] != "api.token" {
		t.Fatalf("unexpected secret names: %v", names)
	}

	removed, err := removeSecretValue("api.token")
	if err != nil {
		t.Fatalf("removeSecretValue returned error: %v", err)
	}
	if !removed {
		t.Fatal("expected removeSecretValue to report removed=true")
	}

	if _, err := readSecretValue("api.token"); err == nil {
		t.Fatal("expected readSecretValue to fail after removal")
	}
}

func TestResolveSetupSecretPasswordRejectsMixedSource(t *testing.T) {
	cfg := &config{SSHPassword: "plain", SSHPasswordSecret: "secret-name"}

	if err := resolveSetupSecretPassword(cfg); err == nil {
		t.Fatal("expected resolveSetupSecretPassword to reject mixed password sources")
	}
}

func TestSecretStoreHardensPermissionsAndKeepsCiphertextOnly(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	const secretValue = "super-secret"
	if err := writeSecretValue("API.Token", secretValue); err != nil {
		t.Fatalf("writeSecretValue returned error: %v", err)
	}

	dirInfo, err := os.Stat(secretsDirectoryPath())
	if err != nil {
		t.Fatalf("failed to stat secrets directory: %v", err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("expected secrets directory mode 0700, got %04o", dirInfo.Mode().Perm())
	}

	for _, path := range []string{secretsKeyFilePath(), secretsStoreFilePath()} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("failed to stat %s: %v", path, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("expected %s mode 0600, got %04o", path, info.Mode().Perm())
		}
	}

	storeContent, err := os.ReadFile(secretsStoreFilePath())
	if err != nil {
		t.Fatalf("failed to read secret store file: %v", err)
	}
	if strings.Contains(string(storeContent), secretValue) {
		t.Fatalf("expected secret store to contain ciphertext only, got plaintext leak: %s", string(storeContent))
	}
}

func TestEnsureSecretKeyRejectsInvalidLength(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := os.MkdirAll(secretsDirectoryPath(), 0o700); err != nil {
		t.Fatalf("failed to create secrets dir: %v", err)
	}
	if err := os.WriteFile(secretsKeyFilePath(), []byte("short-key"), 0o600); err != nil {
		t.Fatalf("failed to write invalid key: %v", err)
	}

	if _, err := ensureSecretKey(); err == nil || !strings.Contains(err.Error(), "invalid secret key length") {
		t.Fatalf("expected invalid secret key length error, got %v", err)
	}
}

func TestReadSecretValueRejectsMalformedCiphertext(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := writeSecretValue("api.token", "super-secret"); err != nil {
		t.Fatalf("writeSecretValue returned error: %v", err)
	}

	store, err := loadSecretStore()
	if err != nil {
		t.Fatalf("loadSecretStore returned error: %v", err)
	}
	store.Secrets["api.token"] = "not-base64"
	if err := saveSecretStore(store); err != nil {
		t.Fatalf("saveSecretStore returned error: %v", err)
	}

	if _, err := readSecretValue("api.token"); err == nil || !strings.Contains(err.Error(), "decode secret payload") {
		t.Fatalf("expected malformed ciphertext decode error, got %v", err)
	}
}

func TestOutputHasAnsibleChanges(t *testing.T) {
	if !outputHasAnsibleChanges("PLAY RECAP\nweb-01 : ok=12 changed=2 unreachable=0 failed=0\n") {
		t.Fatal("expected changed recap to be detected as drift")
	}
	if outputHasAnsibleChanges("PLAY RECAP\nweb-01 : ok=12 changed=0 unreachable=0 failed=0\n") {
		t.Fatal("expected zero changed recap to be treated as no drift")
	}
}

func TestCompareDriftSnapshotDetectsArtifactChanges(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	planDir := filepath.Join(homeDir, ".civa", "runs", "web-01")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatalf("failed to create plan dir: %v", err)
	}

	planPath := filepath.Join(planDir, "plan.md")
	inventoryPath := filepath.Join(planDir, "inventory.yml")
	varsPath := filepath.Join(planDir, "vars.yml")

	for path, content := range map[string]string{
		planPath:      "# plan\n",
		inventoryPath: "all:\n  hosts: {}\n",
		varsPath:      "k: v\n",
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", path, err)
		}
	}

	state := &runtimeState{InventoryFile: inventoryPath, VarsFile: varsPath}
	if _, hasBaseline, err := loadDriftSnapshot(planPath); err != nil {
		t.Fatalf("loadDriftSnapshot returned error: %v", err)
	} else if hasBaseline {
		t.Fatal("expected no baseline before first drift snapshot")
	}

	snapshot, drifted, err := compareDriftSnapshot(planPath, state)
	if err != nil {
		t.Fatalf("compareDriftSnapshot returned error: %v", err)
	}
	if drifted {
		t.Fatal("expected first compare to initialize baseline without drift")
	}
	if err := saveDriftSnapshot(planPath, snapshot); err != nil {
		t.Fatalf("saveDriftSnapshot returned error: %v", err)
	}
	if _, hasBaseline, err := loadDriftSnapshot(planPath); err != nil {
		t.Fatalf("loadDriftSnapshot returned error: %v", err)
	} else if !hasBaseline {
		t.Fatal("expected baseline after saving first drift snapshot")
	}

	if err := os.WriteFile(varsPath, []byte("k: changed\n"), 0o644); err != nil {
		t.Fatalf("failed to update vars file: %v", err)
	}

	_, drifted, err = compareDriftSnapshot(planPath, state)
	if err != nil {
		t.Fatalf("compareDriftSnapshot after change returned error: %v", err)
	}
	if !drifted {
		t.Fatal("expected drift after vars file change")
	}
}

func TestResolveRollbackPlanPathUsesLastSuccessfulPlan(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	planPath := filepath.Join(homeDir, ".civa", "runs", "web-01", "plan.md")
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatalf("failed to create plan directory: %v", err)
	}
	if err := os.WriteFile(planPath, []byte("# plan\n"), 0o644); err != nil {
		t.Fatalf("failed to write plan: %v", err)
	}

	if err := writeRollbackSuccess(planPath); err != nil {
		t.Fatalf("writeRollbackSuccess returned error: %v", err)
	}

	cfg := defaultConfig(commandApply)
	resolved, err := resolveRollbackPlanPath(&cfg)
	if err != nil {
		t.Fatalf("resolveRollbackPlanPath returned error: %v", err)
	}
	if resolved != planPath {
		t.Fatalf("unexpected rollback path: got %s want %s", resolved, planPath)
	}
}

func TestRunApplyRollbackFlowPreflightFailureRecordsRollbackState(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	planDir := filepath.Join(homeDir, ".civa", "runs", "web-01")
	playbookFile := filepath.Join(planDir, "ansible", "main.yml")
	inventoryFile := filepath.Join(planDir, "inventory.yml")
	varsFile := filepath.Join(planDir, "vars.yml")
	planPath := filepath.Join(planDir, "plan.md")

	if err := os.MkdirAll(filepath.Dir(playbookFile), 0o755); err != nil {
		t.Fatalf("failed to create plan directory: %v", err)
	}
	if err := os.WriteFile(planPath, []byte("# plan\n"), 0o644); err != nil {
		t.Fatalf("failed to write plan file: %v", err)
	}
	if err := os.WriteFile(inventoryFile, []byte("all:\n  hosts:\n    web-01:\n      ansible_host: 203.0.113.10\n"), 0o644); err != nil {
		t.Fatalf("failed to write inventory file: %v", err)
	}
	if err := os.WriteFile(varsFile, []byte("k: v\n"), 0o644); err != nil {
		t.Fatalf("failed to write vars file: %v", err)
	}
	if err := os.WriteFile(playbookFile, []byte("this: [is: invalid"), 0o644); err != nil {
		t.Fatalf("failed to write invalid playbook file: %v", err)
	}

	metadata := plannedRunMetadata{
		WebServer:     webServerNone,
		SSHAuthMethod: sshAuthMethodKey,
		SSHUser:       "root",
		SSHPort:       22,
		Components:    []string{"system_update"},
		InventoryFile: inventoryFile,
		VarsFile:      varsFile,
		PlanFile:      planPath,
		PlaybookFile:  playbookFile,
	}
	content, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}
	if err := os.WriteFile(planMetadataPath(planPath), append(content, '\n'), 0o600); err != nil {
		t.Fatalf("failed to write metadata file: %v", err)
	}

	cfg := defaultConfig(commandApply)
	cfg.ApplyAction = applyActionRollback
	cfg.AssumeYes = true
	cfg.NonInteractive = true
	cfg.PlanInputFile = planPath
	cfg.Provided.PlanInputFile = true

	if err := runApplyRollbackFlow(&cfg); err == nil {
		t.Fatal("expected rollback preflight to fail")
	}

	state, err := loadRollbackState()
	if err != nil {
		t.Fatalf("loadRollbackState returned error: %v", err)
	}
	if state.LastFailedPlan != planPath {
		t.Fatalf("expected failed rollback plan %s, got %s", planPath, state.LastFailedPlan)
	}
	if strings.TrimSpace(state.LastFailureReason) == "" {
		t.Fatal("expected rollback failure reason to be recorded")
	}
}
