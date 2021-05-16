package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

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

	cmd := &cobra.Command{
		Use:   "message",
		Short: "Create a message",
		RunE: func(cmd *cobra.Command, args []string) error {

			var d json.RawMessage

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

			return nil
		},
	}

	cmd.Flags().StringVarP(&data, "data", "d", "", "Raw JSON data that will be sent to the endpoints")
	cmd.Flags().StringVarP(&appID, "app", "a", "", "Application ID")
	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Path to file containing JSON data")

	return cmd
}
