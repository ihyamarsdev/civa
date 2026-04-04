package domain

type Command string

const (
	CommandHelp             Command = "help"
	CommandVersion          Command = "version"
	CommandCompletion       Command = "completion"
	CommandCompleteInternal Command = "__complete"
	CommandDoctor           Command = "doctor"
	CommandSetup            Command = "setup"
	CommandConfig           Command = "config"
	CommandUninstall        Command = "uninstall"
	CommandPlan             Command = "plan"
	CommandPreview          Command = "preview"
	CommandApply            Command = "apply"
)

const (
	PlanActionStart  = "start"
	PlanActionList   = "list"
	PlanActionRemove = "remove"
)

const (
	ConfigActionEdit   = "edit"
	ConfigActionList   = "list"
	ConfigActionRemove = "remove"
)

const (
	ApplyActionExecute = "execute"
	ApplyActionReview  = "review"
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
}

type Request struct {
	Command    Command
	HelpTarget string

	PlanAction   string
	ConfigAction string
	ApplyAction  string
	DoctorAction string

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
	Servers            []string

	Provided ProvidedFlags
}

type Executor interface {
	Execute(req Request) error
}
