package infra

import (
	"civa/internal/cli/domain"
	"fmt"
	"strings"
)

type LegacyRunner struct{}

func NewLegacyRunner() LegacyRunner {
	return LegacyRunner{}
}

func (LegacyRunner) ExecuteRequest(req domain.Request) error {
	switch req.Command {
	case domain.CommandHelp:
		target := strings.TrimSpace(req.HelpTarget)
		if target == "" {
			printUsage()
			return nil
		}
		printCommandUsage(target)
		return nil
	case domain.CommandVersion:
		fmt.Printf("civa %s\n", version)
		return nil
	case domain.CommandCompletion:
		return runCompletionCommand(req.CompletionArgs)
	case domain.CommandCompleteInternal:
		return runHiddenCompletion(req.CompletionArgs)
	case domain.CommandDoctor:
		cfg := defaultConfig(commandDoctor)
		applyGlobalRequest(req, &cfg)
		if req.DoctorAction != "" {
			cfg.DoctorAction = req.DoctorAction
		}
		return runDoctor(cfg)
	case domain.CommandSetup:
		cfg := defaultConfig(commandSetup)
		applyGlobalRequest(req, &cfg)
		applySharedRequest(req, &cfg)
		servers, err := parseServers(req.Servers)
		if err != nil {
			return err
		}
		cfg.Servers = servers
		return runSetupFlow(&cfg)
	case domain.CommandUninstall:
		cfg := defaultConfig(commandUninstall)
		applyGlobalRequest(req, &cfg)
		return runUninstall(cfg)
	case domain.CommandPlan:
		cfg := defaultConfig(commandPlan)
		applyGlobalRequest(req, &cfg)
		cfg.PlanName = req.PlanName
		if req.PlanAction != "" {
			cfg.PlanAction = req.PlanAction
		}
		applySharedRequest(req, &cfg)
		servers, err := parseServers(req.Servers)
		if err != nil {
			return err
		}
		cfg.Servers = servers

		switch cfg.PlanAction {
		case planActionList:
			return runPlanListFlow(&cfg)
		case planActionRemove:
			cfg.PlanName = req.PlanName
			return runPlanRemoveFlow(&cfg)
		default:
			cfg.PlanAction = planActionStart
			return runPlanFlow(&cfg)
		}
	case domain.CommandPreview:
		cfg := defaultConfig(commandPreview)
		applyGlobalRequest(req, &cfg)
		cfg.PlanName = req.PlanName
		cfg.PlanInputFile = req.PlanInputFile
		cfg.Provided.PlanInputFile = req.Provided.PlanInputFile
		return runPreviewFlow(&cfg)
	case domain.CommandApply:
		cfg := defaultConfig(commandApply)
		applyGlobalRequest(req, &cfg)
		if req.ApplyAction != "" {
			cfg.ApplyAction = req.ApplyAction
		}
		cfg.PlanName = req.PlanName
		cfg.PlanInputFile = req.PlanInputFile
		cfg.Provided.PlanInputFile = req.Provided.PlanInputFile

		if cfg.ApplyAction == applyActionReview {
			return runApplyReviewFlow(&cfg)
		}
		cfg.ApplyAction = applyActionExecute
		return runApplyFlow(&cfg)
	default:
		return fmt.Errorf("unknown command: %s", req.Command)
	}
}

func applyGlobalRequest(req domain.Request, cfg *config) {
	cfg.AssumeYes = req.AssumeYes
	cfg.NonInteractive = req.NonInteractive
	cfg.Provided.NonInteractive = req.Provided.NonInteractive
}

func applySharedRequest(req domain.Request, cfg *config) {
	cfg.PlanName = req.PlanName
	cfg.SSHUser = req.SSHUser
	cfg.SSHPort = req.SSHPort
	cfg.SSHPassword = req.SSHPassword
	cfg.WebServer = strings.ToLower(req.WebServer)
	cfg.SSHPrivateKey = req.SSHPrivateKey
	cfg.SSHPublicKey = req.SSHPublicKey
	cfg.DeployUser = req.DeployUser
	cfg.Timezone = req.Timezone
	cfg.ComponentsInput = req.ComponentsInput
	cfg.PlanInputFile = req.PlanInputFile
	cfg.PlanFile = req.PlanFile
	cfg.TraefikEmail = req.TraefikEmail
	cfg.TraefikChallenge = strings.ToLower(req.TraefikChallenge)
	cfg.TraefikDNSProvider = req.TraefikDNSProvider

	cfg.Provided.SSHUser = req.Provided.SSHUser
	cfg.Provided.SSHPort = req.Provided.SSHPort
	cfg.Provided.SSHPassword = req.Provided.SSHPassword
	cfg.Provided.WebServer = req.Provided.WebServer
	cfg.Provided.SSHPrivateKey = req.Provided.SSHPrivateKey
	cfg.Provided.SSHPublicKey = req.Provided.SSHPublicKey
	cfg.Provided.DeployUser = req.Provided.DeployUser
	cfg.Provided.Timezone = req.Provided.Timezone
	cfg.Provided.Components = req.Provided.Components
	cfg.Provided.PlanInputFile = req.Provided.PlanInputFile
	cfg.Provided.PlanFile = req.Provided.PlanFile
	cfg.Provided.TraefikEmail = req.Provided.TraefikEmail
	cfg.Provided.TraefikChallenge = req.Provided.TraefikChallenge
	cfg.Provided.TraefikDNSProvider = req.Provided.TraefikDNSProvider
	cfg.Provided.Servers = req.Provided.Servers
}

func parseServers(raw []string) ([]serverSpec, error) {
	servers := make([]serverSpec, 0, len(raw))
	for _, item := range raw {
		server, err := parseServerSpec(item)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	return servers, nil
}
