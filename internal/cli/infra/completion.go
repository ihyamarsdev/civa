package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var completionCommands = []string{
	commandApply,
	commandPlan,
	commandPreview,
	commandSetup,
	commandConfig,
	commandDoctor,
	commandUninstall,
	commandVersion,
	commandHelp,
	commandCompletion,
}

var planSubcommands = []string{planActionStart, planActionList, planActionRemove, commandHelp}
var configSubcommands = []string{configActionEdit, configActionList, configActionRemove, commandHelp}
var doctorSubcommands = []string{doctorActionFix, commandHelp}
var webServerValues = []string{webServerNone, webServerTraefik, webServerNginx, webServerCaddy}
var traefikChallengeValues = []string{"http", "dns"}
var componentCompletionValues = []string{"all", "1", "2", "3", "4", "5", "6", "7", "8", "system_update", "user_management", "ssh_hardening", "security_firewall", "system_config", "dependencies", "containerization", "web_server", "traefik", "nginx", "caddy"}

func runCompletionCommand(args []string) error {
	if len(args) == 0 || args[0] == commandHelp || args[0] == "--help" || args[0] == "-h" {
		printCommandUsage(commandCompletion)
		return nil
	}

	shell := args[0]
	var script string
	switch shell {
	case "bash":
		script = bashCompletionScript()
	case "zsh":
		script = zshCompletionScript()
	case "fish":
		script = fishCompletionScript()
	default:
		return fmt.Errorf("unsupported completion shell: %s", shell)
	}

	fmt.Print(script)
	return nil
}

func runHiddenCompletion(args []string) error {
	for _, suggestion := range completionSuggestions(args) {
		fmt.Println(suggestion)
	}
	return nil
}

func completionSuggestions(words []string) []string {
	if len(words) == 0 {
		return completionCommands
	}

	if !contains(completionCommands, words[0]) {
		return filterByPrefix(completionCommands, words[0])
	}

	current := ""
	if len(words) > 0 {
		current = words[len(words)-1]
	}
	prev := ""
	if len(words) > 1 {
		prev = words[len(words)-2]
	}

	if valueSuggestions, ok := completionValuesForFlag(prev); ok {
		return filterByPrefix(valueSuggestions, current)
	}

	switch words[0] {
	case commandPlan:
		return completePlan(words)
	case commandDoctor:
		return completeDoctor(words)
	case commandPreview:
		return completePreview(words)
	case commandApply:
		return completeApply(words)
	case commandSetup:
		return completeSetup(words)
	case commandConfig:
		return completeConfig(words)
	case commandCompletion:
		return completeCompletionCommand(words)
	default:
		return completeCommonFlags(words, current)
	}
}

func completeDoctor(words []string) []string {
	current := words[len(words)-1]
	if len(words) == 1 {
		return doctorSubcommands
	}

	if len(words) == 2 && !contains(doctorSubcommands, words[1]) {
		return filterByPrefix(doctorSubcommands, current)
	}

	return filterByPrefix([]string{"--help"}, current)
}

func completePlan(words []string) []string {
	if len(words) == 1 {
		return planSubcommands
	}

	if len(words) == 2 && !contains(planSubcommands, words[1]) {
		return filterByPrefix(planSubcommands, words[1])
	}

	subcommand := words[1]
	current := words[len(words)-1]

	switch subcommand {
	case planActionStart:
		return completePlanStart(words, current)
	case planActionList:
		return filterByPrefix([]string{"--help"}, current)
	case planActionRemove:
		return completePlanRemove(words, current)
	default:
		return nil
	}
}

func completePlanStart(words []string, current string) []string {
	flagSuggestions := []string{"--help", "--non-interactive", "--ssh-user", "--ssh-port", "--web-server", "--ssh-private-key", "--ssh-public-key", "--deployer-user", "--timezone", "--components", "--server", "--traefik-email", "--traefik-challenge", "--traefik-dns-provider", "--output"}
	if strings.HasPrefix(current, "-") || previousWordExpectsValue(words) {
		return filterByPrefix(flagSuggestions, current)
	}
	return flagSuggestions
}

func completeSetup(words []string) []string {
	current := words[len(words)-1]
	flags := []string{"--help", "--non-interactive", "--ssh-user", "--ssh-port", "--ssh-password", "--ssh-public-key", "--server"}
	if len(words) == 1 {
		return flags
	}
	if strings.HasPrefix(current, "-") || previousWordExpectsValue(words) {
		return filterByPrefix(flags, current)
	}
	return flags
}

func completeConfig(words []string) []string {
	current := words[len(words)-1]
	flags := []string{"--help", "--non-interactive"}
	removeValues := []string{webServerNginx, webServerCaddy, configProfileAll}
	if len(words) == 1 {
		suggestions := append([]string{}, configSubcommands...)
		suggestions = append(suggestions, generatedPlanNames("")...)
		suggestions = append(suggestions, flags...)
		return suggestions
	}

	if len(words) == 2 && !contains(configSubcommands, words[1]) && !strings.HasPrefix(words[1], "-") {
		plans := append(configSubcommands, generatedPlanNames(current)...)
		return filterByPrefix(plans, current)
	}

	action := words[1]
	if !contains(configSubcommands, action) {
		action = configActionEdit
	}

	if strings.HasPrefix(current, "-") || previousWordExpectsValue(words) {
		return filterByPrefix(flags, current)
	}

	switch action {
	case configActionList:
		return filterByPrefix(flags, current)
	case configActionRemove:
		if len(words) <= 2 {
			return append(removeValues, flags...)
		}
		if len(words) == 3 && !strings.HasPrefix(words[2], "-") {
			return filterByPrefix(removeValues, current)
		}
		return filterByPrefix(flags, current)
	case configActionEdit:
		fallthrough
	default:
		return append(generatedPlanNames(current), flags...)
	}
}

