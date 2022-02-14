package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/frain-dev/convoy/config/algo"
	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

const MaxResponseSize = 50 * 1024

var cfgSingleton atomic.Value

type DatabaseConfiguration struct {
	Type string `json:"type" envconfig:"CONVOY_DB_TYPE"`
	Dsn  string `json:"dsn" envconfig:"CONVOY_DB_DSN"`
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
	RequireAuth bool               `json:"require_auth"`
	File        FileRealmOption    `json:"file"`
	Native      NativeRealmOptions `json:"native"`
}

type NativeRealmOptions struct {
	Enabled bool `json:"enabled"`
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
type LoggerConfiguration struct {
	Type      LoggerProvider `json:"type"`
	ServerLog struct {
		Level string `json:"level"`
	} `json:"server_log"`
}

type TracerConfiguration struct {
	Type TracerProvider `json:"type"`
}

type CacheConfiguration struct {
	Type  CacheProvider           `json:"type"`
	Redis RedisCacheConfiguration `json:"redis"`
}

type RedisCacheConfiguration struct {
	Dsn string `json:"dsn"`
}

type NewRelicConfiguration struct {
	AppName                  string `json:"app_name"`
	LicenseKey               string `json:"license_key"`
	ConfigEnabled            bool   `json:"config_enabled"`
	DistributedTracerEnabled bool   `json:"distributed_tracer_enabled"`
}

type Configuration struct {
	Auth            AuthConfiguration     `json:"auth,omitempty"`
	Database        DatabaseConfiguration `json:"database"`
	Sentry          SentryConfiguration   `json:"sentry"`
	Queue           QueueConfiguration    `json:"queue"`
	Server          ServerConfiguration   `json:"server"`
	MaxResponseSize uint64                `json:"max_response_size"`
	GroupConfig     GroupConfig           `json:"group"`
	SMTP            SMTPConfiguration     `json:"smtp"`
	Environment     string                `json:"env" envconfig:"CONVOY_ENV" required:"true" default:"development"`
	MultipleTenants bool                  `json:"multiple_tenants"`
	Logger          LoggerConfiguration   `json:"logger"`
	Tracer          TracerConfiguration   `json:"tracer"`
	NewRelic        NewRelicConfiguration `json:"new_relic"`
	Cache           CacheConfiguration    `json:"cache"`
}

const (
	envPrefix string = "convoy"

	DevelopmentEnvironment string = "development"
)

const (
	RedisQueueProvider      QueueProvider           = "redis"
	InMemoryQueueProvider   QueueProvider           = "in-memory"
	DefaultStrategyProvider StrategyProvider        = "default"
	DefaultSignatureHeader  SignatureHeaderProvider = "X-Convoy-Signature"
	ConsoleLoggerProvider   LoggerProvider          = "console"
	NewRelicTracerProvider  TracerProvider          = "new_relic"
	RedisCacheProvider      CacheProvider           = "redis"
)

type GroupConfig struct {
	Strategy        StrategyConfiguration  `json:"strategy"`
	Signature       SignatureConfiguration `json:"signature"`
	DisableEndpoint bool                   `json:"disable_endpoint"`
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

type AuthProvider string
type QueueProvider string
type StrategyProvider string
type SignatureHeaderProvider string
type LoggerProvider string
type TracerProvider string
type CacheProvider string

func (s SignatureHeaderProvider) String() string {
	return string(s)
}

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

// LoadConfig is used to load the configuration from either the json config file
// or the environment variables.
func LoadConfig(p string, override *Configuration) error {
	f, err := os.Open(p)
	if err != nil {
		return err
	}

	defer f.Close()

	c := new(Configuration)

	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return err
	}

	err = envconfig.Process(envPrefix, c)
	if err != nil {
		return err
	}

	// if it's still empty, set it to development
	if c.Environment == "" {
		c.Environment = DevelopmentEnvironment
	}

	if c.Server.HTTP.Port == 0 {
		return errors.New("http port cannot be zero")
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
		log.Warnf("using default signature header: %s", DefaultSignatureHeader)
	}

	kb := c.MaxResponseSize * 1024 // to kilobyte
	if kb == 0 {
		c.MaxResponseSize = MaxResponseSize
	} else if kb > MaxResponseSize {
		log.Warnf("maximum response size of %dkb too large, using default value of %dkb", c.MaxResponseSize, MaxResponseSize/1024)
		c.MaxResponseSize = MaxResponseSize
	} else {
		c.MaxResponseSize = kb
	}

	err = ensureStrategyConfig(c.GroupConfig.Strategy)
	if err != nil {
		return err
	}

	err = ensureQueueConfig(c.Queue)
	if err != nil {
		return err
	}

	err = ensureAuthConfig(c.Auth)
	if err != nil {
		return err
	}

	if len(strings.TrimSpace(override.Queue.Redis.DSN)) > 0 {
		c.Queue.Redis.DSN = override.Queue.Redis.DSN
	}

	if len(strings.TrimSpace(override.Database.Dsn)) > 0 {
		c.Database.Dsn = override.Database.Dsn
	}

	cfgSingleton.Store(c)
	return nil
}

func ensureAuthConfig(authCfg AuthConfiguration) error {
	var err error
	for _, r := range authCfg.File.Basic {
		if r.Username == "" || r.Password == "" {
			return errors.New("username and password are required for basic auth config")
		}

		err = r.Role.Validate("basic auth")
		if err != nil {
			return err
		}
	}

	for _, r := range authCfg.File.APIKey {
		if r.APIKey == "" {
			return errors.New("api-key is required for api-key auth config")
		}

		err = r.Role.Validate("api-key auth")
		if err != nil {
			return err
		}
	}

	return nil
}

func ensureSignature(signature SignatureConfiguration) error {
	_, ok := algo.M[signature.Hash]
	if !ok {
		return fmt.Errorf("invalid hash algorithm - '%s', must be one of %s", signature.Hash, algo.Algos)
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

	case InMemoryQueueProvider:
		return nil

	default:
		return fmt.Errorf("unsupported queue type: %s", queueCfg.Type)
	}
	return nil
}

func ensureStrategyConfig(strategyCfg StrategyConfiguration) error {
	switch strategyCfg.Type {
	case DefaultStrategyProvider:
		if strategyCfg.Default.IntervalSeconds == 0 || strategyCfg.Default.RetryLimit == 0 {
			return errors.New("both interval seconds and retry limit are required for default strategy configuration")
		}
	default:
		return fmt.Errorf("unsupported strategy type: %s", strategyCfg.Type)
	}
	return nil
}
