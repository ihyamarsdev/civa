package app

import (
	"errors"
	"testing"

	"civa/internal/cli/domain"
)

type stubRunner struct {
	req domain.Request
	err error
}

func (s *stubRunner) ExecuteRequest(req domain.Request) error {
	s.req = req
	return s.err
}

func TestServiceExecuteDelegatesToRunner(t *testing.T) {
	runner := &stubRunner{}
	service := NewService(runner)

	request := domain.Request{Command: domain.CommandPlan, PlanAction: domain.PlanActionStart}
	if err := service.Execute(request); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if runner.req.Command != domain.CommandPlan || runner.req.PlanAction != domain.PlanActionStart {
		t.Fatalf("unexpected request forwarded to runner: %#v", runner.req)
	}
}

func TestServiceExecuteReturnsRunnerError(t *testing.T) {
	expectedErr := errors.New("runner failed")
	runner := &stubRunner{err: expectedErr}
	service := NewService(runner)

	err := service.Execute(domain.Request{Command: domain.CommandApply, ApplyAction: domain.ApplyActionExecute})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}
