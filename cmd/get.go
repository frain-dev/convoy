package main

import (
	"context"
	"os"

	"github.com/frain-dev/convoy/datastore"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func addGetComamnd(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get all kind of resources",
	}

	cmd.AddCommand(getGroups(a))
	cmd.AddCommand(getApplications(a))

	return cmd
}

func getApplications(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "applications",
		Short:   "Get Applications",
		Aliases: []string{"apps"},
		RunE: func(cmd *cobra.Command, args []string) error {

			apps, _, err := a.applicationRepo.LoadApplicationsPaged(context.Background(), "", "", datastore.Pageable{
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

func getGroups(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "groups",
		Short:   "Get groups",
		Aliases: []string{"groups"},
		RunE: func(cmd *cobra.Command, args []string) error {

			f := &datastore.GroupFilter{}

			if len(args) > 0 {
				f.Names = []string{args[0]}
			}

			groups, err := a.groupRepo.LoadGroups(context.Background(), f)
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			for _, group := range groups {
				table.Append([]string{group.UID, group.Name, group.CreatedAt.Time().String()})
			}

			table.Render()
			return nil
		},
	}

	return cmd
}
