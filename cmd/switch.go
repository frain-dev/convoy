package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/frain-dev/convoy/util"
	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/pkg/log"
)

func addSwitchCommand() *cobra.Command {
	var appName string
	var appId string

	cmd := &cobra.Command{
		Use:               "switch",
		Short:             "Switches the current application context",
		PersistentPreRun:  func(cmd *cobra.Command, args []string) {},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewConfig("", "")
			if err != nil {
				return err
			}

			if !c.hasDefaultConfigFile {
				return errors.New("login with your cli token to be able to use the switch command")
			}

			if util.IsStringEmpty(appName) && util.IsStringEmpty(appId) {
				return errors.New("one of app name or app id is required")
			}

			var application *Application
			if !util.IsStringEmpty(appName) {
				application = FindApplicationByName(c.Applications, appName)
				if application == nil {
					return fmt.Errorf("app with name: %s not found", appName)
				}
			}

			if !util.IsStringEmpty(appId) {
				application = FindApplicationById(c.Applications, appId)
				if application == nil {
					return fmt.Errorf("app with id: %s not found", appId)
				}
			}

			c.ActiveApplication = application.Name
			c.ActiveDeviceID = application.DeviceID
			c.ActiveApiKey = application.ApiKey

			err = c.WriteConfig()
			if err != nil {
				return err
			}

			log.Infof("%s is now the active application", c.ActiveApplication)
			return nil
		},
	}

	cmd.Flags().StringVar(&appName, "name", "", "Application Name")
	cmd.Flags().StringVar(&appId, "id", "", "Application Id")

	return cmd
}

func FindApplicationByName(applications []Application, appName string) *Application {
	var app *Application

	for _, app := range applications {
		if strings.TrimSpace(strings.ToLower(app.Name)) == strings.TrimSpace(strings.ToLower(appName)) {
			return &app
		}
	}

	return app
}

func FindApplicationById(applications []Application, appId string) *Application {
	var app *Application

	for _, app := range applications {
		if strings.TrimSpace(strings.ToLower(app.UID)) == strings.TrimSpace(strings.ToLower(appId)) {
			return &app
		}
	}

	return app
}
