package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sync/atomic"

	"github.com/kelseyhightower/envconfig"

	"github.com/frain-dev/convoy/config/algo"
)

var cfgSingleton atomic.Value

type DatabaseConfiguration struct {
	Dsn string `json:"dsn" envconfig:"CONVOY_MONGO_DSN"`
}

type SentryConfiguration struct {
	Dsn string `json:"dsn" envconfig:"CONVOY_SENTRY_DSN"`
}

type ServerConfiguration struct {
	HTTP HTTPServerConfiguration `json:"http"`
}

type HTTPServerConfiguration struct {
	SSL         bool   `json:"ssl" envconfig:"SSL"`
	SSLCertFile string `json:"ssl_cert_file" envconfig:"CONVOY_SSL_CERT_FILE"`
	SSLKeyFile  string `json:"ssl_key_file" envconfig:"CONVOY_SSL_KEY_FILE"`
	Port        uint32 `json:"port" envconfig:"PORT"`
}

type QueueConfiguration struct {
	Type  QueueProvider           `json:"type" envconfig:"CONVOY_QUEUE_PROVIDER"`
	Redis RedisQueueConfiguration `json:"redis"`
}

type RedisQueueConfiguration struct {
	DSN string `json:"dsn" envconfig:"CONVOY_REDIS_DSN"`
}

type FileRealmOption struct {
	Basic  []BasicAuth  `json:"basic" bson:"basic"`
	APIKey []APIKeyAuth `json:"api_key"`
}

type AuthConfiguration struct {
	RequireAuth bool            `json:"require_auth"`
	File        FileRealmOption `json:"file"`
}

type StrategyConfiguration struct {
	Type    StrategyProvider             `json:"type"`
	Default DefaultStrategyConfiguration `json:"default"`
}

type DefaultStrategyConfiguration struct {
	IntervalSeconds uint64 `json:"intervalSeconds" envconfig:"CONVOY_INTERVAL_SECONDS"`
	RetryLimit      uint64 `json:"retryLimit" envconfig:"CONVOY_RETRY_LIMIT"`
}

type SignatureConfiguration struct {
	Header SignatureHeaderProvider `json:"header" envconfig:"CONVOY_SIGNATURE_HEADER"`
	Hash   string                  `json:"hash" envconfig:"CONVOY_SIGNATURE_HASH"`
}

type SMTPConfiguration struct {
	Provider string `json:"provider"`
	URL      string `json:"url"`
	Port     uint32 `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	ReplyTo  string `json:"reply-to"`
}

type GroupConfig struct {
	Strategy        StrategyConfiguration
	Signature       SignatureConfiguration
	DisableEndpoint bool `envconfig:"CONVOY_DISABLE_ENDPOINT"`
}

type Configuration struct {
	Auth            AuthConfiguration     `json:"auth,omitempty"`
	Database        DatabaseConfiguration `json:"database"`
	Sentry          SentryConfiguration   `json:"sentry"`
	Queue           QueueConfiguration    `json:"queue"`
	Server          ServerConfiguration   `json:"server"`
	GroupConfig     GroupConfig           `json:"group"`
	SMTP            SMTPConfiguration     `json:"smtp"`
	Environment     string                `json:"env" envconfig:"CONVOY_ENV" default:"development"`
	MultipleTenants bool                  `json:"multiple_tenants"`
}

type QueueProvider string
type StrategyProvider string
type SignatureHeaderProvider string

const (
	DevelopmentEnvironment string = "development"
)

const (
	RedisQueueProvider      QueueProvider           = "redis"
	DefaultStrategyProvider StrategyProvider        = "default"
	DefaultSignatureHeader  SignatureHeaderProvider = "X-Convoy-Signature"
)

func (s SignatureHeaderProvider) String() string {
	return string(s)
}

func LoadConfig(p string) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}

	defer f.Close()

	c := new(Configuration)

	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return err
	}

	err = envconfig.Process("CONVOY", c)
	if err != nil {
		return err
	}

	err = ensureSSL(c.Server)
	if err != nil {
		return err
	}

	err = ensureSignature(c.GroupConfig.Signature)
	if err != nil {
		return err
	}
	if c.GroupConfig.Signature.Header == "" {
		c.GroupConfig.Signature.Header = DefaultSignatureHeader
	}

	err = ensureQueueConfig(c.Queue)
	if err != nil {
		return err
	}

	err = ensureStrategyConfig(c.GroupConfig.Strategy)
	if err != nil {
		return err
	}

	err = ensureAuthConfig(c.Auth)
	if err != nil {
		return err
	}

	cfgSingleton.Store(c)
	return nil
}

func ensureAuthConfig(auth AuthConfiguration) error {
	var err error
	for _, r := range auth.File.Basic {
		if r.Username == "" || r.Password == "" {
			return errors.New("username and password are required for basic auth config")
		}

		err = checkRole(&r.Role, "basic auth")
		if err != nil {
			return err
		}
	}

	for _, r := range auth.File.APIKey {
		if r.APIKey == "" {
			return errors.New("api-key is required for api-key auth config")
		}

		err = checkRole(&r.Role, "api-key auth")
		if err != nil {
			return err
		}
	}

	return nil
}

func ensureSignature(signature SignatureConfiguration) error {
	_, ok := algo.M[signature.Hash]
	if !ok {
		return fmt.Errorf("invalid hash algorithm - '%s', must be one of %s", signature.Hash, reflect.ValueOf(algo.M).MapKeys())
	}
	return nil
}

func ensureSSL(s ServerConfiguration) error {
	if s.HTTP.SSL {
		if s.HTTP.SSLCertFile == "" || s.HTTP.SSLKeyFile == "" {
			return errors.New("both cert_file and key_file are required for ssl")
		}
	}
	return nil
}

func ensureQueueConfig(queueCfg QueueConfiguration) error {
	switch queueCfg.Type {
	case RedisQueueProvider:
		if queueCfg.Redis.DSN == "" {
			return errors.New("redis queue dsn is empty")
		}
	default:
		return fmt.Errorf("unsupported queue type: %s", queueCfg.Type)
	}
	return nil
}

func ensureStrategyConfig(strategyCfg StrategyConfiguration) error {
	switch strategyCfg.Type {
	case DefaultStrategyProvider:
		d := &strategyCfg.Default
		if d.IntervalSeconds == 0 || d.RetryLimit == 0 {
			return errors.New("both interval seconds and retry limit are required for default strategy configuration")
		}
	default:
		return fmt.Errorf("unsupported strategy type: %s", strategyCfg.Type)
	}
	return nil
}

//func retrieveIntfromEnv(config string) (uint64, error) {
//	value, err := strconv.Atoi(os.Getenv(config))
//	if err != nil {
//		return 0, errors.New("Failed to parse - " + config)
//	}
//
//	if value == 0 {
//		return 0, errors.New("Invalid - " + config)
//	}
//
//	return uint64(value), nil
//}

// Get fetches the application configuration. LoadConfig must have been called
// previously for this to work.
// Use this when you need to get access to the config object at runtime
func Get() (Configuration, error) {
	c, ok := cfgSingleton.Load().(*Configuration)
	if !ok {
		return Configuration{}, errors.New("call Load before this function")
	}

	return *c, nil
}
