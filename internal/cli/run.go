package cli

import (
	"civa/internal/cli/app"
	"civa/internal/cli/cmd"
	"civa/internal/cli/infra"
)

func Run(args []string) error {
	runner := infra.NewLegacyRunner()
	service := app.NewService(runner)
	root := cmd.NewRoot(service)
	return root.Run(args)
}
