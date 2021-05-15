package main

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/util"
	"github.com/spf13/cobra"
)

func addApplicationCommnand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "app",
		Aliases: []string{"application", "apps"},
		Short:   "Manage applications",
	}

	cmd.AddCommand(createApplication(a))

	return cmd
}

func createApplication(a *app) *cobra.Command {
	var name string
	var orgID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an application",
		RunE: func(cmd *cobra.Command, args []string) error {

			if util.IsStringEmpty(name) {
				return errors.New("please provide application name")
			}

			if util.IsStringEmpty(orgID) {
				return errors.New("please provide the org ID")
			}

			id, err := uuid.Parse(orgID)
			if err != nil {
				return fmt.Errorf("could not parse org ID..%w", err)
			}

			ctx, cancelFn := getCtx()
			defer cancelFn()

			org, err := a.database.FetchOrganisationByID(ctx, id)
			if err != nil {
				return err
			}

			app := &hookcamp.Application{
				Title: name,
				OrgID: org.ID,
			}

			ctx, cancelFn = getCtx()
			defer cancelFn()

			if err := a.database.CreateApplication(ctx, app); err != nil {
				return err
			}

			fmt.Printf("Your application was successfully created. ID = %s \n", app.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "The name of the application")
	cmd.Flags().StringVar(&orgID, "org", "", "The ID of the organisation that owns this application")

	return cmd
}
