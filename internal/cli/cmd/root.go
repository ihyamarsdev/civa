package cmd

import (
	"civa/internal/cli/domain"
	"strings"

	"github.com/spf13/cobra"
)

const (
	helpTargetConfigNginx = "config-nginx"
	helpTargetConfigCaddy = "config-caddy"
	helpTargetConfigAll   = "config-all"
)

type Root struct {
	executor domain.Executor
}

func NewRoot(executor domain.Executor) *Root {
	return &Root{executor: executor}
}

func (r *Root) Run(args []string) error {
	rootCmd := r.newRootCommand()
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

type globalFlags struct {
	assumeYes      bool
	nonInteractive bool
}

type planInitFlags struct {
	sshUser            string
	sshPort            int
	webServer          string
	sshPrivateKey      string
	sshPublicKey       string
	deployUser         string
	timezone           string
	componentsInput    string
	planFile           string
	traefikEmail       string
	traefikChallenge   string
	traefikDNSProvider string
	servers            []string
}

func (r *Root) newRootCommand() *cobra.Command {
	globals := &globalFlags{}

	root := &cobra.Command{
		Use:           "civa",
		Short:         "civa CLI for VPS automation",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp})
		},
	}

	root.PersistentFlags().BoolVarP(&globals.assumeYes, "yes", "y", false, "Skip confirmation prompts for destructive commands")
	root.PersistentFlags().BoolVar(&globals.nonInteractive, "non-interactive", false, "Disable prompts and rely on provided flags")
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		helpTarget := normalizeHelpTargetCommand(cmd)
		_ = r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: helpTarget})
	})

	root.AddCommand(
		r.newVersionCommand(),
		r.newCompletionCommand(),
		r.newDoctorCommand(globals),
		r.newSetupCommand(globals),
		r.newSecretCommand(globals),
		r.newConfigCommand(globals),
		r.newPlanCommand(globals),
		r.newApplyCommand(globals),
		r.newUninstallCommand(globals),
		r.newHiddenCompleteCommand(),
	)

	return root
}

func (r *Root) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   string(domain.CommandVersion),
		Short: "Show the civa version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandVersion})
		},
	}
}

func (r *Root) newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   string(domain.CommandCompletion) + " [shell]",
		Short: "Print shell completion for bash, zsh, or fish",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandCompletion)})
			}
			return r.executor.Execute(domain.Request{Command: domain.CommandCompletion, CompletionArgs: args})
		},
	}
}

func (r *Root) newHiddenCompleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:    string(domain.CommandCompleteInternal),
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandCompleteInternal, CompletionArgs: args})
		},
	}
}

func (r *Root) newDoctorCommand(globals *globalFlags) *cobra.Command {
	doctorCmd := &cobra.Command{
		Use:   string(domain.CommandDoctor),
		Short: "Check or install local dependencies",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandDoctor, DoctorAction: domain.DoctorActionCheck})
			return r.executor.Execute(req)
		},
	}

	fixCmd := &cobra.Command{
		Use:   domain.DoctorActionFix,
		Short: "Install or update required local dependencies",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandDoctor, DoctorAction: domain.DoctorActionFix})
			return r.executor.Execute(req)
		},
	}

	checkCmd := &cobra.Command{
		Use:    domain.DoctorActionCheck,
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandDoctor, DoctorAction: domain.DoctorActionCheck})
			return r.executor.Execute(req)
		},
	}

	doctorCmd.AddCommand(fixCmd, checkCmd)
	return doctorCmd
}

