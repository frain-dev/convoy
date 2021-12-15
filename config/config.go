package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync/atomic"

	"github.com/frain-dev/convoy/config/algo"
)

var cfgSingleton atomic.Value

type DatabaseConfiguration struct {
	Dsn string `json:"dsn"`
}

type SentryConfiguration struct {
	Dsn string `json:"dsn"`
}

type ServerConfiguration struct {
	HTTP struct {
		SSL         bool   `json:"ssl"`
		SSLCertFile string `json:"ssl_cert_file"`
		SSLKeyFile  string `json:"ssl_key_file"`
		Port        uint32 `json:"port"`
	} `json:"http"`
}

type QueueConfiguration struct {
	Type  QueueProvider `json:"type"`
	Redis struct {
		DSN string `json:"dsn"`
	} `json:"redis"`
}

type ConsulConfiguration struct {
	DSN string `json:"dsn"`
}

type FileRealmOption struct {
	Basic  []BasicAuth  `json:"basic"`
	APIKey []APIKeyAuth `json:"api_key"`
}

type AuthConfiguration struct {
	RequireAuth bool         `json:"require_auth"`
	Type        AuthProvider `json:"type"`
	Basic       Basic
	File        FileRealmOption `json:"file"`
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
	Strategy        StrategyConfiguration  `json:"strategy"`
	Signature       SignatureConfiguration `json:"signature"`
	DisableEndpoint bool                   `json:"disable_endpoint"`
}

type Configuration struct {
	Auth              AuthConfiguration     `json:"auth,omitempty"`
	UIAuthorizedUsers map[string]string     `json:"-"`
	Database          DatabaseConfiguration `json:"database"`
	Sentry            SentryConfiguration   `json:"sentry"`
	Queue             QueueConfiguration    `json:"queue"`
	Consul            ConsulConfiguration   `json:"consul"`
	Server            ServerConfiguration   `json:"server"`
	GroupConfig       GroupConfig           `json:"group"`
	SMTP              SMTPConfiguration     `json:"smtp"`
	Environment       string                `json:"env"`
	MultipleTenants   bool                  `json:"multiple_tenants"`
}

type AuthProvider string
type QueueProvider string
type StrategyProvider string
type SignatureHeaderProvider string

const (
	DevelopmentEnvironment string = "development"
)

const (
	NoAuthProvider          AuthProvider            = "none"
	BasicAuthProvider       AuthProvider            = "basic"
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

	if mongoDsn := os.Getenv("CONVOY_MONGO_DSN"); mongoDsn != "" {
		c.Database = DatabaseConfiguration{Dsn: mongoDsn}
	}

	if queueDsn := os.Getenv("CONVOY_REDIS_DSN"); queueDsn != "" {
		c.Queue = QueueConfiguration{
			Type: "redis",
			Redis: struct {
				DSN string `json:"dsn"`
			}{
				DSN: queueDsn,
			},
		}
	}

	// This enables us deploy to Heroku where the $PORT is provided
	// dynamically.
	if port, err := strconv.Atoi(os.Getenv("PORT")); err == nil {
		c.Server.HTTP.Port = uint32(port)
	}

	if s := os.Getenv("CONVOY_SSL"); s != "" {
		v, err := strconv.ParseBool(s)
		if err != nil {
			return err
		}
		c.Server.HTTP.SSL = v

		if c.Server.HTTP.SSL {
			c.Server.HTTP.SSLCertFile = os.Getenv("CONVOY_SSL_CERT_FILE")
			c.Server.HTTP.SSLKeyFile = os.Getenv("CONVOY_SSL_KEY_FILE")
		}
	}
	err = ensureSSL(c.Server)
	if err != nil {
		return err
	}

	if env := os.Getenv("CONVOY_ENV"); env != "" {
		c.Environment = env
	}

	// if it's still empty, set it to development
	if c.Environment == "" {
		c.Environment = DevelopmentEnvironment
	}

	if sentryDsn := os.Getenv("CONVOY_SENTRY_DSN"); sentryDsn != "" {
		c.Sentry = SentryConfiguration{Dsn: sentryDsn}
	}

	if signatureHeader := os.Getenv("CONVOY_SIGNATURE_HEADER"); signatureHeader != "" {
		c.GroupConfig.Signature.Header = SignatureHeaderProvider(signatureHeader)
	}

	if signatureHash := os.Getenv("CONVOY_SIGNATURE_HASH"); signatureHash != "" {
		c.GroupConfig.Signature.Hash = signatureHash
	}
	err = ensureSignature(c.GroupConfig.Signature)
	if err != nil {
		return err
	}

	if retryStrategy := os.Getenv("CONVOY_RETRY_STRATEGY"); retryStrategy != "" {

		intervalSeconds, err := retrieveIntfromEnv("CONVOY_INTERVAL_SECONDS")
		if err != nil {
			return err
		}

		retryLimit, err := retrieveIntfromEnv("CONVOY_RETRY_LIMIT")
		if err != nil {
			return err
		}

		c.GroupConfig.Strategy = StrategyConfiguration{
			Type: StrategyProvider(retryStrategy),
			Default: struct {
				IntervalSeconds uint64 `json:"intervalSeconds"`
				RetryLimit      uint64 `json:"retryLimit"`
			}{
				IntervalSeconds: intervalSeconds,
				RetryLimit:      retryLimit,
			},
		}

	}

	if e := os.Getenv("CONVOY_DISABLE_ENDPOINT"); e != "" {
		if d, err := strconv.ParseBool(e); err == nil {
			c.GroupConfig.DisableEndpoint = d
		}
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

func retrieveIntfromEnv(config string) (uint64, error) {
	value, err := strconv.Atoi(os.Getenv(config))
	if err != nil {
		return 0, errors.New("Failed to parse - " + config)
	}

	if value == 0 {
		return 0, errors.New("Invalid - " + config)
	}

	return uint64(value), nil
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
