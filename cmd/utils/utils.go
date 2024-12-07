package utils

import (
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/spf13/cobra"
)

var utilsCmd = &cobra.Command{
	Use:   "utils",
	Short: "runs utility commands",
	Annotations: map[string]string{
		"CheckMigration":  "true",
		"ShouldBootstrap": "false",
	},
}

func AddUtilsCommand(app *cli.App) *cobra.Command {

	utilsCmd.AddCommand(AddInitEncryptionCommand(app))
	utilsCmd.AddCommand(AddRotateKeyCommand(app))
	utilsCmd.AddCommand(AddRevertEncryptionCommand(app))

	return utilsCmd
}
