package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func addApplicationCommnand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "app",
		Aliases: []string{"application", "apps"},
		Short:   "Manage applications",
	}

	cmd.AddCommand(createApplication(a))
	cmd.AddCommand(listApplications(a))
	cmd.AddCommand(getApplicationCommand(a))

	return cmd
}

func listApplications(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all applications",
		RunE: func(cmd *cobra.Command, args []string) error {
			// ctx, cancelFn := getCtx()
			// defer cancelFn()

			// apps, err := a.applicationRepo.LoadApplications(ctx)
			// if err != nil {
			// 	return err
			// }

			// table := tablewriter.NewWriter(os.Stdout)
			// table.SetHeader([]string{"ID", "Name", "Org name", "Created at"})

			// for _, app := range apps {
			// 	table.Append([]string{app.UID.String(), app.Title, app.Organisation.OrgName, app.CreatedAt.String()})
			// }

			// table.Render()
			return nil
		},
	}

	return cmd
}

func createApplication(a *app) *cobra.Command {
	var name string
	var orgID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an application",
		RunE: func(cmd *cobra.Command, args []string) error {
			// if util.IsStringEmpty(name) {
			// 	return errors.New("please provide application name")
			// }

			// if util.IsStringEmpty(orgID) {
			// 	return errors.New("please provide the org ID")
			// }

			// id, err := uuid.Parse(orgID)
			// if err != nil {
			// 	return fmt.Errorf("could not parse org ID..%w", err)
			// }

			// ctx, cancelFn := getCtx()
			// defer cancelFn()

			// org, err := a.orgRepo.FetchOrganisationByID(ctx, id)
			// if err != nil {
			// 	return err
			// }

			// app := &hookcamp.Application{
			// 	Title: name,
			// 	OrgID: org.ID,
			// }

			// ctx, cancelFn = getCtx()
			// defer cancelFn()

			// if err := a.applicationRepo.CreateApplication(ctx, app); err != nil {
			// 	return err
			// }

			// fmt.Printf("Your application was successfully created. ID = %s \n", app.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "The name of the application")
	cmd.Flags().StringVar(&orgID, "org", "", "The ID of the organisation that owns this application")

	return cmd
}

func getApplicationCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get the details of an application",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires an ID argument")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// ID := args[0]

			// appID, err := uuid.Parse(ID)
			// if err != nil {
			// 	return fmt.Errorf("Please provide a valid ID..%w", err)
			// }

			// ctx, cancelFn := getCtx()
			// defer cancelFn()

			// app, err := a.applicationRepo.FindApplicationByID(ctx, appID)
			// if err != nil {
			// 	return fmt.Errorf("could not fetch app ID..%w", err)
			// }

			// table := tablewriter.NewWriter(os.Stdout)
			// table.SetHeader([]string{"ID", "Description"})

			// table.Append([]string{app.ID.String(), app.Title})

			// table.Render()
			return nil
		},
	}

	return cmd
}
