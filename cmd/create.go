package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func addCreateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a resource",
	}

	cmd.AddCommand(createMessageCommand(a))
	cmd.AddCommand(createGroupCommand(a))
	cmd.AddCommand(createApplicationCommand(a))
	cmd.AddCommand(createEndpointCommand(a))

	return cmd
}

func createEndpointCommand(a *app) *cobra.Command {

	e := new(convoy.Endpoint)

	var appID string

	cmd := &cobra.Command{
		Use:   "endpoint",
		Short: "Create a new endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			if util.IsStringEmpty(e.Description) {
				return errors.New("please provide a description")
			}

			if util.IsStringEmpty(e.TargetURL) {
				return errors.New("please provide your target url")
			}

			s, err := util.CleanEndpoint(e.TargetURL)
			if err != nil {
				return err
			}

			e.TargetURL = s

			e.UID = uuid.New().String()

			ctx, cancelFn := getCtx()
			defer cancelFn()

			app, err := a.applicationRepo.FindApplicationByID(ctx, appID)
			if err != nil {
				return fmt.Errorf("could not fetch application from the database...%w", err)
			}

			app.Endpoints = append(app.Endpoints, *e)

			ctx, cancelFn = getCtx()
			defer cancelFn()

			err = a.applicationRepo.UpdateApplication(ctx, app)
			if err != nil {
				return fmt.Errorf("could not add endpoint...%w", err)
			}

			fmt.Println("Endpoint was successfully created")
			fmt.Println()

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Target URL", "Description"})

			table.Append([]string{e.UID, e.TargetURL, e.Description})

			table.Render()
			return nil
		},
	}

	cmd.Flags().StringVar(&e.Description, "description", "", "Description of this endpoint")
	cmd.Flags().StringVar(&e.TargetURL, "target", "", "The target url of this endpoint")
	cmd.Flags().StringVar(&appID, "app", "", "The app this endpoint belongs to")

	return cmd
}

func createApplicationCommand(a *app) *cobra.Command {

	var groupID string
	var appSecret string

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

			if util.IsStringEmpty(groupID) {
				return errors.New("please provide a valid Group ID")
			}

			group, err := a.groupRepo.FetchGroupByID(context.Background(), groupID)
			if err != nil {
				return err
			}

			if util.IsStringEmpty(appSecret) {
				appSecret, err = util.GenerateSecret()
				if err != nil {
					return fmt.Errorf("could not generate secret...%v", err)
				}
			}

			app := &convoy.Application{
				UID:            uuid.New().String(),
				GroupID:        group.UID,
				Title:          appName,
				CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				Endpoints:      []convoy.Endpoint{},
				DocumentStatus: convoy.ActiveDocumentStatus,
			}

			err = a.applicationRepo.CreateApplication(context.Background(), app)
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Group", "Created at"})

			table.Append([]string{app.UID, app.Title, group.Name, app.CreatedAt.Time().String()})
			table.Render()

			return nil
		},
	}

	cmd.Flags().StringVar(&groupID, "group", "", "Group that owns this application")
	cmd.Flags().StringVar(&appSecret, "secret", "", "Provide the secret for app endpoint(s). If blank, it will be automatically generated")

	return cmd
}

func createGroupCommand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "group",
		Short: "Create an group",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) <= 0 {
				return errors.New("please provide the group name")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			name := args[0]

			if util.IsStringEmpty(name) {
				return errors.New("please provide a valid name")
			}

			group := &convoy.Group{
				UID:            uuid.New().String(),
				Name:           name,
				CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				DocumentStatus: convoy.ActiveDocumentStatus,
			}

			err := a.groupRepo.CreateGroup(context.Background(), group)
			if err != nil {
				return fmt.Errorf("could not create group... %w", err)
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			table.Append([]string{group.UID, group.Name, group.CreatedAt.Time().String()})
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
			var d json.RawMessage

			if util.IsStringEmpty(eventType) {
				return errors.New("please provide an event type")
			}

			if util.IsStringEmpty(data) && util.IsStringEmpty(filePath) {
				return errors.New("please provide one of -f or -d")
			}

			if !util.IsStringEmpty(data) && !util.IsStringEmpty(filePath) {
				return errors.New("please provide only one of -f or -d")
			}

			if !util.IsStringEmpty(data) {
				if !util.IsJSON(data) {
					return errors.New("invalid json provided: " + data)
				}

				d = []byte(data)
			}

			if !util.IsStringEmpty(filePath) {
				f, err := os.Open(filePath)
				if err != nil {
					return fmt.Errorf("could not open file... %w", err)
				}

				defer func() {
					err := f.Close()
					if err != nil {
						log.Errorf("failed to close file - %+v", err)
					}
				}()

				if err := json.NewDecoder(f).Decode(&d); err != nil {
					return err
				}
			}

			id, err := uuid.Parse(appID)
			if err != nil {
				return fmt.Errorf("please provide a valid app ID.. %w", err)
			}

			ctx, cancelFn := getCtx()
			defer cancelFn()

			appData, err := a.applicationRepo.FindApplicationByID(ctx, id.String())
			if err != nil {
				return err
			}

			if len(appData.Endpoints) == 0 {
				return errors.New("app has no configured endpoints")
			}

			activeEndpoints := util.ParseMetadataFromActiveEndpoints(appData.Endpoints)
			if len(activeEndpoints) == 0 {
				return errors.New("app has no enabled endpoints")
			}

			log.Println("Event ", string(d))
			msg := &convoy.Event{
				UID: uuid.New().String(),
				AppMetadata: &convoy.AppMetadata{
					UID: appData.UID,
				},
				EventType: convoy.EventType(eventType),
				Data:      d,

				CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				DocumentStatus: convoy.ActiveDocumentStatus,
			}

			ctx, cancelFn = getCtx()
			defer cancelFn()

			if err := a.eventRepo.CreateEvent(ctx, msg); err != nil {
				return fmt.Errorf("could not create event... %w", err)
			}

			fmt.Println("Your event has been created. And will be sent to available endpoints")
			return nil
		},
	}

	cmd.Flags().StringVarP(&data, "data", "d", "", "Raw JSON data that will be sent to the endpoints")
	cmd.Flags().StringVarP(&appID, "app", "a", "", "Application ID")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to file containing JSON data")
	cmd.Flags().StringVarP(&eventType, "event", "e", "", "Event type")
	cmd.Flags().BoolVar(&publish, "publish", false, `If true, it will send the data to the endpoints
attached to the application`)

	return cmd
}
