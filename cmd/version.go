package main

import (
	"fmt"
	"strings"

	"github.com/frain-dev/convoy"
	"github.com/spf13/cobra"
)

func addVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := "0.1.0"

			f, err := convoy.ReadVersion()
			if err != nil {
				fmt.Println(v)
				return nil
			}
			v = strings.TrimSuffix(string(f), "\n")
			fmt.Println(v)
			return nil
		},
	}

	return cmd
}
