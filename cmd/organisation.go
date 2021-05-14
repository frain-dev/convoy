package main

import (
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func addOrganisationCommnad(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "org",
		Short: "Manage organisations",
	}

	cmd.AddCommand(listOrganisationCommand(a))
	return cmd
}

func listOrganisationCommand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all organisations currently known",
		RunE: func(cmd *cobra.Command, args []string) error {

			orgs, err := a.database.LoadOrganisations()
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			for _, org := range orgs {
				table.Append([]string{org.ID.String(), org.Name, org.CreatedAt.String()})
			}

			table.Render()
			return nil
		},
	}

	return cmd
}
