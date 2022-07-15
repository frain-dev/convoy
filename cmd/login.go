package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const (
	defaultConfigDir = ".convoy/config"
)

func addLoginCommand(a *app) *cobra.Command {
	var apiKey string
	var host string

	cmd := &cobra.Command{
		Use:               "login",
		Short:             "Logins to your Convoy instance using your CLI token",
		PersistentPreRun:  func(cmd *cobra.Command, args []string) {},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
		RunE: func(cmd *cobra.Command, args []string) error {
			var deviceId string

			homedir := os.Getenv("HOME")
			if util.IsStringEmpty(homedir) {
				return errors.New("No $HOME environment variable found, required to set Config Directory")
			}

			path := filepath.Join(homedir, defaultConfigDir)
			hasConfigFile := hasDefaultConfigFile(path)

			if hasConfigFile {
				//Todo (fetch the device ID from the config file)
				fmt.Println("has config file")
			}

			// get the host name
			hostname, err := os.Hostname()
			if err != nil {
				return err
			}

			loginRequest := &LoginRequest{HostName: fmt.Sprintf("%s-%s", uuid.New().String(), hostname), DeviceID: deviceId}
			body, err := json.Marshal(loginRequest)
			if err != nil {
				return err
			}

			var device datastore.Device

			dispatch := net.NewDispatcher(time.Second * 10)
			url := fmt.Sprintf("%s/stream/login", host)
			resp, err := dispatch.SendCliRequest(url, convoy.HttpPost, apiKey, body)
			if err != nil {
				return err
			}

			err = json.Unmarshal(resp.Body, &device)
			if err != nil {
				return err
			}

			fmt.Println("resp", resp)
			fmt.Printf("%+v\n", device)

			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "API Key")
	cmd.Flags().StringVar(&host, "host", "", "Host")

	return cmd
}

func hasDefaultConfigFile(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}
