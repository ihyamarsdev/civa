package domain

type Command string

const (
	CommandHelp             Command = "help"
	CommandVersion          Command = "version"
	CommandCompletion       Command = "completion"
	CommandCompleteInternal Command = "__complete"
	CommandDoctor           Command = "doctor"
	CommandSetup            Command = "setup"
	CommandTools            Command = "tools"
	CommandConfig           Command = "config"
	CommandUninstall        Command = "uninstall"
	CommandPlan             Command = "plan"
	CommandApply            Command = "apply"
	CommandSecret           Command = "secret"
)

const (
	ToolsProviderCloudflare = "cloudflare"
)

const (
	ToolsActionCloudflareZones = "zones"
)

const (
	PlanActionInit   = "init"
	PlanActionReview = "review"
	PlanActionEdit   = "edit"
	PlanActionList   = "list"
	PlanActionRemove = "remove"
)

const (
	ConfigActionInit   = "init"
	ConfigActionEdit   = "edit"
	ConfigActionList   = "list"
	ConfigActionRemove = "remove"
)

const (
	ApplyActionExecute  = "execute"
	ApplyActionReview   = "review"
	ApplyActionDrift    = "drift"
	ApplyActionRollback = "rollback"
)

const (
	SecretActionSet    = "set"
	SecretActionList   = "list"
	SecretActionRemove = "remove"
)

const (
	DoctorActionCheck = "check"
	DoctorActionFix   = "fix"
)

type ProvidedFlags struct {
	SSHUser            bool
	SSHPort            bool
	SSHPassword        bool
	WebServer          bool
	SSHPrivateKey      bool
	SSHPublicKey       bool
	DeployUser         bool
	Timezone           bool
	Components         bool
	PlanInputFile      bool
	PlanFile           bool
	TraefikEmail       bool
	TraefikChallenge   bool
	TraefikDNSProvider bool
	Servers            bool
	NonInteractive     bool
	SSHPasswordSecret  bool
	SecretValue        bool
	SecretValueFile    bool
	CloudflareToken    bool
}

type Request struct {
	Command    Command
	HelpTarget string

	PlanAction   string
	ConfigAction string
	ApplyAction  string
	DoctorAction string
	SecretAction string
	ToolsAction  string

	CompletionArgs []string

	PlanName       string
	AssumeYes      bool
	NonInteractive bool

	SSHUser            string
	SSHPort            int
	SSHPassword        string
	WebServer          string
	SSHPrivateKey      string
	SSHPublicKey       string
	DeployUser         string
	Timezone           string
	TraefikEmail       string
	TraefikChallenge   string
	TraefikDNSProvider string
	ComponentsInput    string
	PlanInputFile      string
	PlanFile           string
	SecretName         string
	SecretValue        string
	SecretValueFile    string
	ToolsProvider      string
	CloudflareToken    string
	SSHPasswordSecret  string
	Servers            []string

	Provided ProvidedFlags
}

type Executor interface {
	Execute(req Request) error
}
