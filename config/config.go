package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

var cfgSingleton atomic.Value

// String canonlizes a database provider
func (p DatabaseProvider) String() string { return strings.ToLower(string(p)) }

// Validate makes sure we can support the said database
func (p DatabaseProvider) Validate() error {
	switch p {
	case MysqlDatabaseProvider, PostgresDatabaseProvider:
		return nil
	default:
		return fmt.Errorf("unsupported database type (%s)", p)
	}
}

type DatabaseProvider string

const (
	MysqlDatabaseProvider    = "mysql"
	PostgresDatabaseProvider = "postgres"
)

type DatabaseConfiguration struct {
	Type DatabaseProvider `json:"type"`
	Dsn  string           `json:"dsn"`
}

type Configuration struct {
	Database DatabaseConfiguration `json:"database"`
	Queue    QueueConfiguration    `json:"queue"`
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

type QueueProvider string

const (
	RedisQueueProvider QueueProvider = "redis"
)

type QueueConfiguration struct {
	Type  QueueProvider `json:"type"`
	Redis struct {
		DSN string `json:"dsn"`
	} `json:"redis"`
}
