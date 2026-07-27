package cmd

import (
	"civa/internal/cli/domain"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	helpTargetConfigNginx = "config-nginx"
	helpTargetConfigCaddy = "config-caddy"
	helpTargetConfigAll   = "config-all"
	helpTargetAuthCF      = "auth-cloudflare"
	helpTargetToolsCF     = "tools-cloudflare"
	helpTargetToolsCFTunnels = "tools-cloudflare-tunnels"
	helpTargetDeployRun   = "deploy-run"
	wizardActionSetup     = "setup"
	wizardActionPlanInit  = "plan-init"
	wizardActionHelp      = "help"
	wizardActionExit      = "exit"
)

var errWizardCancelled = errors.New("wizard cancelled")

var promptWizardActionFn = promptWizardAction

var isTerminalFn = term.IsTerminal

var stdinFdFn = func() uintptr {
	return os.Stdin.Fd()
}

var stdoutFdFn = func() uintptr {
	return os.Stdout.Fd()
}

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
		RunE: func(cmd *cobra.Command, _ []string) error {
			return r.executor.Execute(r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandHelp}))
		},
	}

	root.PersistentFlags().BoolVarP(&globals.assumeYes, "yes", "y", false, "Skip confirmation prompts for destructive commands")
	root.PersistentFlags().BoolVar(&globals.nonInteractive, "non-interactive", false, "Disable prompts and rely on provided flags")
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		helpTarget := normalizeHelpTargetCommand(cmd)
		_ = r.executor.Execute(r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandHelp, HelpTarget: helpTarget}))
	})

	root.AddCommand(
		r.newVersionCommand(),
		r.newCompletionCommand(),
		r.newBootstrapCommand(globals),
		r.newDeployCommand(globals),
		r.newOpsCommand(globals),
		r.newDoctorCommand(globals),
		r.newStartCommand(globals),
		r.newSetupCommand(globals),
		r.newAuthCommand(globals),
		r.newToolsCommand(globals),
		r.newSecretCommand(globals),
		r.newConfigCommand(globals),
		r.newPlanCommand(globals),
		r.newPlaybookCommand(globals),
		r.newApplyCommand(globals),
		r.newUninstallCommand(globals),
		r.newHiddenCompleteCommand(),
	)

	return root
}

