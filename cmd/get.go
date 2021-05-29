package main

import (
	"context"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func addGetComamnd(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get all kind of resources",
	}

	cmd.AddCommand(getOrganisations(a))
	cmd.AddCommand(getApplications(a))

	return cmd
}

func getApplications(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "applications",
		Short:   "Get Applications",
		Aliases: []string{"apps"},
		RunE: func(cmd *cobra.Command, args []string) error {

			apps, err := a.applicationRepo.LoadApplications(context.Background())
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			for _, app := range apps {
				table.Append([]string{app.UID, app.Title, time.Unix(app.CreatedAt, 0).String()})
			}

			table.Render()
			return nil
		},
	}

	return cmd
}

func getOrganisations(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "organisations",
		Short:   "Get organisations",
		Aliases: []string{"orgs"},
		RunE: func(cmd *cobra.Command, args []string) error {

			orgs, err := a.orgRepo.LoadOrganisations(context.Background())
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			for _, org := range orgs {
				table.Append([]string{org.UID, org.OrgName, time.Unix(org.CreatedAt, 0).String()})
			}

			table.Render()
			return nil
		},
	}

	return cmd
}
