package main

import (
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func addListAppsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "apps",
		Short:             "List all your convoy cli apps",
		PersistentPreRun:  func(cmd *cobra.Command, args []string) {},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
		RunE: func(cmd *cobra.Command, args []string) error {
			var host, apiKey string
			c, err := NewConfig(host, apiKey)
			if err != nil {
				return err
			}

			t := table.NewWriter()
			t.SetOutputMirror(os.Stdout)
			t.AppendHeader(table.Row{"Current Endpoint", "ID", "Name"})

			for _, project := range c.Endpoints {
				var current string

				if project.Name == c.ActiveEndpoint {
					current = "*"
				}

				t.AppendRow(table.Row{current, project.UID, project.Name})
			}

			t.Render()
			return nil
		},
	}

	return cmd
}
