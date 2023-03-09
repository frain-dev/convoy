package version

import (
	"github.com/spf13/cobra"
)

func AddVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:              "version",
		Short:            "Print the version",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			root.SetArgs([]string{"--version"})
			err := root.Execute()
			if err != nil {
				return err
			}

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
	}

	return cmd
}