func (r *Root) newSetupCommand(globals *globalFlags) *cobra.Command {
	flags := struct {
		sshUser           string
		sshPort           int
		sshPassword       string
		sshPasswordSecret string
		sshPublicKey      string
		servers           []string
	}{
		sshUser:      "root",
		sshPort:      22,
		sshPublicKey: "~/.ssh/id_ed25519.pub",
	}

	cmd := &cobra.Command{
		Use:   string(domain.CommandSetup),
		Short: "Install a public SSH key on a server with ssh-copy-id",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:           domain.CommandSetup,
				SSHUser:           flags.sshUser,
				SSHPort:           flags.sshPort,
				SSHPassword:       flags.sshPassword,
				SSHPasswordSecret: flags.sshPasswordSecret,
				SSHPublicKey:      flags.sshPublicKey,
				Servers:           append([]string(nil), flags.servers...),
				Provided: domain.ProvidedFlags{
					SSHUser:           command.Flags().Changed("ssh-user"),
					SSHPort:           command.Flags().Changed("ssh-port"),
					SSHPassword:       command.Flags().Changed("ssh-password"),
					SSHPasswordSecret: command.Flags().Changed("ssh-password-secret"),
					SSHPublicKey:      command.Flags().Changed("ssh-public-key"),
					Servers:           command.Flags().Changed("server"),
				},
			}
			req = r.withGlobalFlags(command, globals, req)
			return r.executor.Execute(req)
		},
	}

	cmd.Flags().StringVar(&flags.sshUser, "ssh-user", "root", "SSH user used to connect to every target server")
	cmd.Flags().IntVar(&flags.sshPort, "ssh-port", 22, "SSH port used to connect to every target server")
	cmd.Flags().StringVar(&flags.sshPassword, "ssh-password", "", "SSH password used by civa setup")
	cmd.Flags().StringVar(&flags.sshPasswordSecret, "ssh-password-secret", "", "Secret name in civa secret store for SSH password")
	cmd.Flags().StringVar(&flags.sshPublicKey, "ssh-public-key", "~/.ssh/id_ed25519.pub", "Local public key path that will be installed for the deploy user")
	cmd.Flags().StringArrayVar(&flags.servers, "server", nil, "Add a target server as addr[,hostname][,port]; hostname and SSH port are optional")

	return cmd
}

func (r *Root) newSecretCommand(globals *globalFlags) *cobra.Command {
	var setValue string
	var setValueFile string

	secretCmd := &cobra.Command{
		Use:   string(domain.CommandSecret),
		Short: "Manage encrypted secrets used by civa runtime",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandSecret)})
		},
	}

	setCmd := &cobra.Command{
		Use:   domain.SecretActionSet + " <name>",
		Short: "Store or update an encrypted secret value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:         domain.CommandSecret,
				SecretAction:    domain.SecretActionSet,
				SecretName:      args[0],
				SecretValue:     setValue,
				SecretValueFile: setValueFile,
				Provided: domain.ProvidedFlags{
					SecretValue:     cmd.Flags().Changed("value"),
					SecretValueFile: cmd.Flags().Changed("value-file"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	setCmd.Flags().StringVar(&setValue, "value", "", "Secret value to encrypt and store")
	setCmd.Flags().StringVar(&setValueFile, "value-file", "", "Path to file containing secret value to encrypt and store")

	listCmd := &cobra.Command{
		Use:   domain.SecretActionList,
		Short: "List secret names in the encrypted store",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandSecret, SecretAction: domain.SecretActionList})
			return r.executor.Execute(req)
		},
	}

	removeCmd := &cobra.Command{
		Use:   domain.SecretActionRemove + " <name>",
		Short: "Remove an encrypted secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandSecret, SecretAction: domain.SecretActionRemove, SecretName: args[0]})
			return r.executor.Execute(req)
		},
	}

	secretCmd.AddCommand(setCmd, listCmd, removeCmd)
	return secretCmd
}

