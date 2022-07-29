package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
			homedir, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			path := filepath.Join(homedir, defaultConfigDir)
			c, err := NewConfig("", "", path)
			if err != nil {
				return err
			}

			if !c.hasDefaultConfigFile {
				return errors.New("login with your cli token to be able to use the switch command")
			}

			if util.IsStringEmpty(projectId) {
				return errors.New("project name is required")
			}

			project := FindProjectByName(c.Projects, projectId)
			if project == nil {
				return fmt.Errorf("project with name: %s not found", projectId)
			}

			c.ActiveProject = project.Name
			c.ActiveDeviceID = project.DeviceID
			c.ActiveApiKey = project.ApiKey

			err = c.WriteConfig()
			if err != nil {
				return err
			}

			log.Info("Switch is successful")
			log.Infof("Project with name: %s is now the active project", projectId)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectId, "project-name", "", "Project Name")

	return cmd
}

func FindProjectByName(projects []Project, projectName string) *Project {
	var project *Project

	for _, project := range projects {
		if strings.TrimSpace(strings.ToLower(project.Name)) ==
			strings.TrimSpace(strings.ToLower(projectName)) {
			return &project
		}
	}

	return project
}
