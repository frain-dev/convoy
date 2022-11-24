package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/socket"
	convoyNet "github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/util"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/frain-dev/convoy/pkg/log"
)

const (
	defaultConfigDir = ".convoy/config"
)

type Config struct {
	Host                 string        `yaml:"host"`
	ActiveDeviceID       string        `yaml:"active_device_id"`
	ActiveApiKey         string        `yaml:"active_api_key"`
	ActiveApplication    string        `yaml:"active_application"`
	Applications         []Application `yaml:"applications"`
	path                 string
	hasDefaultConfigFile bool
	isNewApiKey          bool
	isNewHost            bool
}

func NewConfig(host, apiKey string) (*Config, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(homedir, defaultConfigDir)

	c := &Config{path: path}
	c.hasDefaultConfigFile = HasDefaultConfigFile(path)

	if c.hasDefaultConfigFile {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(data, &c)
		if err != nil {
			return nil, err
		}

		if !util.IsStringEmpty(host) {
			c.isNewHost = IsNewHost(c.Host, host)
			c.Host = host
		}

		if !util.IsStringEmpty(apiKey) {
			c.isNewApiKey = IsNewApiKey(c, apiKey)
			c.ActiveApiKey = apiKey
		}
		return c, nil
	}

	c.Host = host
	c.ActiveApiKey = apiKey

	return c, nil
}

func (c *Config) WriteConfig() error {
	d, err := yaml.Marshal(&c)
	if err != nil {
		return err
	}

	if err := os.WriteFile(c.path, []byte(d), 0644); err != nil {
		return err
	}

	return nil
}

type Application struct {
	UID      string `yaml:"uid"`
	Name     string `yaml:"name"`
	ApiKey   string `yaml:"api_key"`
	DeviceID string `yaml:"device_id"`
}

func addLoginCommand() *cobra.Command {
	var apiKey string
	var host string

	cmd := &cobra.Command{
		Use:               "login",
		Short:             "Logs into your Convoy instance using a CLI API Key",
		PersistentPreRun:  func(cmd *cobra.Command, args []string) {},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := NewConfig(host, apiKey)
			if err != nil {
				return err
			}

			if util.IsStringEmpty(c.Host) {
				return errors.New("host is required")
			}

			if util.IsStringEmpty(c.ActiveApiKey) {
				return errors.New("api key is required")
			}

			deviceID := findDeviceID(c)
			hostName, err := generateDeviceHostName()
			if err != nil {
				return err
			}

			loginRequest := &socket.LoginRequest{HostName: hostName, DeviceID: deviceID}
			body, err := json.Marshal(loginRequest)
			if err != nil {
				return err
			}

			var response *socket.LoginResponse

			dispatch, err := convoyNet.NewDispatcher(time.Second*10, "")
			if err != nil {
				return err
			}

			url := fmt.Sprintf("%s/stream/login", c.Host)
			resp, err := dispatch.SendCliRequest(url, convoy.HttpPost, c.ActiveApiKey, body)
			if err != nil {
				return err
			}

			if resp.StatusCode != 200 {
				return errors.New(string(resp.Body))
			}

			err = json.Unmarshal(resp.Body, &response)
			if err != nil {
				return err
			}

			err = WriteConfig(c, response)
			if err != nil {
				return err
			}

			log.Info("Login Success!")
			log.Infof("Project: %s", response.Group.Name)
			log.Infof("Application: %s", response.App.Title)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiKey, "api-key", "", "API Key")
	cmd.Flags().StringVar(&host, "host", "", "Host")

	return cmd
}

func WriteConfig(c *Config, response *socket.LoginResponse) error {
	name := fmt.Sprintf("%s (%s)", response.App.Title, response.Group.Name)
	c.ActiveApplication = name
	c.ActiveDeviceID = response.Device.UID

	if c.hasDefaultConfigFile {
		if c.isNewHost {
			// if the host is different from the current host in the config file,
			// the data in the config file is overwritten
			c.Applications = []Application{
				{
					UID:      response.App.UID,
					Name:     name,
					ApiKey:   c.ActiveApiKey,
					DeviceID: response.Device.UID,
				},
			}
		}

		if c.isNewApiKey {
			if doesAppExist(c, response.App.UID) {
				return fmt.Errorf("app with ID (%s) has been added already", response.App.UID)
			}

			// If the api key provided is different from the active api key,
			// we append the project returned to the list of projects within the config
			c.Applications = append(c.Applications, Application{
				UID:      response.App.UID,
				Name:     name,
				ApiKey:   c.ActiveApiKey,
				DeviceID: response.Device.UID,
			})
		}

	} else {
		// Make sure the directory holding our config exists
		if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
			return err
		}
		c.Applications = []Application{
			{
				UID:      response.App.UID,
				Name:     name,
				ApiKey:   c.ActiveApiKey,
				DeviceID: response.Device.UID,
			},
		}
	}

	err := c.WriteConfig()
	if err != nil {
		return err
	}

	return nil
}

func HasDefaultConfigFile(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func IsNewHost(currentHost, newHost string) bool {
	return currentHost != newHost
}

// The api key is considered new if it doesn't already
// exist within the config file
func IsNewApiKey(c *Config, apiKey string) bool {
	for _, project := range c.Applications {
		if project.ApiKey == apiKey {
			return false
		}
	}

	return true
}

func findDeviceID(c *Config) string {
	var deviceID string

	for _, app := range c.Applications {
		if app.ApiKey == c.ActiveApiKey {
			return app.DeviceID
		}
	}

	return deviceID
}

func doesAppExist(c *Config, appId string) bool {
	for _, app := range c.Applications {
		if app.UID == appId {
			return true
		}
	}

	return false
}

// generateDeviceHostName uses the machine's host name and the mac address to generate a predictable unique id per device
func generateDeviceHostName() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		return "", err
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var mac uint64
	for _, i := range interfaces {
		if i.Flags&net.FlagUp != 0 && !bytes.Equal(i.HardwareAddr, nil) {

			// Skip virtual MAC addresses (Locally Administered Addresses).
			if i.HardwareAddr[0]&2 == 2 {
				continue
			}

			for j, b := range i.HardwareAddr {
				if j >= 8 {
					break
				}
				mac <<= 8
				mac += uint64(b)
			}
		}
	}

	return fmt.Sprintf("%v-%v", name, mac), nil
}
