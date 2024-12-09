package utils

import (
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

func AddUtilsCommand() *cobra.Command {
	return utilsCmd
}
