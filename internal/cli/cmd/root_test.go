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

func TestRootRunRoutesPreviewCommand(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	if err := root.Run([]string{"preview", "my-plan"}); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	if executor.requests[0].Command != domain.CommandPreview || executor.requests[0].PlanName != "my-plan" {
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

func TestRootRunRoutesPlanStartFlags(t *testing.T) {
	executor := &stubExecutor{}
	root := NewRoot(executor)

	err := root.Run([]string{"plan", "start", "--server", "203.0.113.10,web-01,2201", "--ssh-user", "ubuntu", "--non-interactive"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if len(executor.requests) != 1 {
		t.Fatalf("expected one request, got %d", len(executor.requests))
	}
	req := executor.requests[0]
	if req.Command != domain.CommandPlan || req.PlanAction != domain.PlanActionStart {
		t.Fatalf("unexpected plan start request: %#v", req)
	}
	if req.SSHUser != "ubuntu" || len(req.Servers) != 1 || req.Servers[0] != "203.0.113.10,web-01,2201" {
		t.Fatalf("unexpected mapped flags: %#v", req)
	}
	if !req.NonInteractive || !req.Provided.NonInteractive || !req.Provided.SSHUser || !req.Provided.Servers {
		t.Fatalf("expected provided/global flags to be tracked: %#v", req.Provided)
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
