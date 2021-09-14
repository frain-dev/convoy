package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/config/algo"
	"os"
	"reflect"
	"sync/atomic"
	"time"
)

var cfgSingleton atomic.Value

type DatabaseConfiguration struct {
	Dsn string `json:"dsn"`
}

type Configuration struct {
	Auth              AuthConfiguration   `json:"auth"`
	UIAuth            UIAuthConfiguration `json:"ui"`
	UIAuthorizedUsers map[string]string
	Database          DatabaseConfiguration `json:"database"`
	Queue             QueueConfiguration    `json:"queue"`
	Server            struct {
		HTTP struct {
			Port int `json:"port"`
		} `json:"http"`
	}
	Strategy  StrategyConfiguration  `json:"strategy"`
	Signature SignatureConfiguration `json:"signature"`
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
	err = ensureSignature(c.Signature)
	if err != nil {
		return err
	}

	c.UIAuthorizedUsers = parseAuthorizedUsers(c.UIAuth)

	cfgSingleton.Store(c)
	return nil
}

func ensureSignature(signature SignatureConfiguration) error {
	_, ok := algo.M[signature.Hash]
	if !ok {
		return fmt.Errorf("invalid hash algorithm - '%s', must be one of %s", signature.Hash, reflect.ValueOf(algo.M).MapKeys())
	}
	return nil
}

func parseAuthorizedUsers(auth UIAuthConfiguration) map[string]string {
	users := auth.Basic
	usersMap := make(map[string]string)
	for i := 0; i < len(users); i++ {
		usersMap[users[i].Username] = users[i].Password
	}
	return usersMap
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
type StrategyProvider string
type SignatureHeaderProvider string

const (
	NoAuthProvider          AuthProvider            = "none"
	BasicAuthProvider       AuthProvider            = "basic"
	RedisQueueProvider      QueueProvider           = "redis"
	DefaultStrategyProvider StrategyProvider        = "default"
	DefaultSignatureHeader  SignatureHeaderProvider = "X-Convoy-Signature"
)

type QueueConfiguration struct {
	Type  QueueProvider `json:"type"`
	Redis struct {
		DSN string `json:"dsn"`
	} `json:"redis"`
}

type AuthConfiguration struct {
	Type  AuthProvider `json:"type"`
	Basic Basic        `json:"basic"`
}

type UIAuthConfiguration struct {
	Type                  AuthProvider  `json:"type"`
	Basic                 []Basic       `json:"basic"`
	JwtKey                string        `json:"jwtKey"`
	JwtTokenExpirySeconds time.Duration `json:"jwtTokenExpirySeconds"`
}
type Basic struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type StrategyConfiguration struct {
	Type    StrategyProvider `json:"type"`
	Default struct {
		IntervalSeconds uint64 `json:"intervalSeconds"`
		RetryLimit      uint64 `json:"retryLimit"`
	} `json:"default"`
}

type SignatureConfiguration struct {
	Header SignatureHeaderProvider `json:"header"`
	Hash   string                  `json:"hash"`
}
