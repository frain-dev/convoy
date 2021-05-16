package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/util"
	"github.com/spf13/cobra"
)

func addCreateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a resource",
	}

	cmd.AddCommand(createMessageCommand(a))
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
				d = json.RawMessage([]byte(data))
			}

			if !util.IsStringEmpty(filePath) {
				f, err := os.Open(filePath)
				if err != nil {
					return fmt.Errorf("could not open file... %w", err)
				}

				defer f.Close()

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

			appData, err := a.applicationRepo.FindApplicationByID(ctx, id)
			if err != nil {
				return err
			}

			msg := &hookcamp.Message{
				ID:        uuid.New(),
				AppID:     appData.ID,
				EventType: hookcamp.EventType(eventType),
				Data:      hookcamp.JSONData(d),
				Metadata: &hookcamp.MessageMetadata{
					NumTrials: 0,
				},
				Status: hookcamp.ScheduledMessageStatus,
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
	cmd.Flags().StringVar(&eventType, "event", "", "Event type")
	cmd.Flags().BoolVar(&publish, "publish", false, `If true, it will send the data to the endpoints
attached to the application`)

	return cmd
}
