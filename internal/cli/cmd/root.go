package cmd

import (
	"civa/internal/cli/domain"
	"strings"

	"github.com/spf13/cobra"
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

type planStartFlags struct {
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
		helpTarget := normalizeHelpTarget(cmd.Name())
		_ = r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: helpTarget})
	})

	root.AddCommand(
		r.newVersionCommand(),
		r.newCompletionCommand(),
		r.newDoctorCommand(globals),
		r.newSetupCommand(globals),
		r.newPlanCommand(globals),
		r.newPreviewCommand(globals),
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
		sshUser      string
		sshPort      int
		sshPassword  string
		sshPublicKey string
		servers      []string
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
				Command:      domain.CommandSetup,
				SSHUser:      flags.sshUser,
				SSHPort:      flags.sshPort,
				SSHPassword:  flags.sshPassword,
				SSHPublicKey: flags.sshPublicKey,
				Servers:      append([]string(nil), flags.servers...),
				Provided: domain.ProvidedFlags{
					SSHUser:      command.Flags().Changed("ssh-user"),
					SSHPort:      command.Flags().Changed("ssh-port"),
					SSHPassword:  command.Flags().Changed("ssh-password"),
					SSHPublicKey: command.Flags().Changed("ssh-public-key"),
					Servers:      command.Flags().Changed("server"),
				},
			}
			req = r.withGlobalFlags(command, globals, req)
			return r.executor.Execute(req)
		},
	}

	cmd.Flags().StringVar(&flags.sshUser, "ssh-user", "root", "SSH user used to connect to every target server")
	cmd.Flags().IntVar(&flags.sshPort, "ssh-port", 22, "SSH port used to connect to every target server")
	cmd.Flags().StringVar(&flags.sshPassword, "ssh-password", "", "SSH password used by civa setup")
	cmd.Flags().StringVar(&flags.sshPublicKey, "ssh-public-key", "~/.ssh/id_ed25519.pub", "Local public key path that will be installed for the deploy user")
	cmd.Flags().StringArrayVar(&flags.servers, "server", nil, "Add a target server as addr[,hostname][,port]; hostname and SSH port are optional")

	return cmd
}

func (r *Root) newPlanCommand(globals *globalFlags) *cobra.Command {
	planFlags := defaultPlanStartFlags()

	planCmd := &cobra.Command{
		Use:   string(domain.CommandPlan),
		Short: "Generate and manage execution plans",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !hasAnyPlanStartInput(cmd) {
				return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandPlan)})
			}
			req := r.planStartRequest(cmd, globals, planFlags)
			return r.executor.Execute(req)
		},
	}
	r.bindPlanStartFlags(planCmd, planFlags)

	startFlags := defaultPlanStartFlags()
	startCmd := &cobra.Command{
		Use:   domain.PlanActionStart,
		Short: "Generate inventory, vars, and execution plan only",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.planStartRequest(cmd, globals, startFlags)
			return r.executor.Execute(req)
		},
	}
	r.bindPlanStartFlags(startCmd, startFlags)

	listCmd := &cobra.Command{
		Use:   domain.PlanActionList,
		Short: "List generated plans",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandPlan, PlanAction: domain.PlanActionList})
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

	planCmd.AddCommand(startCmd, listCmd, removeCmd)
	return planCmd
}

func (r *Root) newPreviewCommand(globals *globalFlags) *cobra.Command {
	var planFile string

	previewCmd := &cobra.Command{
		Use:   string(domain.CommandPreview) + " [plan-name]",
		Short: "Show an existing generated plan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && strings.TrimSpace(planFile) == "" {
				return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandPreview)})
			}

			req := domain.Request{
				Command:       domain.CommandPreview,
				PlanInputFile: planFile,
				Provided:      domain.ProvidedFlags{PlanInputFile: cmd.Flags().Changed("plan-file")},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	previewCmd.Flags().StringVar(&planFile, "plan-file", "", "Existing plan file override used by preview or apply")
	return previewCmd
}

func (r *Root) newApplyCommand(globals *globalFlags) *cobra.Command {
	var applyPlanFile string
	var reviewPlanFile string

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
	applyCmd.Flags().StringVar(&applyPlanFile, "plan-file", "", "Existing plan file override used by preview or apply")

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
	reviewCmd.Flags().StringVar(&reviewPlanFile, "plan-file", "", "Existing plan file override used by preview or apply")

	applyCmd.AddCommand(reviewCmd)
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

func (r *Root) bindPlanStartFlags(cmd *cobra.Command, flags *planStartFlags) {
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
	cmd.Flags().StringVar(&flags.planFile, "output", "", "Extra exported Markdown copy for plan start")
}

func defaultPlanStartFlags() *planStartFlags {
	return &planStartFlags{
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

func (r *Root) planStartRequest(cmd *cobra.Command, globals *globalFlags, flags *planStartFlags) domain.Request {
	req := domain.Request{
		Command:            domain.CommandPlan,
		PlanAction:         domain.PlanActionStart,
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

func hasAnyPlanStartInput(cmd *cobra.Command) bool {
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
	case domain.PlanActionStart, domain.PlanActionList, domain.PlanActionRemove:
		return string(domain.CommandPlan)
	case domain.ApplyActionReview:
		return string(domain.CommandApply)
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