func (r *Root) newConfigCommand(globals *globalFlags) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   string(domain.CommandConfig),
		Short: "Configure persistent civa settings with provider-scoped init/list/remove",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandConfig)})
		},
	}

	legacyEditCmd := &cobra.Command{
		Use:   domain.ConfigActionEdit + " [plan-name]",
		Short: "Deprecated: use civa config <nginx|caddy> init [plan-name]",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandConfig, ConfigAction: domain.ConfigActionInit})
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			return r.executor.Execute(req)
		},
	}
	legacyEditCmd.Hidden = true

	legacyInitCmd := &cobra.Command{
		Use:   domain.ConfigActionInit + " [plan-name]",
		Short: "Deprecated: use civa config <nginx|caddy> init [plan-name]",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandConfig, ConfigAction: domain.ConfigActionInit})
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			return r.executor.Execute(req)
		},
	}
	legacyInitCmd.Hidden = true

	legacyListCmd := &cobra.Command{
		Use:   domain.ConfigActionList,
		Short: "Deprecated: use civa config <provider> list",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandConfig, ConfigAction: domain.ConfigActionList, WebServer: "all", Provided: domain.ProvidedFlags{WebServer: true}})
			return r.executor.Execute(req)
		},
	}
	legacyListCmd.Hidden = true

	legacyRemoveCmd := &cobra.Command{
		Use:   domain.ConfigActionRemove + " <provider> <plan-name>",
		Short: "Deprecated: use civa config <nginx|caddy> remove <plan-name>",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandConfig, ConfigAction: domain.ConfigActionRemove})
			req.WebServer = strings.ToLower(args[0])
			req.Provided.WebServer = true
			req.PlanName = args[1]
			return r.executor.Execute(req)
		},
	}
	legacyRemoveCmd.Hidden = true

	providerCommand := func(provider string, enableInit bool, enableRemove bool) *cobra.Command {
		providerCmd := &cobra.Command{
			Use:   provider,
			Short: "Provider-scoped config actions (init/list/remove)",
			Args:  cobra.NoArgs,
			RunE: func(_ *cobra.Command, _ []string) error {
				return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: configProviderHelpTarget(provider)})
			},
		}

		if enableInit {
			initCmd := &cobra.Command{
				Use:   domain.ConfigActionInit + " [plan-name]",
				Short: "Initialize or update persisted config profile for this provider",
				Args:  cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandConfig, ConfigAction: domain.ConfigActionInit, WebServer: provider, Provided: domain.ProvidedFlags{WebServer: true}})
					if len(args) == 1 {
						req.PlanName = args[0]
					}
					return r.executor.Execute(req)
				},
			}

			legacyEditCmd := &cobra.Command{
				Use:        domain.ConfigActionEdit + " [plan-name]",
				Short:      "Deprecated: use init",
				Deprecated: "use civa config " + provider + " init [plan-name]",
				Hidden:     true,
				Args:       cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandConfig, ConfigAction: domain.ConfigActionInit, WebServer: provider, Provided: domain.ProvidedFlags{WebServer: true}})
					if len(args) == 1 {
						req.PlanName = args[0]
					}
					return r.executor.Execute(req)
				},
			}

			providerCmd.AddCommand(initCmd, legacyEditCmd)
		}

		listCmd := &cobra.Command{
			Use:   domain.ConfigActionList,
			Short: "List persisted config profile for this provider",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandConfig, ConfigAction: domain.ConfigActionList, WebServer: provider, Provided: domain.ProvidedFlags{WebServer: true}})
				return r.executor.Execute(req)
			},
		}

		if enableRemove {
			removeCmd := &cobra.Command{
				Use:   domain.ConfigActionRemove + " <plan-name>",
				Short: "Remove persisted config profile for this provider",
				Args:  cobra.ExactArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandConfig, ConfigAction: domain.ConfigActionRemove, WebServer: provider, Provided: domain.ProvidedFlags{WebServer: true}})
					req.PlanName = args[0]
					return r.executor.Execute(req)
				},
			}
			providerCmd.AddCommand(listCmd, removeCmd)
			return providerCmd
		}

		providerCmd.AddCommand(listCmd)
		return providerCmd
	}

	configCmd.AddCommand(providerCommand("nginx", true, true), providerCommand("caddy", true, true), providerCommand("all", false, false), legacyEditCmd, legacyInitCmd, legacyListCmd, legacyRemoveCmd)
	return configCmd
}

func configProviderHelpTarget(provider string) string {
	switch provider {
	case "nginx":
		return helpTargetConfigNginx
	case "caddy":
		return helpTargetConfigCaddy
	case "all":
		return helpTargetConfigAll
	default:
		return string(domain.CommandConfig)
	}
}

