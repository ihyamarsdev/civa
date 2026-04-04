package cli

import (
	"civa/internal/cli/app"
	"civa/internal/cli/cmd"
	"civa/internal/cli/infra"
	"strings"
)

func Run(args []string) error {
	args = normalizeLegacyCompatibleArgs(args)
	runner := infra.NewLegacyRunner()
	service := app.NewService(runner)
	root := cmd.NewRoot(service)
	return root.Run(args)
}

func normalizeLegacyCompatibleArgs(args []string) []string {
	if len(args) == 3 && args[0] == "plan" && strings.EqualFold(args[2], "list") && !strings.HasPrefix(args[1], "-") {
		return []string{"plan", "list", args[1]}
	}

	return args
}