func completePlanRemove(words []string, current string) []string {
	if len(words) <= 2 {
		return generatedPlanNames(current)
	}
	if len(words) == 3 && !strings.HasPrefix(words[2], "-") {
		return filterByPrefix(generatedPlanNames(""), current)
	}
	return filterByPrefix([]string{"--yes", "--help"}, current)
}

func completePreview(words []string) []string {
	current := words[len(words)-1]
	if len(words) == 1 {
		return append(generatedPlanNames(""), "--plan-file", "--help")
	}
	if strings.HasPrefix(current, "-") || previousWordExpectsValue(words) {
		return filterByPrefix([]string{"--plan-file", "--help"}, current)
	}
	return generatedPlanNames(current)
}

func completeApply(words []string) []string {
	current := words[len(words)-1]
	if len(words) == 2 && words[1] != applyActionReview && !strings.HasPrefix(current, "-") {
		suggestions := append([]string{applyActionReview}, generatedPlanNames(current)...)
		return filterByPrefix(suggestions, current)
	}

	if len(words) > 1 && words[1] == applyActionReview {
		reviewFlags := []string{"--plan-file", "--non-interactive", "--help"}
		if len(words) == 2 {
			return append(generatedPlanNames(""), reviewFlags...)
		}
		if strings.HasPrefix(current, "-") || previousWordExpectsValue(words) {
			return filterByPrefix(reviewFlags, current)
		}
		return generatedPlanNames(current)
	}

	if len(words) == 1 {
		return append([]string{applyActionReview}, append(generatedPlanNames(""), "--plan-file", "--yes", "--non-interactive", "--help")...)
	}
	if strings.HasPrefix(current, "-") || previousWordExpectsValue(words) {
		return filterByPrefix([]string{"--plan-file", "--yes", "--non-interactive", "--help"}, current)
	}
	return generatedPlanNames(current)
}

func completeCompletionCommand(words []string) []string {
	current := words[len(words)-1]
	return filterByPrefix([]string{"bash", "zsh", "fish", "help"}, current)
}

func completeCommonFlags(words []string, current string) []string {
	flags := []string{"--help"}
	if words[0] == commandUninstall {
		flags = []string{"--yes", "--help"}
	}
	return filterByPrefix(flags, current)
}

func completionValuesForFlag(flag string) ([]string, bool) {
	switch flag {
	case "--web-server":
		return webServerValues, true
	case "--traefik-challenge":
		return traefikChallengeValues, true
	case "--components":
		return componentCompletionValues, true
	default:
		return nil, false
	}
}

func generatedPlanNames(prefix string) []string {
	entries, err := os.ReadDir(runRootDirectoryPath())
	if err != nil {
		return nil
	}

	plans := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(runRootDirectoryPath(), entry.Name(), "plan.md")); err == nil {
			plans = append(plans, entry.Name())
		}
	}
	sort.Strings(plans)
	return filterByPrefix(plans, prefix)
}

func previousWordExpectsValue(words []string) bool {
	if len(words) < 2 {
		return false
	}
	_, expectsValue := completionValuesForFlag(words[len(words)-2])
	if expectsValue {
		return true
	}

	switch words[len(words)-2] {
	case "--plan-file", "--ssh-private-key", "--ssh-public-key", "--output", "--ssh-user", "--ssh-port", "--ssh-password", "--deployer-user", "--timezone", "--server", "--traefik-email", "--traefik-dns-provider":
		return true
	default:
		return false
	}
}

func filterByPrefix(values []string, prefix string) []string {
	if prefix == "" {
		return values
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func bashCompletionScript() string {
	return `# bash completion for civa
_civa_completion() {
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"

  case "$prev" in
    --plan-file|--ssh-private-key|--ssh-public-key|--output)
      compopt -o default
      return 0
      ;;
  esac

  local suggestions
  suggestions=$(civa __complete "${COMP_WORDS[@]:1}")
  COMPREPLY=($(compgen -W "$suggestions" -- "$cur"))
}

complete -F _civa_completion civa
`
}

func zshCompletionScript() string {
	return `#compdef civa
_civa_completion() {
  local prev
  prev=${words[CURRENT-1]}

  case "$prev" in
    --plan-file|--ssh-private-key|--ssh-public-key|--output)
      _files
      return
      ;;
  esac

  local -a suggestions
  suggestions=(${(f)"$(civa __complete ${words[2,-1]})"})
  _describe 'values' suggestions
}

compdef _civa_completion civa
`
}

func fishCompletionScript() string {
	return `function __civa_complete
    civa __complete (commandline -opc)[2..-1]
end

function __civa_complete_path_flag
    set prev (commandline -opc)[-1]
    contains -- $prev --plan-file --ssh-private-key --ssh-public-key --output
end

complete -c civa -f -n '__civa_complete_path_flag' -a '(__fish_complete_path)'
complete -c civa -f -n 'not __civa_complete_path_flag' -a '(__civa_complete)'
`
}
