package app

import "civa/internal/cli/domain"

type Runner interface {
	ExecuteRequest(req domain.Request) error
}

type Service struct {
	runner Runner
}

func NewService(runner Runner) *Service {
	return &Service{runner: runner}
}

func (s *Service) Execute(req domain.Request) error {
	return s.runner.ExecuteRequest(req)
}