func (r *Root) newAuthCommand(globals *globalFlags) *cobra.Command {
	var cloudflareToken string

	authCmd := &cobra.Command{
		Use:   string(domain.CommandAuth),
		Short: "Manage external provider authentication profiles",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandAuth)})
		},
	}

	cloudflareCmd := &cobra.Command{
		Use:   domain.AuthProviderCloudflare,
		Short: "Manage Cloudflare API credential profiles",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: helpTargetAuthCF})
		},
	}

	setCmd := &cobra.Command{
		Use:   domain.AuthActionSet + " <profile>",
		Short: "Create or update a Cloudflare auth profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:      domain.CommandAuth,
				AuthProvider: domain.AuthProviderCloudflare,
				AuthAction:   domain.AuthActionSet,
				AuthProfile:  args[0],
				AuthToken:    cloudflareToken,
				Provided: domain.ProvidedFlags{
					AuthProfile: true,
					AuthToken:   cmd.Flags().Changed("token"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	setCmd.Flags().StringVar(&cloudflareToken, "token", "", "Cloudflare API token to store in this profile")

	getCmd := &cobra.Command{
		Use:   domain.AuthActionGet + " <profile>",
		Short: "Show Cloudflare auth profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:      domain.CommandAuth,
				AuthProvider: domain.AuthProviderCloudflare,
				AuthAction:   domain.AuthActionGet,
				AuthProfile:  args[0],
				Provided:     domain.ProvidedFlags{AuthProfile: true},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	listCmd := &cobra.Command{
		Use:   domain.AuthActionList,
		Short: "List Cloudflare auth profiles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{Command: domain.CommandAuth, AuthProvider: domain.AuthProviderCloudflare, AuthAction: domain.AuthActionList}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	removeCmd := &cobra.Command{
		Use:   domain.AuthActionRemove + " <profile>",
		Short: "Remove a Cloudflare auth profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:      domain.CommandAuth,
				AuthProvider: domain.AuthProviderCloudflare,
				AuthAction:   domain.AuthActionRemove,
				AuthProfile:  args[0],
				Provided:     domain.ProvidedFlags{AuthProfile: true},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	loginCmd := &cobra.Command{
		Use:   domain.AuthActionLogin + " [profile]",
		Short: "Login to Cloudflare via browser and save as a profile",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profile := "default"
			if len(args) > 0 {
				profile = args[0]
			}
			req := domain.Request{
				Command:      domain.CommandAuth,
				AuthProvider: domain.AuthProviderCloudflare,
				AuthAction:   domain.AuthActionLogin,
				AuthProfile:  profile,
				Provided: domain.ProvidedFlags{
					AuthProfile: true,
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	cloudflareCmd.AddCommand(loginCmd, setCmd, getCmd, listCmd, removeCmd)
	authCmd.AddCommand(cloudflareCmd)
	return authCmd
}

func (r *Root) newToolsCommand(globals *globalFlags) *cobra.Command {
	var authProfile string
	var zoneID string
	var zoneName string
	var zoneAccountID string
	var zoneType string
	var zonePaused string
	var tunnelID string
	var tunnelName string
	var hostname string
	var serviceURL string

	toolsCmd := &cobra.Command{
		Use:   string(domain.CommandTools),
		Short: "Run external provider tools (Cloudflare and others)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandTools})
			return r.executor.Execute(req)
		},
	}

	cloudflareCmd := &cobra.Command{
		Use:   domain.ToolsProviderCloudflare,
		Short: "Cloudflare helper tools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:       domain.CommandTools,
				ToolsProvider: domain.ToolsProviderCloudflare,
				AuthProfile:   authProfile,
				Provided: domain.ProvidedFlags{
					AuthProfile: cmd.Flags().Changed("profile"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	zonesCmd := &cobra.Command{
		Use:   domain.ToolsActionCloudflareZones,
		Short: "Manage Cloudflare zones (list/create/update/delete)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:       domain.CommandTools,
				ToolsProvider: domain.ToolsProviderCloudflare,
				ToolsAction:   domain.ToolsActionCloudflareZones,
				AuthProfile:   authProfile,
				Provided: domain.ProvidedFlags{
					AuthProfile: cmd.Flags().Changed("profile"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	zoneListCmd := &cobra.Command{
		Use:   domain.ToolsOperationList,
		Short: "List Cloudflare zones",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:        domain.CommandTools,
				ToolsProvider:  domain.ToolsProviderCloudflare,
				ToolsAction:    domain.ToolsActionCloudflareZones,
				ToolsOperation: domain.ToolsOperationList,
				AuthProfile:    authProfile,
				Provided:       domain.ProvidedFlags{AuthProfile: cmd.Flags().Changed("profile")},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	zoneCreateCmd := &cobra.Command{
		Use:   domain.ToolsOperationCreate,
		Short: "Create a Cloudflare zone",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:             domain.CommandTools,
				ToolsProvider:       domain.ToolsProviderCloudflare,
				ToolsAction:         domain.ToolsActionCloudflareZones,
				ToolsOperation:      domain.ToolsOperationCreate,
				AuthProfile:         authProfile,
				CloudflareZoneName:  zoneName,
				CloudflareAccountID: zoneAccountID,
				CloudflareZoneType:  zoneType,
				Provided: domain.ProvidedFlags{
					AuthProfile:        cmd.Flags().Changed("profile"),
					CloudflareZoneName: cmd.Flags().Changed("name"),
					CloudflareAccount:  cmd.Flags().Changed("account-id"),
					CloudflareZoneType: cmd.Flags().Changed("type"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	zoneUpdateCmd := &cobra.Command{
		Use:   domain.ToolsOperationUpdate,
		Short: "Update a Cloudflare zone",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:                   domain.CommandTools,
				ToolsProvider:             domain.ToolsProviderCloudflare,
				ToolsAction:               domain.ToolsActionCloudflareZones,
				ToolsOperation:            domain.ToolsOperationUpdate,
				AuthProfile:               authProfile,
				CloudflareZoneID:          zoneID,
				CloudflareZoneType:        zoneType,
				CloudflareZonePausedInput: zonePaused,
				Provided: domain.ProvidedFlags{
					AuthProfile:        cmd.Flags().Changed("profile"),
					CloudflareZoneID:   cmd.Flags().Changed("zone-id"),
					CloudflareZoneType: cmd.Flags().Changed("type"),
					CloudflarePaused:   cmd.Flags().Changed("paused"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	zoneDeleteCmd := &cobra.Command{
		Use:   domain.ToolsOperationDelete,
		Short: "Delete a Cloudflare zone",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:          domain.CommandTools,
				ToolsProvider:    domain.ToolsProviderCloudflare,
				ToolsAction:      domain.ToolsActionCloudflareZones,
				ToolsOperation:   domain.ToolsOperationDelete,
				AuthProfile:      authProfile,
				CloudflareZoneID: zoneID,
				Provided: domain.ProvidedFlags{
					AuthProfile:      cmd.Flags().Changed("profile"),
					CloudflareZoneID: cmd.Flags().Changed("zone-id"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	tunnelsCmd := &cobra.Command{
		Use:   domain.ToolsActionCloudflareTunnels,
		Short: "Manage Cloudflare Zero Trust tunnels (list/create/get/delete/route)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:       domain.CommandTools,
				ToolsProvider: domain.ToolsProviderCloudflare,
				ToolsAction:   domain.ToolsActionCloudflareTunnels,
				AuthProfile:   authProfile,
				Provided: domain.ProvidedFlags{
					AuthProfile: cmd.Flags().Changed("profile"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	tunnelListCmd := &cobra.Command{
		Use:   domain.ToolsOperationList,
		Short: "List Cloudflare Zero Trust tunnels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:             domain.CommandTools,
				ToolsProvider:       domain.ToolsProviderCloudflare,
				ToolsAction:         domain.ToolsActionCloudflareTunnels,
				ToolsOperation:      domain.ToolsOperationList,
				AuthProfile:         authProfile,
				CloudflareAccountID: zoneAccountID,
				Provided: domain.ProvidedFlags{
					AuthProfile:       cmd.Flags().Changed("profile"),
					CloudflareAccount: cmd.Flags().Changed("account-id"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	tunnelCreateCmd := &cobra.Command{
		Use:   domain.ToolsOperationCreate,
		Short: "Create a Cloudflare Zero Trust tunnel",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:              domain.CommandTools,
				ToolsProvider:        domain.ToolsProviderCloudflare,
				ToolsAction:          domain.ToolsActionCloudflareTunnels,
				ToolsOperation:       domain.ToolsOperationCreate,
				AuthProfile:          authProfile,
				CloudflareAccountID:  zoneAccountID,
				CloudflareTunnelName: tunnelName,
				Provided: domain.ProvidedFlags{
					AuthProfile:          cmd.Flags().Changed("profile"),
					CloudflareAccount:    cmd.Flags().Changed("account-id"),
					CloudflareTunnelName: cmd.Flags().Changed("name"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	tunnelGetCmd := &cobra.Command{
		Use:   domain.ToolsOperationGet,
		Short: "Get details for a Cloudflare Zero Trust tunnel",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:             domain.CommandTools,
				ToolsProvider:       domain.ToolsProviderCloudflare,
				ToolsAction:         domain.ToolsActionCloudflareTunnels,
				ToolsOperation:      domain.ToolsOperationGet,
				AuthProfile:         authProfile,
				CloudflareAccountID: zoneAccountID,
				CloudflareTunnelID:  tunnelID,
				Provided: domain.ProvidedFlags{
					AuthProfile:        cmd.Flags().Changed("profile"),
					CloudflareAccount:  cmd.Flags().Changed("account-id"),
					CloudflareTunnelID: cmd.Flags().Changed("tunnel-id"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	tunnelDeleteCmd := &cobra.Command{
		Use:   domain.ToolsOperationDelete,
		Short: "Delete a Cloudflare Zero Trust tunnel",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:             domain.CommandTools,
				ToolsProvider:       domain.ToolsProviderCloudflare,
				ToolsAction:         domain.ToolsActionCloudflareTunnels,
				ToolsOperation:      domain.ToolsOperationDelete,
				AuthProfile:         authProfile,
				CloudflareAccountID: zoneAccountID,
				CloudflareTunnelID:  tunnelID,
				Provided: domain.ProvidedFlags{
					AuthProfile:        cmd.Flags().Changed("profile"),
					CloudflareAccount:  cmd.Flags().Changed("account-id"),
					CloudflareTunnelID: cmd.Flags().Changed("tunnel-id"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	tunnelRouteCmd := &cobra.Command{
		Use:   domain.ToolsOperationRoute,
		Short: "Route a domain/subdomain to a Cloudflare Zero Trust tunnel (updates ingress & creates DNS CNAME)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := domain.Request{
				Command:             domain.CommandTools,
				ToolsProvider:       domain.ToolsProviderCloudflare,
				ToolsAction:         domain.ToolsActionCloudflareTunnels,
				ToolsOperation:      domain.ToolsOperationRoute,
				AuthProfile:         authProfile,
				CloudflareAccountID: zoneAccountID,
				CloudflareTunnelID:  tunnelID,
				CloudflareHostname:  hostname,
				CloudflareService:   serviceURL,
				CloudflareZoneID:    zoneID,
				Provided: domain.ProvidedFlags{
					AuthProfile:        cmd.Flags().Changed("profile"),
					CloudflareAccount:  cmd.Flags().Changed("account-id"),
					CloudflareTunnelID: cmd.Flags().Changed("tunnel-id"),
					CloudflareHostname: cmd.Flags().Changed("hostname"),
					CloudflareService:  cmd.Flags().Changed("service"),
					CloudflareZoneID:   cmd.Flags().Changed("zone-id"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	cloudflareCmd.PersistentFlags().StringVar(&authProfile, "profile", "", "Cloudflare auth profile name managed by civa auth cloudflare")
	zoneCreateCmd.Flags().StringVar(&zoneName, "name", "", "Cloudflare zone name (example.com)")
	zoneCreateCmd.Flags().StringVar(&zoneAccountID, "account-id", "", "Cloudflare account ID for zone creation")
	zoneCreateCmd.Flags().StringVar(&zoneType, "type", "", "Cloudflare zone type: full, partial, secondary, internal")
	zoneUpdateCmd.Flags().StringVar(&zoneID, "zone-id", "", "Cloudflare zone ID to update")
	zoneUpdateCmd.Flags().StringVar(&zoneType, "type", "", "Cloudflare zone type: full, partial, secondary, internal")
	zoneUpdateCmd.Flags().StringVar(&zonePaused, "paused", "", "Pause state to set: true or false")
	zoneDeleteCmd.Flags().StringVar(&zoneID, "zone-id", "", "Cloudflare zone ID to delete")

	tunnelListCmd.Flags().StringVar(&zoneAccountID, "account-id", "", "Cloudflare account ID")
	tunnelCreateCmd.Flags().StringVar(&zoneAccountID, "account-id", "", "Cloudflare account ID")
	tunnelCreateCmd.Flags().StringVar(&tunnelName, "name", "", "Cloudflare tunnel name")
	tunnelGetCmd.Flags().StringVar(&zoneAccountID, "account-id", "", "Cloudflare account ID")
	tunnelGetCmd.Flags().StringVar(&tunnelID, "tunnel-id", "", "Cloudflare tunnel ID")
	tunnelDeleteCmd.Flags().StringVar(&zoneAccountID, "account-id", "", "Cloudflare account ID")
	tunnelDeleteCmd.Flags().StringVar(&tunnelID, "tunnel-id", "", "Cloudflare tunnel ID")

	tunnelRouteCmd.Flags().StringVar(&zoneAccountID, "account-id", "", "Cloudflare account ID")
	tunnelRouteCmd.Flags().StringVar(&tunnelID, "tunnel-id", "", "Cloudflare tunnel ID")
	tunnelRouteCmd.Flags().StringVar(&hostname, "hostname", "", "Public hostname for the route (e.g. app.example.com)")
	tunnelRouteCmd.Flags().StringVar(&serviceURL, "service", "", "Target origin service URL (e.g. http://localhost:8080)")
	tunnelRouteCmd.Flags().StringVar(&zoneID, "zone-id", "", "Cloudflare zone ID (optional, auto-detected from hostname if omitted)")

	zonesCmd.AddCommand(zoneListCmd, zoneCreateCmd, zoneUpdateCmd, zoneDeleteCmd)
	tunnelsCmd.AddCommand(tunnelListCmd, tunnelCreateCmd, tunnelGetCmd, tunnelDeleteCmd, tunnelRouteCmd)

	cloudflareCmd.AddCommand(zonesCmd, tunnelsCmd)
	toolsCmd.AddCommand(cloudflareCmd)
	return toolsCmd
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

func (r *Root) newStartCommand(globals *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Run beginner wizard for setup or plan init",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			wizardReq, handled, err := r.beginnerWizardRequest(cmd, globals)
			if err != nil {
				if errors.Is(err, errWizardCancelled) {
					return nil
				}
				return err
			}
			if !handled {
				req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandHelp, HelpTarget: "start"})
				return r.executor.Execute(req)
			}
			if wizardReq.Command == "" {
				return nil
			}
			return r.executor.Execute(wizardReq)
		},
	}
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

	cmd.Flags().StringVar(&flags.sshUser, "ssh-user", "", "SSH user used to connect to every target server when hosts omit an explicit user")
	cmd.Flags().IntVar(&flags.sshPort, "ssh-port", 0, "SSH port used to connect to every target server when hosts omit an explicit port")
	cmd.Flags().StringVar(&flags.sshPassword, "ssh-password", "", "SSH password used by civa setup")
	cmd.Flags().StringVar(&flags.sshPasswordSecret, "ssh-password-secret", "", "Secret name in civa secret store for SSH password")
	cmd.Flags().StringVar(&flags.sshPublicKey, "ssh-public-key", "~/.ssh/id_ed25519.pub", "Local public key path that will be installed for the deploy user")
	cmd.Flags().StringArrayVar(&flags.servers, "server", nil, "Add a target server as [user@]addr[,hostname][,port]; user overrides --ssh-user, hostname and SSH port are optional")

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

func (r *Root) newPlaybookCommand(globals *globalFlags) *cobra.Command {
	var runPlanFile string
	var runPlaybookName string
	var runPlaybookFile string
	var addPlaybookFile string

	playbookCmd := &cobra.Command{
		Use:   string(domain.CommandPlaybook),
		Short: "Run and manage custom user playbooks",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandPlaybook)})
		},
	}

	runCmd := &cobra.Command{
		Use:   domain.PlaybookActionRun + " [plan-name]",
		Short: "Run a custom playbook against an existing generated plan context",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:        domain.CommandPlaybook,
				PlaybookAction: domain.PlaybookActionRun,
				PlanInputFile:  runPlanFile,
				PlaybookName:   runPlaybookName,
				PlaybookFile:   runPlaybookFile,
				Provided: domain.ProvidedFlags{
					PlanInputFile: cmd.Flags().Changed("plan-file"),
					PlaybookName:  cmd.Flags().Changed("name"),
					PlaybookFile:  cmd.Flags().Changed("file"),
				},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	runCmd.Flags().StringVar(&runPlanFile, "plan-file", "", "Existing plan file override used by playbook run")
	runCmd.Flags().StringVar(&runPlaybookName, "name", "", "Managed playbook name stored under ~/.civa/playbooks")
	runCmd.Flags().StringVar(&runPlaybookFile, "file", "", "Local custom playbook file path (.yml/.yaml)")

	addCmd := &cobra.Command{
		Use:   domain.PlaybookActionAdd + " <name>",
		Short: "Add or update a managed custom playbook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:        domain.CommandPlaybook,
				PlaybookAction: domain.PlaybookActionAdd,
				PlaybookName:   args[0],
				PlaybookFile:   addPlaybookFile,
				Provided: domain.ProvidedFlags{
					PlaybookName: true,
					PlaybookFile: cmd.Flags().Changed("file"),
				},
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	addCmd.Flags().StringVar(&addPlaybookFile, "file", "", "Local custom playbook file path to register (.yml/.yaml)")

	listCmd := &cobra.Command{
		Use:   domain.PlaybookActionList,
		Short: "List managed custom playbooks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			req := r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandPlaybook, PlaybookAction: domain.PlaybookActionList})
			return r.executor.Execute(req)
		},
	}

	removeCmd := &cobra.Command{
		Use:   domain.PlaybookActionRemove + " [name]",
		Short: "Remove a managed custom playbook",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{Command: domain.CommandPlaybook, PlaybookAction: domain.PlaybookActionRemove}
			if len(args) == 1 {
				req.PlaybookName = args[0]
				req.Provided.PlaybookName = true
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}

	playbookCmd.AddCommand(runCmd, addCmd, listCmd, removeCmd)
	return playbookCmd
}

func (r *Root) newBootstrapCommand(globals *globalFlags) *cobra.Command {
	bootstrapCmd := &cobra.Command{
		Use:   string(domain.CommandBootstrap),
		Short: "Simplified onboarding: setup, doctor, and config",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandBootstrap)})
		},
	}

	bootstrapCmd.AddCommand(
		r.newSetupCommand(globals),
		r.newDoctorCommand(globals),
		r.newConfigCommand(globals),
	)

	return bootstrapCmd
}

func (r *Root) newDeployCommand(globals *globalFlags) *cobra.Command {
	var runPlanFile string
	var runPlaybookName string
	var runPlaybookFile string

	deployCmd := &cobra.Command{
		Use:   string(domain.CommandDeploy),
		Short: "Simplified delivery flow: plan, apply, and custom playbook run",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandDeploy)})
		},
	}

	runCmd := &cobra.Command{
		Use:   "run [plan-name]",
		Short: "Run a managed/local custom playbook with existing plan artifacts",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := domain.Request{
				Command:        domain.CommandPlaybook,
				PlaybookAction: domain.PlaybookActionRun,
				PlanInputFile:  runPlanFile,
				PlaybookName:   runPlaybookName,
				PlaybookFile:   runPlaybookFile,
				Provided: domain.ProvidedFlags{
					PlanInputFile: cmd.Flags().Changed("plan-file"),
					PlaybookName:  cmd.Flags().Changed("name"),
					PlaybookFile:  cmd.Flags().Changed("file"),
				},
			}
			if len(args) == 1 {
				req.PlanName = args[0]
			}
			req = r.withGlobalFlags(cmd, globals, req)
			return r.executor.Execute(req)
		},
	}
	runCmd.Flags().StringVar(&runPlanFile, "plan-file", "", "Existing plan file override used by deploy run")
	runCmd.Flags().StringVar(&runPlaybookName, "name", "", "Managed playbook name stored under ~/.civa/playbooks")
	runCmd.Flags().StringVar(&runPlaybookFile, "file", "", "Local custom playbook file path (.yml/.yaml)")

	deployCmd.AddCommand(
		r.newPlanCommand(globals),
		r.newApplyCommand(globals),
		runCmd,
	)

	return deployCmd
}

func (r *Root) newOpsCommand(globals *globalFlags) *cobra.Command {
	opsCmd := &cobra.Command{
		Use:   string(domain.CommandOps),
		Short: "Operational tools: playbooks, secrets, auth profiles, and provider tools",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return r.executor.Execute(domain.Request{Command: domain.CommandHelp, HelpTarget: string(domain.CommandOps)})
		},
	}

	opsCmd.AddCommand(
		r.newPlaybookCommand(globals),
		r.newSecretCommand(globals),
		r.newAuthCommand(globals),
		r.newToolsCommand(globals),
	)

	return opsCmd
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

func (r *Root) beginnerWizardRequest(cmd *cobra.Command, globals *globalFlags) (domain.Request, bool, error) {
	if !shouldLaunchBeginnerWizard(globals) {
		return domain.Request{}, false, nil
	}

	action, err := promptWizardActionFn()
	if err != nil {
		return domain.Request{}, true, err
	}

	switch action {
	case wizardActionSetup:
		return r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandSetup}), true, nil
	case wizardActionPlanInit:
		return r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandPlan, PlanAction: domain.PlanActionInit}), true, nil
	case wizardActionHelp, "":
		return r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandHelp}), true, nil
	case wizardActionExit:
		return domain.Request{}, true, nil
	default:
		return r.withGlobalFlags(cmd, globals, domain.Request{Command: domain.CommandHelp}), true, nil
	}
}

func shouldLaunchBeginnerWizard(globals *globalFlags) bool {
	if globals == nil || globals.nonInteractive {
		return false
	}
	stdinFD := int(stdinFdFn())
	stdoutFD := int(stdoutFdFn())
	return isTerminalFn(stdinFD) && isTerminalFn(stdoutFD)
}

func promptWizardAction() (string, error) {
	action := wizardActionSetup
	field := huh.NewSelect[string]().
		Title("Welcome to civa — let's get started").
		Description("Choose what you want to do. civa will guide you step by step. Press Enter to use the default option.").
		Options(
			huh.NewOption("Set up SSH access first (recommended, default)", wizardActionSetup),
			huh.NewOption("Create a deployment plan (safe preview)", wizardActionPlanInit),
			huh.NewOption("Show command help", wizardActionHelp),
			huh.NewOption("Exit civa", wizardActionExit),
		).
		Value(&action)

	if err := field.Run(); err != nil {
		return "", normalizeWizardPromptError(err)
	}

	return strings.TrimSpace(action), nil
}

func normalizeWizardPromptError(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "interrupt") || strings.Contains(message, "ctrl+c") || strings.Contains(message, "user aborted") {
		return errWizardCancelled
	}
	return fmt.Errorf("beginner wizard: %w", err)
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
	parent := cmd.Parent()
	if parent != nil {
		parentName := parent.Name()

		if parentName == string(domain.CommandAuth) && name == domain.AuthProviderCloudflare {
			return helpTargetAuthCF
		}
		if parentName == string(domain.CommandTools) && name == domain.ToolsProviderCloudflare {
			return helpTargetToolsCF
		}
		if parentName == domain.AuthProviderCloudflare {
			if grandParent := parent.Parent(); grandParent != nil && grandParent.Name() == string(domain.CommandAuth) {
				if name == domain.AuthActionLogin || name == domain.AuthActionSet || name == domain.AuthActionGet || name == domain.AuthActionList || name == domain.AuthActionRemove || name == string(domain.CommandHelp) {
					return helpTargetAuthCF
				}
			}
		}
		if parentName == domain.ToolsProviderCloudflare {
			if grandParent := parent.Parent(); grandParent != nil && grandParent.Name() == string(domain.CommandTools) {
				if name == domain.ToolsActionCloudflareTunnels {
					return helpTargetToolsCFTunnels
				}
				if name == domain.ToolsActionCloudflareZones || name == string(domain.CommandHelp) {
					return helpTargetToolsCF
				}
			}
		}
		if parentName == domain.ToolsActionCloudflareZones {
			if grandParent := parent.Parent(); grandParent != nil && grandParent.Name() == domain.ToolsProviderCloudflare {
				if greatGrandParent := grandParent.Parent(); greatGrandParent != nil && greatGrandParent.Name() == string(domain.CommandTools) {
					if name == domain.ToolsOperationList || name == domain.ToolsOperationCreate || name == domain.ToolsOperationUpdate || name == domain.ToolsOperationDelete || name == string(domain.CommandHelp) {
						return helpTargetToolsCF
					}
				}
			}
		}
		if parentName == domain.ToolsActionCloudflareTunnels {
			if grandParent := parent.Parent(); grandParent != nil && grandParent.Name() == domain.ToolsProviderCloudflare {
				if greatGrandParent := grandParent.Parent(); greatGrandParent != nil && greatGrandParent.Name() == string(domain.CommandTools) {
					if name == domain.ToolsOperationList || name == domain.ToolsOperationCreate || name == domain.ToolsOperationGet || name == domain.ToolsOperationDelete || name == domain.ToolsOperationRoute || name == string(domain.CommandHelp) {
						return helpTargetToolsCFTunnels
					}
				}
			}
		}

		switch parentName {
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
					return configProviderHelpTarget(parentName)
				}
			}
		case string(domain.CommandApply):
			if name == domain.ApplyActionReview || name == domain.ApplyActionDrift || name == domain.ApplyActionRollback {
				return string(domain.CommandApply)
			}
		case string(domain.CommandBootstrap):
			// Keep subcommand help specific under bootstrap wrappers.
			// Example: `civa bootstrap setup --help` should show setup help.
		case string(domain.CommandDeploy):
			if name == "run" {
				return helpTargetDeployRun
			}
		case string(domain.CommandPlaybook):
			if name == domain.PlaybookActionRun || name == domain.PlaybookActionAdd || name == domain.PlaybookActionList || name == domain.PlaybookActionRemove {
				return string(domain.CommandPlaybook)
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
