package domain

type Command string

const (
	CommandHelp             Command = "help"
	CommandVersion          Command = "version"
	CommandCompletion       Command = "completion"
	CommandCompleteInternal Command = "__complete"
	CommandDoctor           Command = "doctor"
	CommandSetup            Command = "setup"
	CommandAuth             Command = "auth"
	CommandTools            Command = "tools"
	CommandConfig           Command = "config"
	CommandUninstall        Command = "uninstall"
	CommandPlan             Command = "plan"
	CommandApply            Command = "apply"
	CommandPlaybook         Command = "playbook"
	CommandSecret           Command = "secret"
	CommandBootstrap        Command = "bootstrap"
	CommandDeploy           Command = "deploy"
	CommandOps              Command = "ops"
)

const (
	AuthProviderCloudflare = "cloudflare"
)

const (
	AuthActionSet    = "set"
	AuthActionGet    = "get"
	AuthActionList   = "list"
	AuthActionRemove = "remove"
	AuthActionLogin  = "login"
)

const (
	ToolsProviderCloudflare = "cloudflare"
)

const (
	ToolsActionCloudflareZones   = "zones"
	ToolsActionCloudflareTunnels = "tunnels"
)

const (
	ToolsOperationList   = "list"
	ToolsOperationCreate = "create"
	ToolsOperationUpdate = "update"
	ToolsOperationDelete = "delete"
	ToolsOperationGet    = "get"
	ToolsOperationRoute  = "route"
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
	PlaybookActionRun    = "run"
	PlaybookActionAdd    = "add"
	PlaybookActionList   = "list"
	PlaybookActionRemove = "remove"
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
	AuthToken          bool
	AuthProfile        bool
	CloudflareToken    bool
	ToolsOperation     bool
	CloudflareAccount  bool
	CloudflareZoneID   bool
	CloudflareZoneName bool
	CloudflareZoneType bool
	CloudflarePaused     bool
	CloudflareTunnelID   bool
	CloudflareTunnelName bool
	CloudflareHostname   bool
	CloudflareService    bool
	PlaybookName         bool
	PlaybookFile         bool
}

type Request struct {
	Command    Command
	HelpTarget string

	PlanAction     string
	ConfigAction   string
	ApplyAction    string
	PlaybookAction string
	DoctorAction   string
	SecretAction   string
	AuthAction     string
	ToolsAction    string
	ToolsOperation string

	CompletionArgs []string

	PlanName       string
	AssumeYes      bool
	NonInteractive bool

	SSHUser                   string
	SSHPort                   int
	SSHPassword               string
	WebServer                 string
	SSHPrivateKey             string
	SSHPublicKey              string
	DeployUser                string
	Timezone                  string
	TraefikEmail              string
	TraefikChallenge          string
	TraefikDNSProvider        string
	ComponentsInput           string
	PlanInputFile             string
	PlanFile                  string
	SecretName                string
	SecretValue               string
	SecretValueFile           string
	AuthProvider              string
	AuthProfile               string
	AuthToken                 string
	ToolsProvider             string
	CloudflareToken           string
	CloudflareAccountID       string
	CloudflareZoneID          string
	CloudflareZoneName        string
	CloudflareZoneType        string
	CloudflareZonePaused      bool
	CloudflareZonePausedInput string
	CloudflareTunnelID        string
	CloudflareTunnelName      string
	CloudflareHostname        string
	CloudflareService         string
	PlaybookName              string
	PlaybookFile              string
	SSHPasswordSecret         string
	Servers                   []string

	Provided ProvidedFlags
}

type Executor interface {
	Execute(req Request) error
}
