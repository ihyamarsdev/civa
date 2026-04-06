package cmd

import (
	"errors"
	"strings"
	"testing"

	"civa/internal/cli/domain"
)

type stubExecutor struct {
	requests []domain.Request
	err      error
}

func (s *stubExecutor) Execute(req domain.Request) error {
	s.requests = append(s.requests, req)
	return s.err
}

func TestRootRunNoArgsRoutesHelpWhenNonInteractive(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"--non-interactive"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandHelp {
		t.Fatalf("expected help request, got %#v", req)
	}
}

func TestRootRunNoArgsRoutesHelp(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandHelp {
		t.Fatalf("expected help request, got %#v", req)
	}
}

func TestRootRunStartWizardRoutesSetup(t *testing.T) {
	origTerminalFn := isTerminalFn
	origPromptFn := promptWizardActionFn
	t.Cleanup(func() {
		isTerminalFn = origTerminalFn
		promptWizardActionFn = origPromptFn
	})

	isTerminalFn = func(_ int) bool { return true }
	promptWizardActionFn = func() (string, error) { return wizardActionSetup, nil }

	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"start"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandSetup {
		t.Fatalf("expected setup request, got %#v", req)
	}
}

func TestRootRunStartWizardRoutesPlanInit(t *testing.T) {
	origTerminalFn := isTerminalFn
	origPromptFn := promptWizardActionFn
	t.Cleanup(func() {
		isTerminalFn = origTerminalFn
		promptWizardActionFn = origPromptFn
	})

	isTerminalFn = func(_ int) bool { return true }
	promptWizardActionFn = func() (string, error) { return wizardActionPlanInit, nil }

	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"start"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandPlan || req.PlanAction != domain.PlanActionInit {
		t.Fatalf("expected plan init request, got %#v", req)
	}
}

func TestRootRunStartWizardExitSkipsExecution(t *testing.T) {
	origTerminalFn := isTerminalFn
	origPromptFn := promptWizardActionFn
	t.Cleanup(func() {
		isTerminalFn = origTerminalFn
		promptWizardActionFn = origPromptFn
	})

	isTerminalFn = func(_ int) bool { return true }
	promptWizardActionFn = func() (string, error) { return wizardActionExit, nil }

	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"start"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 0 {
		t.Fatalf("expected no request when exiting wizard, got %d", len(executor.requests))
	}
}

func TestRootRunStartNonInteractiveRoutesStartHelp(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"start", "--non-interactive"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandHelp || req.HelpTarget != "start" {
		t.Fatalf("expected start help request, got %#v", req)
	}
	if !req.NonInteractive || !req.Provided.NonInteractive {
		t.Fatalf("expected non-interactive global flags to be mapped, got %#v", req)
	}
}

func TestRootRunRoutesPlanReviewCommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	if err := root.Run([]string{"plan", "review", "my-plan"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	if executor.requests[0].Command != domain.CommandPlan || executor.requests[0].PlanAction != domain.PlanActionReview || executor.requests[0].PlanName != "my-plan" {
		t.Fatalf("unexpected request: %#v", executor.requests[0])
	}
}

func TestRootRunRoutesPlanEditCommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	if err := root.Run([]string{"plan", "edit", "my-plan"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	if executor.requests[0].Command != domain.CommandPlan || executor.requests[0].PlanAction != domain.PlanActionEdit || executor.requests[0].PlanName != "my-plan" {
		t.Fatalf("unexpected request: %#v", executor.requests[0])
	}
}

func TestRootRunRoutesPlanWithoutFlagsToHelp(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	if err := root.Run([]string{"plan"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandHelp || req.HelpTarget != string(domain.CommandPlan) {
		t.Fatalf("expected plan help request, got %#v", req)
	}
}

func TestRootRunRoutesPlanInitFlags(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"plan", "init", "--server", "203.0.113.10,web-01,2201", "--ssh-user", "ubuntu", "--non-interactive"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandPlan || req.PlanAction != domain.PlanActionInit {
		t.Fatalf("unexpected plan init request: %#v", req)
	}
	if req.SSHUser != "ubuntu" || len(req.Servers) != 1 || req.Servers[0] != "203.0.113.10,web-01,2201" {
		t.Fatalf("unexpected mapped flags: %#v", req)
	}
	if !req.NonInteractive || !req.Provided.NonInteractive || !req.Provided.SSHUser || !req.Provided.Servers {
		t.Fatalf("expected provided/global flags to be tracked: %#v", req.Provided)
	}
}

func TestRootRunRoutesPlanListForSpecificName(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"plan", "list", "hyane"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandPlan || req.PlanAction != domain.PlanActionList || req.PlanName != "hyane" {
		t.Fatalf("unexpected plan list request: %#v", req)
	}
}

func TestRootRunRoutesConfigCommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"config"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandHelp || req.HelpTarget != string(domain.CommandConfig) {
		t.Fatalf("unexpected config help request: %#v", req)
	}
}

func TestRootRunRejectsDirectConfigWithPlanName(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"config", "web-01-v2"})
	if err == nil {
		t.Fatal("expected error for direct config with plan-name")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestRootRunRoutesConfigProviderListSubcommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"config", "nginx", "list"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandConfig || req.ConfigAction != domain.ConfigActionList || req.WebServer != "nginx" {
		t.Fatalf("unexpected config list request: %#v", req)
	}
	if !req.Provided.WebServer {
		t.Fatalf("expected provided webserver flag for provider list: %#v", req.Provided)
	}
}

func TestRootRunRoutesConfigProviderInitSubcommandWithPlanName(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"config", "nginx", "init", "web-01-v2"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandConfig || req.ConfigAction != domain.ConfigActionInit || req.PlanName != "web-01-v2" || req.WebServer != "nginx" {
		t.Fatalf("unexpected config init request: %#v", req)
	}
	if !req.Provided.WebServer {
		t.Fatalf("expected provided webserver flag for provider init: %#v", req.Provided)
	}
}

func TestRootRunRoutesConfigProviderRemoveSubcommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"config", "nginx", "remove", "web-01-v2"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandConfig || req.ConfigAction != domain.ConfigActionRemove || req.WebServer != "nginx" || req.PlanName != "web-01-v2" {
		t.Fatalf("unexpected config remove request: %#v", req)
	}
	if !req.Provided.WebServer {
		t.Fatalf("expected remove profile to mark provided webserver flag: %#v", req.Provided)
	}
}

func TestRootRunRoutesToolsCommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"tools"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	if req := executor.requests[0]; req.Command != domain.CommandTools {
		t.Fatalf("unexpected tools request: %#v", req)
	}
}

func TestRootRunRoutesToolsCloudflareZones(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"tools", "cloudflare", "zones", "list", "--profile", "default"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandTools || req.ToolsProvider != domain.ToolsProviderCloudflare || req.ToolsAction != domain.ToolsActionCloudflareZones || req.ToolsOperation != domain.ToolsOperationList {
		t.Fatalf("unexpected tools cloudflare zones request: %#v", req)
	}
	if req.AuthProfile != "default" {
		t.Fatalf("expected auth profile to be mapped, got %#v", req)
	}
	if !req.Provided.AuthProfile {
		t.Fatalf("expected auth profile provided flag, got %#v", req.Provided)
	}
}

