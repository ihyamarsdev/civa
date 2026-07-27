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
			printUsage(req.NonInteractive)
			return nil
		}
		printCommandUsage(target, req.NonInteractive)
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
	case domain.CommandAuth:
		cfg := defaultConfig(commandAuth)
		applyGlobalRequest(req, &cfg)
		cfg.AuthProvider = strings.ToLower(strings.TrimSpace(req.AuthProvider))
		cfg.AuthAction = strings.ToLower(strings.TrimSpace(req.AuthAction))
		cfg.AuthProfile = req.AuthProfile
		cfg.AuthToken = req.AuthToken
		cfg.Provided.AuthProfile = req.Provided.AuthProfile
		cfg.Provided.AuthToken = req.Provided.AuthToken
		return runAuthFlow(&cfg)
	case domain.CommandTools:
		cfg := defaultConfig(commandTools)
		applyGlobalRequest(req, &cfg)
		cfg.ToolsProvider = strings.ToLower(strings.TrimSpace(req.ToolsProvider))
		cfg.ToolsAction = strings.ToLower(strings.TrimSpace(req.ToolsAction))
		cfg.ToolsOperation = strings.ToLower(strings.TrimSpace(req.ToolsOperation))
		cfg.AuthProfile = req.AuthProfile
		cfg.CloudflareAccountID = req.CloudflareAccountID
		cfg.CloudflareZoneID = req.CloudflareZoneID
		cfg.CloudflareZoneName = req.CloudflareZoneName
		cfg.CloudflareZoneType = req.CloudflareZoneType
		cfg.CloudflareZonePausedInput = req.CloudflareZonePausedInput
		cfg.CloudflareTunnelID = req.CloudflareTunnelID
		cfg.CloudflareTunnelName = req.CloudflareTunnelName
		cfg.CloudflareHostname = req.CloudflareHostname
		cfg.CloudflareService = req.CloudflareService
		cfg.Provided.AuthProfile = req.Provided.AuthProfile
		cfg.Provided.ToolsOperation = req.Provided.ToolsOperation
		cfg.Provided.CloudflareAccount = req.Provided.CloudflareAccount
		cfg.Provided.CloudflareZoneID = req.Provided.CloudflareZoneID
		cfg.Provided.CloudflareZoneName = req.Provided.CloudflareZoneName
		cfg.Provided.CloudflareZoneType = req.Provided.CloudflareZoneType
		cfg.Provided.CloudflarePaused = req.Provided.CloudflarePaused
		cfg.Provided.CloudflareTunnelID = req.Provided.CloudflareTunnelID
		cfg.Provided.CloudflareTunnelName = req.Provided.CloudflareTunnelName
		cfg.Provided.CloudflareHostname = req.Provided.CloudflareHostname
		cfg.Provided.CloudflareService = req.Provided.CloudflareService
		return runToolsFlow(&cfg)
	case domain.CommandConfig:
		cfg := defaultConfig(commandConfig)
		applyGlobalRequest(req, &cfg)
		applySharedRequest(req, &cfg)
		if req.ConfigAction != "" {
			cfg.ConfigAction = req.ConfigAction
		}
		return runConfigFlow(&cfg)
	case domain.CommandSecret:
		cfg := defaultConfig(commandSecret)
		applyGlobalRequest(req, &cfg)
		cfg.SecretAction = req.SecretAction
		cfg.SecretName = req.SecretName
		cfg.SecretValue = req.SecretValue
		cfg.SecretValueFile = req.SecretValueFile
		cfg.Provided.SecretValue = req.Provided.SecretValue
		cfg.Provided.SecretValueFile = req.Provided.SecretValueFile
		if cfg.SecretAction == "" {
			cfg.SecretAction = secretActionList
		}
		return runSecretFlow(&cfg)
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
		case planActionReview:
			return runPlanReviewFlow(&cfg)
		case planActionEdit:
			return runPlanEditFlow(&cfg)
		case planActionRemove:
			cfg.PlanName = req.PlanName
			return runPlanRemoveFlow(&cfg)
		default:
			cfg.PlanAction = planActionInit
			return runPlanFlow(&cfg)
		}
	case domain.CommandApply:
		cfg := defaultConfig(commandApply)
		applyGlobalRequest(req, &cfg)
		if req.ApplyAction != "" {
			cfg.ApplyAction = req.ApplyAction
		}
		cfg.PlanName = req.PlanName
		cfg.PlanInputFile = req.PlanInputFile
		cfg.Provided.PlanInputFile = req.Provided.PlanInputFile

		switch cfg.ApplyAction {
		case applyActionReview:
			return runApplyReviewFlow(&cfg)
		case applyActionDrift:
			return runApplyDriftFlow(&cfg)
		case applyActionRollback:
			return runApplyRollbackFlow(&cfg)
		}
		cfg.ApplyAction = applyActionExecute
		return runApplyFlow(&cfg)
	case domain.CommandPlaybook:
		cfg := defaultConfig(commandPlaybook)
		applyGlobalRequest(req, &cfg)
		cfg.PlaybookAction = strings.ToLower(strings.TrimSpace(req.PlaybookAction))
		cfg.PlaybookName = req.PlaybookName
		cfg.PlaybookFile = req.PlaybookFile
		cfg.PlanName = req.PlanName
		cfg.PlanInputFile = req.PlanInputFile
		cfg.Provided.PlanInputFile = req.Provided.PlanInputFile
		cfg.Provided.PlaybookName = req.Provided.PlaybookName
		cfg.Provided.PlaybookFile = req.Provided.PlaybookFile
		if cfg.PlaybookAction == "" {
			cfg.PlaybookAction = playbookActionList
		}
		return runPlaybookFlow(&cfg)
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
	cfg.SSHPasswordSecret = req.SSHPasswordSecret
	cfg.WebServer = strings.ToLower(req.WebServer)
	cfg.SSHPrivateKey = req.SSHPrivateKey
	cfg.SSHPublicKey = req.SSHPublicKey
	cfg.DeployUser = req.DeployUser
	cfg.Timezone = req.Timezone
	cfg.ComponentsInput = req.ComponentsInput
	cfg.PlanInputFile = req.PlanInputFile
	cfg.PlanFile = req.PlanFile
	cfg.PlaybookName = req.PlaybookName
	cfg.PlaybookFile = req.PlaybookFile
	cfg.TraefikEmail = req.TraefikEmail
	cfg.TraefikChallenge = strings.ToLower(req.TraefikChallenge)
	cfg.TraefikDNSProvider = req.TraefikDNSProvider

	cfg.Provided.SSHUser = req.Provided.SSHUser
	cfg.Provided.SSHPort = req.Provided.SSHPort
	cfg.Provided.SSHPassword = req.Provided.SSHPassword
	cfg.Provided.SSHPasswordSecret = req.Provided.SSHPasswordSecret
	cfg.Provided.WebServer = req.Provided.WebServer
	cfg.Provided.SSHPrivateKey = req.Provided.SSHPrivateKey
	cfg.Provided.SSHPublicKey = req.Provided.SSHPublicKey
	cfg.Provided.DeployUser = req.Provided.DeployUser
	cfg.Provided.Timezone = req.Provided.Timezone
	cfg.Provided.Components = req.Provided.Components
	cfg.Provided.PlanInputFile = req.Provided.PlanInputFile
	cfg.Provided.PlanFile = req.Provided.PlanFile
	cfg.Provided.PlaybookName = req.Provided.PlaybookName
	cfg.Provided.PlaybookFile = req.Provided.PlaybookFile
	cfg.Provided.TraefikEmail = req.Provided.TraefikEmail
	cfg.Provided.TraefikChallenge = req.Provided.TraefikChallenge
	cfg.Provided.TraefikDNSProvider = req.Provided.TraefikDNSProvider
	cfg.Provided.Servers = req.Provided.Servers
	cfg.Provided.SecretValue = req.Provided.SecretValue
	cfg.Provided.SecretValueFile = req.Provided.SecretValueFile
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
