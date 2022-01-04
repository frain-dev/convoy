package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func addVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			var version string

			f, err := os.ReadFile("VERSION")
			if err != nil {
				version = "0.1.0"
			}
			version = strings.TrimSuffix(string(f), "\n")
			fmt.Println(version)
			return nil
		},
	}

	return cmd
}
