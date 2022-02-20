package main

import (
	"github.com/spf13/cobra"
)

func addVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:              "version",
		Short:            "Print the version",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {},
		Run: func(cmd *cobra.Command, args []string) {
			root := cmd.Root()
			root.SetArgs([]string{"--version"})
			root.Execute()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
	}

	return cmd
}