func TestRootRunRejectsConfigAllRemoveSubcommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"config", "all", "remove", "web-01-v2"})
	if err == nil {
		t.Fatal("expected error for config all remove")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestRootRunReturnsExecutorError(t *testing.T) {
	expectedErr := errors.New("executor failed")
	executor := &stubExecutor{err: expectedErr}
	root := NewRoot(executor)

	err := root.Run([]string{"doctor", "fix"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestRootRunUnknownCommandReturnsError(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"unknown-cmd"})
	if err == nil {
		t.Fatal("expected unknown command error")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestRootRunRoutesApplyDriftSubcommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"apply", "drift", "web-01"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandApply || req.ApplyAction != domain.ApplyActionDrift || req.PlanName != "web-01" {
		t.Fatalf("unexpected apply drift request: %#v", req)
	}
}

func TestRootRunRoutesApplyRollbackSubcommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"apply", "rollback", "--yes"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandApply || req.ApplyAction != domain.ApplyActionRollback || !req.AssumeYes {
		t.Fatalf("unexpected apply rollback request: %#v", req)
	}
}

func TestRootRunRoutesSecretSet(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"secret", "set", "vps-password", "--value", "hunter2"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandSecret || req.SecretAction != domain.SecretActionSet || req.SecretName != "vps-password" || req.SecretValue != "hunter2" {
		t.Fatalf("unexpected secret set request: %#v", req)
	}
	if !req.Provided.SecretValue {
		t.Fatalf("expected secret value to be marked as provided: %#v", req.Provided)
	}
	if req.Provided.SecretValueFile {
		t.Fatalf("expected secret value file flag to be false: %#v", req.Provided)
	}
}

func TestRootRunRoutesSecretSetValueFile(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"secret", "set", "vps-password", "--value-file", "/tmp/secret.txt"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandSecret || req.SecretAction != domain.SecretActionSet || req.SecretName != "vps-password" || req.SecretValueFile != "/tmp/secret.txt" {
		t.Fatalf("unexpected secret set value-file request: %#v", req)
	}
	if !req.Provided.SecretValueFile {
		t.Fatalf("expected secret value file to be marked as provided: %#v", req.Provided)
	}
	if req.Provided.SecretValue {
		t.Fatalf("expected secret value flag to be false when using --value-file: %#v", req.Provided)
	}
}

func TestRootRunRoutesSecretListAndRemove(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"secret", "list"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	err = root.Run([]string{"secret", "remove", "vps-password"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 2 {
		t.Fatalf("expected two requests, got %d", len(executor.requests))
	}
	if executor.requests[0].Command != domain.CommandSecret || executor.requests[0].SecretAction != domain.SecretActionList {
		t.Fatalf("unexpected secret list request: %#v", executor.requests[0])
	}
	if executor.requests[1].Command != domain.CommandSecret || executor.requests[1].SecretAction != domain.SecretActionRemove || executor.requests[1].SecretName != "vps-password" {
		t.Fatalf("unexpected secret remove request: %#v", executor.requests[1])
	}
}

func TestRootRunRoutesSetupPasswordSecretFlag(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"setup", "--server", "203.0.113.10", "--ssh-user", "root", "--ssh-password-secret", "vps-password", "--ssh-public-key", "~/.ssh/id_ed25519.pub"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandSetup || req.SSHPasswordSecret != "vps-password" {
		t.Fatalf("unexpected setup request: %#v", req)
	}
	if !req.Provided.SSHPasswordSecret {
		t.Fatalf("expected SSH password secret flag to be marked provided: %#v", req.Provided)
	}
}

func TestRootRunHelpRoutesSecretSubcommandsToSecretHelp(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	if err := root.Run([]string{"secret", "list", "--help"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := root.Run([]string{"secret", "remove", "--help"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 2 {
		t.Fatalf("expected two help requests, got %d", len(executor.requests))
	}
	for idx, req := range executor.requests {
		if req.Command != domain.CommandHelp || req.HelpTarget != string(domain.CommandSecret) {
			t.Fatalf("unexpected help request at %d: %#v", idx, req)
		}
	}
}

func TestRootRunHelpRoutesConfigProviderSubcommandsToProviderHelp(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	if err := root.Run([]string{"config", "nginx", "list", "--help"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := root.Run([]string{"config", "caddy", "init", "--help"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := root.Run([]string{"config", "all", "list", "--help"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 3 {
		t.Fatalf("expected three help requests, got %d", len(executor.requests))
	}
	if req := executor.requests[0]; req.Command != domain.CommandHelp || req.HelpTarget != helpTargetConfigNginx {
		t.Fatalf("unexpected nginx help request: %#v", req)
	}
	if req := executor.requests[1]; req.Command != domain.CommandHelp || req.HelpTarget != helpTargetConfigCaddy {
		t.Fatalf("unexpected caddy help request: %#v", req)
	}
	if req := executor.requests[2]; req.Command != domain.CommandHelp || req.HelpTarget != helpTargetConfigAll {
		t.Fatalf("unexpected all help request: %#v", req)
	}
}

func TestRootRunHelpRoutesToolsCloudflareSubcommandsToProviderHelp(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	if err := root.Run([]string{"tools", "cloudflare", "--help"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := root.Run([]string{"tools", "cloudflare", "zones", "--help"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 2 {
		t.Fatalf("expected two help requests, got %d", len(executor.requests))
	}
	for idx, req := range executor.requests {
		if req.Command != domain.CommandHelp || req.HelpTarget != helpTargetToolsCF {
			t.Fatalf("unexpected tools help request at %d: %#v", idx, req)
		}
	}
}
