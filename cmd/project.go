package main

import (
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

func addProjectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "projects",
		Short:             "List all your convoy projects",
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
			t.AppendHeader(table.Row{"Current Project", "ID", "Name"})

			for _, project := range c.Projects {
				var current string

				if project.Name == c.ActiveProject {
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
