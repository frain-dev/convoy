package main

import (
	"os"
	"strconv"

	"github.com/hookstack/hookstack"
	"github.com/hookstack/hookstack/config"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func addOrganisationCommnad(a *hookstack.App) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "org",
		Short: "Manage organisations",
	}

	cmd.AddCommand(listOrganisationCommand(a))
	return cmd
}

func listOrganisationCommand(a *hookstack.App) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all organisations currently known",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			if err := cfg.Organisation.FetchMode.Validate(); err != nil {
				return err
			}

			orgs, err := a.OrgLoader.LoadOrganisations()
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Counter", "ID", "Name", "Token"})

			for v, org := range orgs {
				table.Append([]string{strconv.Itoa(v + 1), org.ID, org.Name, org.Token.String()})
			}

			table.Render()
			return nil
		},
	}

	return cmd
}
