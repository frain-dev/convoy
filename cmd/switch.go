package main

import (
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/util"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

func addSwitchCommand() *cobra.Command {
	var projectId string

	cmd := &cobra.Command{
		Use:               "switch",
		Short:             "Switches the current project context using your Project ID",
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

			if util.IsStringEmpty(projectId) {
				return errors.New("project ID is required")
			}

			project := FindProjectByID(c.Projects, projectId)
			if project == nil {
				return fmt.Errorf("project with ID :%s not found", projectId)
			}

			c.ActiveProject = project.Name
			c.ActiveDeviceID = project.DeviceID
			c.ActiveApiKey = project.ApiKey

			err = c.WriteConfig()
			if err != nil {
				return err
			}

			log.Info("Switch is successful")
			log.Infof("Project with ID: %s is now the active project", projectId)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectId, "project-id", "", "Project ID")

	return cmd
}

func FindProjectByID(projects []Project, projectId string) *Project {
	var project *Project

	for _, project := range projects {
		if project.UID == projectId {
			return &project
		}
	}

	return project
}
