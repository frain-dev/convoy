package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	log "github.com/sirupsen/logrus"
)

const (
	defaultConfigDir = ".convoy/config"
)

type Config struct {
	Host                 string    `yaml:"host"`
	ActiveDeviceID       string    `yaml:"active_device_id"`
	ActiveApiKey         string    `yaml:"active_api_key"`
	ActiveProject        string    `yaml:"active_project"`
	Projects             []Project `yaml:"projects"`
	path                 string
	hasDefaultConfigFile bool
	isNewApiKey          bool
	isNewHost            bool
}

func NewConfig(host, apiKey, path string) (*Config, error) {
	c := &Config{path: path}
	c.hasDefaultConfigFile = HasDefaultConfigFile(path)

	if c.hasDefaultConfigFile {
		data, err := ioutil.ReadFile(path)
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

	if err := ioutil.WriteFile(c.path, []byte(d), 0644); err != nil {
		return err
	}

	return nil
}

type Project struct {
	UID      string `yaml:"uid"`
	Name     string `yaml:"name"`
	ApiKey   string `yaml:"api_key"`
	DeviceID string `yaml:"device_id"`
	App      App    `yaml:"app"`
}

type App struct {
	UID  string `yaml:"uid"`
	Name string `yaml:"name"`
}

func addLoginCommand(a *app) *cobra.Command {
	var apiKey string
	var host string

	cmd := &cobra.Command{
		Use:               "login",
		Short:             "Logs into your Convoy instance using your CLI API Key",
		PersistentPreRun:  func(cmd *cobra.Command, args []string) {},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
		RunE: func(cmd *cobra.Command, args []string) error {
			homedir := os.Getenv("HOME")
			if util.IsStringEmpty(homedir) {
				return errors.New("No $HOME environment variable found, required to set Config Directory")
			}

			path := filepath.Join(homedir, defaultConfigDir)
			c, err := NewConfig(host, apiKey, path)
			if err != nil {
				return err
			}

			if util.IsStringEmpty(c.Host) {
				return errors.New("host is required")
			}

			if util.IsStringEmpty(c.ActiveApiKey) {
				return errors.New("api key is required")
			}

			deviceID := FindDeviceID(c)

			loginRequest := &services.LoginRequest{HostName: uuid.NewString(), DeviceID: deviceID}
			body, err := json.Marshal(loginRequest)
			if err != nil {
				return err
			}

			var response *services.LoginResponse

			dispatch := net.NewDispatcher(time.Second * 10)
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

func WriteConfig(c *Config, response *services.LoginResponse) error {
	c.ActiveProject = response.Group.Name
	c.ActiveDeviceID = response.Device.UID

	if c.hasDefaultConfigFile {
		if c.isNewApiKey {
			// If the api key provided is different from the active api key,
			// we append the project returned to the list of projects within the config
			c.Projects = append(c.Projects, Project{
				UID:      response.Group.UID,
				Name:     response.Group.Name,
				ApiKey:   c.ActiveApiKey,
				DeviceID: response.Device.UID,
				App: App{
					UID:  response.App.UID,
					Name: response.App.Title,
				},
			})
		} else if c.isNewHost {
			// if the host is different from the current host in the config file,
			// the data in the config file is overwritten
			c.Projects = []Project{
				{
					UID:      response.Group.UID,
					Name:     response.Group.Name,
					ApiKey:   c.ActiveApiKey,
					DeviceID: response.Device.UID,
					App: App{
						UID:  response.App.UID,
						Name: response.App.Title,
					},
				},
			}
		}
	} else {
		// Make sure the directory holding our config exists
		if err := os.MkdirAll(filepath.Dir(c.path), 0755); err != nil {
			return err
		}
		c.Projects = []Project{
			{
				UID:      response.Group.UID,
				Name:     response.Group.Name,
				ApiKey:   c.ActiveApiKey,
				DeviceID: response.Device.UID,
				App: App{
					UID:  response.App.UID,
					Name: response.App.Title,
				},
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
	for _, project := range c.Projects {
		if project.ApiKey == apiKey {
			return false
		}
	}

	return true
}

func FindDeviceID(c *Config) string {
	var deviceID string

	for _, project := range c.Projects {
		if project.ApiKey == c.ActiveApiKey {
			return project.DeviceID
		}
	}

	return deviceID
}
