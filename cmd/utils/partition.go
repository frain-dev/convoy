package utils

import (
	"fmt"
	"github.com/spf13/cobra"
)

func AddPartitionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "partition",
		Short: "runs partition commands",
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Running partition command...")
			return nil
		},
	}

	return cmd
}

func init() {
	utilsCmd.AddCommand(AddPartitionCommand())
}
