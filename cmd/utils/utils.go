package utils

import (
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/spf13/cobra"
)

type Utils struct {
	cmd *cobra.Command
	app *cli.App
}

func NewUtils(a *cli.App) *Utils {
	u := &Utils{
		cmd: &cobra.Command{
			Use:   "utils",
			Short: "runs utility commands",
			Annotations: map[string]string{
				"CheckMigration":  "true",
				"ShouldBootstrap": "false",
			},
		},
		app: a,
	}

	u.cmd.AddCommand(AddPartitionCommand(a))
	u.cmd.AddCommand(AddUnPartitionCommand(a))

	return u
}

func AddUtilsCommand(a *cli.App) *cobra.Command {
	u := NewUtils(a)
	return u.cmd
}
