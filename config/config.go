package config

import (
	"encoding/json"
	"errors"
	"os"
	"sync/atomic"
)

var cfgSingleton atomic.Value

type DatabaseConfiguration struct {
	Dsn string `json:"dsn"`
}

type Configuration struct {
	Auth     AuthConfiguration     `json:"auth"`
	Database DatabaseConfiguration `json:"database"`
	Queue    QueueConfiguration    `json:"queue"`
	Server   struct {
		HTTP struct {
			Port int `json:"port"`
		} `json:"http"`
	}
}

func LoadFromFile(p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}

	defer f.Close()

	c := new(Configuration)

	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return err
	}

	cfgSingleton.Store(c)
	return nil
}

// Get fetches the application configuration. LoadFromFile must have been called
// previously for this to work.
// Use this when you need to get access to the config object at runtime
func Get() (Configuration, error) {
	c, ok := cfgSingleton.Load().(*Configuration)
	if !ok {
		return Configuration{}, errors.New("call Load before this function")
	}

	return *c, nil
}

type AuthProvider string
type QueueProvider string

const (
	NoAuthProvider    AuthProvider = "none"
	BasicAuthProvider AuthProvider = "basic"

	RedisQueueProvider QueueProvider = "redis"
)

type QueueConfiguration struct {
	Type  QueueProvider `json:"type"`
	Redis struct {
		DSN string `json:"dsn"`
	} `json:"redis"`
}

type AuthConfiguration struct {
	Type  AuthProvider `json:"type"`
	Basic struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"basic"`
}
