package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hookcamp/hookcamp/config"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/url"
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
	cmd.AddCommand(createEndpointCommand(a))

	return cmd
}

func createEndpointCommand(a *app) *cobra.Command {

	e := new(hookcamp.Endpoint)

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

			u, err := url.Parse(e.TargetURL)
			if err != nil {
				return fmt.Errorf("please provide a valid url...%w", err)
			}

			e.TargetURL = u.String()

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

	var orgID string
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

			if util.IsStringEmpty(orgID) {
				return errors.New("please provide a valid Organisation ID")
			}

			org, err := a.orgRepo.FetchOrganisationByID(context.Background(), orgID)
			if err != nil {
				return err
			}

			if util.IsStringEmpty(appSecret) {
				appSecret, err = util.GenerateSecret()
				if err != nil {
					return fmt.Errorf("could not generate secret...%v", err)
				}
			}

			app := &hookcamp.Application{
				UID:       uuid.New().String(),
				OrgID:     org.UID,
				Title:     appName,
				Secret:    appSecret,
				CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
				Endpoints: []hookcamp.Endpoint{},
			}

			err = a.applicationRepo.CreateApplication(context.Background(), app)
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Organisation", "Secret", "Created at"})

			table.Append([]string{app.UID, app.Title, org.OrgName, app.Secret, app.CreatedAt.Time().String()})
			table.Render()

			return nil
		},
	}

	cmd.Flags().StringVar(&orgID, "org", "", "Organisation that owns this application")
	cmd.Flags().StringVar(&appSecret, "secret", "", "Provide the secret for app endpoint(s). If blank, it will be automatically generated")

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
				CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
			}

			err := a.orgRepo.CreateOrganisation(context.Background(), org)
			if err != nil {
				return fmt.Errorf("could not create organisation... %w", err)
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"ID", "Name", "Created at"})

			table.Append([]string{org.UID, org.OrgName, org.CreatedAt.Time().String()})
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

			cfg, err := config.Get()
			if err != nil {
				log.Errorln("error fetching config - ", err)
				return err
			}

			var intervalSeconds uint64
			var retryLimit uint64
			if cfg.Strategy.Type == config.DefaultStrategyProvider {
				intervalSeconds = cfg.Strategy.Default.IntervalSeconds
				retryLimit = cfg.Strategy.Default.RetryLimit
			} else {
				return errors.New("retry strategy not defined in configuration")
			}

			log.Println("Message ", string(d))
			msg := &hookcamp.Message{
				UID:       uuid.New().String(),
				AppID:     appData.UID,
				EventType: hookcamp.EventType(eventType),
				Data:      d,
				Metadata: &hookcamp.MessageMetadata{
					Strategy:        cfg.Strategy.Type,
					NumTrials:       0,
					IntervalSeconds: intervalSeconds,
					RetryLimit:      retryLimit,
					NextSendTime:    primitive.NewDateTimeFromTime(time.Now().Add(time.Duration(intervalSeconds) * time.Second)),
				},
				AppMetadata: &hookcamp.AppMetadata{
					OrgID:     appData.OrgID,
					Secret:    appData.Secret,
					Endpoints: util.ParseMetadataFromEndpoints(appData.Endpoints),
				},
				MessageAttempts: make([]hookcamp.MessageAttempt, 0),
				CreatedAt:       primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:       primitive.NewDateTimeFromTime(time.Now()),
				Status:          hookcamp.ScheduledMessageStatus,
			}

			if len(appData.Endpoints) == 0 {
				return errors.New("app has no configured endpoints")
			}

			ctx, cancelFn = getCtx()
			defer cancelFn()

			if err := a.messageRepo.CreateMessage(ctx, msg); err != nil {
				return fmt.Errorf("could not create message... %w", err)
			}

			fmt.Println("Your message has been created. And will be sent to available endpoints")
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