func (r *Root) newPlanCommand(globals *globalFlags) *cobra.Command {
	planFlags := defaultPlanInitFlags()
	var reviewPlanFile string
	var editPlanFile string

	planCmd := &cobra.Command{
		Use:   string(domain.CommandPlan),
		Short: "Generate and manage execution plans",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !hasAnyPlanInitInput(cmd) {
				return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandPlan)})
			}
			req := r.planInitRequest(cmd, globals, planFlags)
			return r.executor.Execute(req)
		},
	}
	r.bindPlanInitFlags(planCmd, planFlags)

	initFlags := defaultPlanInitFlags()
	initCmd := &cobra.Command{
		Use:   domain.PlanActionInit,
		Short: "Generate inventory, vars, and execution plan only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.planInitRequest(cmd, globals, initFlags)
			return r.executor.Execute(req)
		},
	}
	r.bindPlanInitFlags(initCmd, initFlags)

	reviewCmd := &cobra.Command{
		Use:   domain.PlanActionReview + " [plan-name]",
		Short: "Show an existing generated plan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && strings.TrimSpace(reviewPlanFile) == "" {
				return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandPlan)})
			}

			req := domain.Request{
				Command:       domain.CommandPlan,
				PlanAction:    domain.PlanActionReview,
				PlanInputFile: reviewPlanFile,
				Provided:      domain.ProvidedFlags{PlanInputFile: cmd.Flags().Changed("plan-file")},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	reviewCmd.Flags().StringVar(&reviewPlanFile, "plan-file", "", "Existing plan file override used by plan review or apply")

	editCmd := &cobra.Command{
		Use:   domain.PlanActionEdit + " [plan-name]",
		Short: "Edit an existing generated plan with your editor",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && strings.TrimSpace(editPlanFile) == "" {
				return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandPlan)})
			}

			req := domain.Request{
				Command:       domain.CommandPlan,
				PlanAction:    domain.PlanActionEdit,
				PlanInputFile: editPlanFile,
				Provided:      domain.ProvidedFlags{PlanInputFile: cmd.Flags().Changed("plan-file")},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	editCmd.Flags().StringVar(&editPlanFile, "plan-file", "", "Existing plan file override used by plan edit or apply")

	listCmd := &cobra.Command{
		Use:   domain.PlanActionList + " [plan-name]",
		Short: "List generated plans or list versions for a plan name",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{Command: domain.CommandPlan, PlanAction: domain.PlanActionList}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	removeCmd := &cobra.Command{
		Use:   domain.PlanActionRemove + " [plan-name]",
		Short: "Remove a generated plan and its artifacts",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{Command: domain.CommandPlan, PlanAction: domain.PlanActionRemove}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	planCmd.AddCommand(initCmd, reviewCmd, editCmd, listCmd, removeCmd)
	return planCmd
}

func (r *Root) newApplyCommand(globals *globalFlags) *cobra.Command {
	var applyPlanFile string
	var reviewPlanFile string
	var driftPlanFile string
	var rollbackPlanFile string

	applyCmd := &cobra.Command{
		Use:   string(domain.CommandApply) + " [plan-name]",
		Short: "Execute an existing generated plan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && strings.TrimSpace(applyPlanFile) == "" {
				return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandApply)})
			}

			req := domain.Request{
				Command:       domain.CommandApply,
				ApplyAction:   domain.ApplyActionExecute,
				PlanInputFile: applyPlanFile,
				Provided:      domain.ProvidedFlags{PlanInputFile: cmd.Flags().Changed("plan-file")},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	applyCmd.Flags().StringVar(&applyPlanFile, "plan-file", "", "Existing plan file override used by plan review/edit or apply")

	reviewCmd := &cobra.Command{
		Use:   domain.ApplyActionReview + " [plan-name]",
		Short: "Verify an applied plan with ansible check mode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:       domain.CommandApply,
				ApplyAction:   domain.ApplyActionReview,
				PlanInputFile: reviewPlanFile,
				Provided:      domain.ProvidedFlags{PlanInputFile: cmd.Flags().Changed("plan-file")},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	reviewCmd.Flags().StringVar(&reviewPlanFile, "plan-file", "", "Existing plan file override used by plan review/edit or apply")

	driftCmd := &cobra.Command{
		Use:   domain.ApplyActionDrift + " [plan-name]",
		Short: "Detect infrastructure drift using ansible check mode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:       domain.CommandApply,
				ApplyAction:   domain.ApplyActionDrift,
				PlanInputFile: driftPlanFile,
				Provided:      domain.ProvidedFlags{PlanInputFile: cmd.Flags().Changed("plan-file")},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	driftCmd.Flags().StringVar(&driftPlanFile, "plan-file", "", "Existing plan file override used by plan review/edit or apply")

	rollbackCmd := &cobra.Command{
		Use:   domain.ApplyActionRollback + " [plan-name]",
		Short: "Rollback to the last successful plan or a specific plan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:       domain.CommandApply,
				ApplyAction:   domain.ApplyActionRollback,
				PlanInputFile: rollbackPlanFile,
				Provided:      domain.ProvidedFlags{PlanInputFile: cmd.Flags().Changed("plan-file")},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	rollbackCmd.Flags().StringVar(&rollbackPlanFile, "plan-file", "", "Existing plan file override used by plan review/edit or apply")

	applyCmd.AddCommand(reviewCmd, driftCmd, rollbackCmd)
	return applyCmd
}

func (r *Root) newUninstallCommand(globals *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   string(domain.CommandUninstall),
		Short: "Remove the currently installed civa binary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandUninstall})
			return r.executor.Execute(req)
		},
	}
}

func (r *Root) bindPlanInitFlags(cmd *cobra.Command, flags *planInitFlags) {
	cmd.Flags().StringVar(&flags.sshUser, "ssh-user", "root", "SSH user used to connect to every target server")
	cmd.Flags().IntVar(&flags.sshPort, "ssh-port", 22, "SSH port used to connect to every target server")
	cmd.Flags().StringVar(&flags.webServer, "web-server", "none", "Web server to prepare: traefik, nginx, caddy, or none")
	cmd.Flags().StringVar(&flags.sshPrivateKey, "ssh-private-key", "~/.ssh/id_ed25519", "Local private key path used by Ansible for SSH")
	cmd.Flags().StringVar(&flags.sshPublicKey, "ssh-public-key", "~/.ssh/id_ed25519.pub", "Local public key path that will be installed for the deploy user")
	cmd.Flags().StringVar(&flags.deployUser, "deployer-user", "deployer", "User created and configured on the target servers")
	cmd.Flags().StringVar(&flags.timezone, "timezone", "Asia/Jakarta", "Timezone applied to the target servers")
	cmd.Flags().StringVar(&flags.componentsInput, "components", "all", "Components to run: all or a comma list such as 1,2,4 or docker,traefik")
	cmd.Flags().StringArrayVar(&flags.servers, "server", nil, "Add a target server as addr[,hostname][,port]; hostname and SSH port are optional")
	cmd.Flags().StringVar(&flags.traefikEmail, "traefik-email", "", "Email used by Let's Encrypt ACME")
	cmd.Flags().StringVar(&flags.traefikChallenge, "traefik-challenge", "http", "Traefik challenge type: http or dns")
	cmd.Flags().StringVar(&flags.traefikDNSProvider, "traefik-dns-provider", "cloudflare", "DNS provider name used when challenge type is dns")
	cmd.Flags().StringVar(&flags.planFile, "output", "", "Extra exported Markdown copy for plan init")
}

func defaultPlanInitFlags() *planInitFlags {
	return &planInitFlags{
		sshUser:            "root",
		sshPort:            22,
		webServer:          "none",
		sshPrivateKey:      "~/.ssh/id_ed25519",
		sshPublicKey:       "~/.ssh/id_ed25519.pub",
		deployUser:         "deployer",
		timezone:           "Asia/Jakarta",
		componentsInput:    "all",
		traefikChallenge:   "http",
		traefikDNSProvider: "cloudflare",
	}
}

func (r *Root) planInitRequest(cmd *cobra.Command, globals *globalFlags, flags *planInitFlags) domain.Request {
	req := domain.Request{
		Command:            domain.CommandPlan,
		PlanAction:         domain.PlanActionInit,
		SSHUser:            flags.sshUser,
		SSHPort:            flags.sshPort,
		WebServer:          strings.ToLower(flags.webServer),
		SSHPrivateKey:      flags.sshPrivateKey,
		SSHPublicKey:       flags.sshPublicKey,
		DeployUser:         flags.deployUser,
		Timezone:           flags.timezone,
		ComponentsInput:    flags.componentsInput,
		PlanFile:           flags.planFile,
		TraefikEmail:       flags.traefikEmail,
		TraefikChallenge:   strings.ToLower(flags.traefikChallenge),
		TraefikDNSProvider: flags.traefikDNSProvider,
		Servers:            append([]string(nil), flags.servers...),
		Provided: domain.ProvidedFlags{
			SSHUser:            cmd.Flags().Changed("ssh-user"),
			SSHPort:            cmd.Flags().Changed("ssh-port"),
			WebServer:          cmd.Flags().Changed("web-server"),
			SSHPrivateKey:      cmd.Flags().Changed("ssh-private-key"),
			SSHPublicKey:       cmd.Flags().Changed("ssh-public-key"),
			DeployUser:         cmd.Flags().Changed("deployer-user"),
			Timezone:           cmd.Flags().Changed("timezone"),
			Components:         cmd.Flags().Changed("components"),
			PlanFile:           cmd.Flags().Changed("output"),
			TraefikEmail:       cmd.Flags().Changed("traefik-email"),
			TraefikChallenge:   cmd.Flags().Changed("traefik-challenge"),
			TraefikDNSProvider: cmd.Flags().Changed("traefik-dns-provider"),
			Servers:            cmd.Flags().Changed("server"),
		},
	}

	return r.withGlobalFlags(cmd, globals, req)
}

func (r *Root) withGlobalFlags(cmd *cobra.Command, globals *globalFlags, req domain.Request) domain.Request {
	req.AssumeYes = globals.assumeYes
	req.NonInteractive = globals.nonInteractive
	req.Provided.NonInteractive = isFlagChanged(cmd, "non-interactive")
	return req
}

func hasAnyPlanInitInput(cmd *cobra.Command) bool {
	if cmd.Flags().NFlag() > 0 {
		return true
	}

	return isFlagChanged(cmd, "yes") || isFlagChanged(cmd, "non-interactive")
}

func isFlagChanged(cmd *cobra.Command, name string) bool {
	if flag := cmd.Flags().Lookup(name); flag != nil && flag.Changed {
		return true
	}
	if flag := cmd.InheritedFlags().Lookup(name); flag != nil && flag.Changed {
		return true
	}
	return false
}

func normalizeHelpTarget(name string) string {
	switch name {
	case domain.PlanActionInit, domain.PlanActionList, domain.PlanActionRemove:
		return string(domain.CommandPlan)
	case domain.ApplyActionReview, domain.ApplyActionDrift, domain.ApplyActionRollback:
		return string(domain.CommandApply)
	case domain.ConfigActionEdit:
		return string(domain.CommandConfig)
	case domain.SecretActionSet:
		return string(domain.CommandSecret)
	case domain.DoctorActionCheck, domain.DoctorActionFix:
		return string(domain.CommandDoctor)
	case "", "civa":
		return ""
	default:
		if strings.HasPrefix(name, "-") {
			return ""
		}
		return name
	}
}

func normalizeHelpTargetCommand(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}

	name := cmd.Name()
	if parent := cmd.Parent(); parent != nil {
		switch parent.Name() {
		case string(domain.CommandSecret):
			if name == domain.SecretActionSet || name == domain.SecretActionList || name == domain.SecretActionRemove {
				return string(domain.CommandSecret)
			}
		case string(domain.CommandConfig):
			if name == "nginx" || name == "caddy" || name == "all" {
				return configProviderHelpTarget(name)
			}
			if name == domain.ConfigActionInit || name == domain.ConfigActionEdit || name == domain.ConfigActionList || name == domain.ConfigActionRemove {
				return string(domain.CommandConfig)
			}
		case "nginx", "caddy", "all":
			if grandParent := parent.Parent(); grandParent != nil && grandParent.Name() == string(domain.CommandConfig) {
				if name == domain.ConfigActionInit || name == domain.ConfigActionEdit || name == domain.ConfigActionList || name == domain.ConfigActionRemove || name == string(domain.CommandHelp) {
					return configProviderHelpTarget(parent.Name())
				}
			}
		case string(domain.CommandApply):
			if name == domain.ApplyActionReview || name == domain.ApplyActionDrift || name == domain.ApplyActionRollback {
				return string(domain.CommandApply)
			}
		case string(domain.CommandPlan):
			if name == domain.PlanActionInit || name == domain.PlanActionReview || name == domain.PlanActionEdit || name == domain.PlanActionList || name == domain.PlanActionRemove {
				return string(domain.CommandPlan)
			}
		case string(domain.CommandDoctor):
			if name == domain.DoctorActionCheck || name == domain.DoctorActionFix {
				return string(domain.CommandDoctor)
			}
		}
	}

	return normalizeHelpTarget(name)
}
