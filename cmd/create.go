package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/util"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func addCreateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a resource",
	}

	cmd.AddCommand(createMessageCommand(a))
	cmd.AddCommand(createOrganisationCommand(a))
	cmd.AddCommand(createApplicationCommand(a))

	return cmd
}

func createApplicationCommand(a *app) *cobra.Command {

	var orgID string

	cmd := &cobra.Command{
		Use:     "application",
		Aliases: []string{"app"},
		Short:   "Create an application",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) <= 0 {
				return errors.New("please provide the application name")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			appName := args[0]
			if util.IsStringEmpty(appName) {
				return errors.New("please provide your app name")
			}

			if util.IsStringEmpty(orgID) {
				return errors.New("please provide a valid Organisation ID")
			}

			id, err := uuid.Parse(orgID)
			if err != nil {
				return err
			}

			org, err := a.orgRepo.FetchOrganisationByID(context.Background(), id)
			if err != nil {
				return err
			}

			app := &hookcamp.Application{
				UID:       uuid.New().String(),
				OrgID:     org.UID,
				Title:     appName,
				CreatedAt: time.Now().Unix(),
				UpdatedAt: time.Now().Unix(),
				Endpoints: []hookcamp.Endpoint{},
			}

			err = a.applicationRepo.CreateApplication(context.Background(), app)
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Organisation", "Created at"})

			table.Append([]string{app.UID, app.Title, org.OrgName, time.Unix(app.CreatedAt, 0).String()})
			table.Render()

			return nil
		},
	}

	cmd.Flags().StringVar(&orgID, "org", "", "Organisation that owns this application")

	return cmd
}

func createOrganisationCommand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "organisation",
		Short: "Create an organisation",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) <= 0 {
				return errors.New("please provide the organisation name")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			name := args[0]

			if util.IsStringEmpty(name) {
				return errors.New("please provide a valid name")
			}

			org := &hookcamp.Organisation{
				UID:       uuid.New().String(),
				OrgName:   name,
				CreatedAt: time.Now().Unix(),
				UpdatedAt: time.Now().Unix(),
			}

			err := a.orgRepo.CreateOrganisation(context.Background(), org)
			if err != nil {
				return fmt.Errorf("could not create organisation... %w", err)
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			table.Append([]string{org.UID, org.OrgName, time.Unix(org.CreatedAt, 0).String()})
			table.Render()
			return nil
		},
	}

	return cmd
}

func createMessageCommand(a *app) *cobra.Command {
	var data string
	var appID string
	var filePath string
	var eventType string
	var publish bool

	cmd := &cobra.Command{
		Use:   "message",
		Short: "Create a message",
		RunE: func(cmd *cobra.Command, args []string) error {
			// var d json.RawMessage

			// if util.IsStringEmpty(eventType) {
			// 	return errors.New("please provide an event type")
			// }

			// if util.IsStringEmpty(data) && util.IsStringEmpty(filePath) {
			// 	return errors.New("please provide one of -f or -d")
			// }

			// if !util.IsStringEmpty(data) && !util.IsStringEmpty(filePath) {
			// 	return errors.New("please provide only one of -f or -d")
			// }

			// if !util.IsStringEmpty(data) {
			// 	d = json.RawMessage([]byte(data))
			// }

			// if !util.IsStringEmpty(filePath) {
			// 	f, err := os.Open(filePath)
			// 	if err != nil {
			// 		return fmt.Errorf("could not open file... %w", err)
			// 	}

			// 	defer f.Close()

			// 	if err := json.NewDecoder(f).Decode(&d); err != nil {
			// 		return err
			// 	}
			// }

			// id, err := uuid.Parse(appID)
			// if err != nil {
			// 	return fmt.Errorf("please provide a valid app ID.. %w", err)
			// }

			// ctx, cancelFn := getCtx()
			// defer cancelFn()

			// appData, err := a.applicationRepo.FindApplicationByID(ctx, id)
			// if err != nil {
			// 	return err
			// }

			// msg := &hookcamp.Message{
			// 	ID:        uuid.New(),
			// 	AppID:     appData.ID,
			// 	EventType: hookcamp.EventType(eventType),
			// 	Data:      hookcamp.JSONData(d),
			// 	Metadata: &hookcamp.MessageMetadata{
			// 		NumTrials: 0,
			// 	},
			// 	Status: hookcamp.ScheduledMessageStatus,
			// }

			// ctx, cancelFn = getCtx()
			// defer cancelFn()

			// if err := a.messageRepo.CreateMessage(ctx, msg); err != nil {
			// 	return fmt.Errorf("could not create message... %w", err)
			// }

			// fmt.Println("Your message has been created. And will be sent to available endpoints")
			return nil
		},
	}

	cmd.Flags().StringVarP(&data, "data", "d", "", "Raw JSON data that will be sent to the endpoints")
	cmd.Flags().StringVarP(&appID, "app", "a", "", "Application ID")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to file containing JSON data")
	cmd.Flags().StringVar(&eventType, "event", "", "Event type")
	cmd.Flags().BoolVar(&publish, "publish", false, `If true, it will send the data to the endpoints
attached to the application`)

	return cmd
}
