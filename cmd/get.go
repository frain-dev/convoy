package main

import (
	"context"
	"os"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
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

			apps, _, err := a.applicationRepo.LoadApplicationsPaged(context.Background(), "", models.Pageable{
				Page:    0,
				PerPage: 50,
			})
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			for _, app := range apps {
				table.Append([]string{app.UID, app.Title, app.CreatedAt.Time().String()})
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
		Args: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			f := &convoy.OrganisationFilter{}

			if len(args) > 0 {
				f.Name = args[0]
			}

			orgs, err := a.orgRepo.LoadOrganisations(context.Background(), f)
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			for _, org := range orgs {
				table.Append([]string{org.UID, org.OrgName, org.CreatedAt.Time().String()})
			}

			table.Render()
			return nil
		},
	}

	return cmd
}
