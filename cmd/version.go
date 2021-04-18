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
			fmt.Println("0.0.1")
			return nil
		},
	}

	return cmd
}
