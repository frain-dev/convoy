package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func addVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(subomi): this hasn't changed in a while
			fmt.Println("0.1.0")
			return nil
		},
	}

	return cmd
}
