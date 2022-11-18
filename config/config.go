package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync/atomic"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

const (
	MaxResponseSizeKb = 50                       // in kilobytes
	MaxResponseSize   = MaxResponseSizeKb * 1024 // in bytes
	MaxRequestSize    = MaxResponseSize

	DefaultHost = "localhost:5005"
)

var cfgSingleton atomic.Value

var DefaultConfiguration = Configuration{
	Host:            DefaultHost,
	Environment:     OSSEnvironment,
	MaxResponseSize: MaxResponseSize,
	Server: ServerConfiguration{
		HTTP: HTTPServerConfiguration{
			SSL:        false,
			Port:       5005,
			WorkerPort: 5006,
		},
	},
	Database: DatabaseConfiguration{
		Type: MongodbDatabaseProvider,
		Dsn:  "mongodb://localhost:27017/convoy",
	},
	Queue: QueueConfiguration{
		Type: RedisQueueProvider,
		Redis: RedisQueueConfiguration{
			Dsn: "redis://localhost:6378",
		},
	},
	Logger: LoggerConfiguration{
		Level: "error",
	},
}

type DatabaseConfiguration struct {
	Type DatabaseProvider `json:"type" envconfig:"CONVOY_DB_TYPE"`
	Dsn  string           `json:"dsn" envconfig:"CONVOY_DB_DSN"`
}

type ServerConfiguration struct {
	HTTP HTTPServerConfiguration `json:"http"`
}

type HTTPServerConfiguration struct {
	SSL         bool   `json:"ssl" envconfig:"SSL"`
	SSLCertFile string `json:"ssl_cert_file" envconfig:"CONVOY_SSL_CERT_FILE"`
	SSLKeyFile  string `json:"ssl_key_file" envconfig:"CONVOY_SSL_KEY_FILE"`
	Port        uint32 `json:"port" envconfig:"PORT"`
	WorkerPort  uint32 `json:"worker_port" envconfig:"WORKER_PORT"`
	SocketPort  uint32 `json:"socket_port" envconfig:"SOCKET_PORT"`
	HttpProxy   string `json:"proxy" envconfig:"HTTP_PROXY"`
}

type QueueConfiguration struct {
	Type  QueueProvider           `json:"type" envconfig:"CONVOY_QUEUE_PROVIDER"`
	Redis RedisQueueConfiguration `json:"redis"`
}

type PrometheusConfiguration struct {
	Dsn string `json:"dsn" envconfig:"CONVOY_PROM_DSN"`
}

type RedisQueueConfiguration struct {
	Dsn string `json:"dsn" envconfig:"CONVOY_REDIS_DSN"`
}

type FileRealmOption struct {
	Basic  BasicAuthConfig  `json:"basic" bson:"basic" envconfig:"CONVOY_BASIC_AUTH_CONFIG"`
	APIKey APIKeyAuthConfig `json:"api_key" envconfig:"CONVOY_API_KEY_CONFIG"`
}

type AuthConfiguration struct {
	File   FileRealmOption    `json:"file"`
	Native NativeRealmOptions `json:"native"`
	Jwt    JwtRealmOptions    `json:"jwt"`
}

type NativeRealmOptions struct {
	Enabled bool `json:"enabled" envconfig:"CONVOY_NATIVE_REALM_ENABLED"`
}

type JwtRealmOptions struct {
	Enabled       bool   `json:"enabled" envconfig:"CONVOY_JWT_REALM_ENABLED"`
	Secret        string `json:"secret" envconfig:"CONVOY_JWT_SECRET"`
	Expiry        int    `json:"expiry" envconfig:"CONVOY_JWT_EXPIRY"`
	RefreshSecret string `json:"refresh_secret" envconfig:"CONVOY_JWT_REFRESH_SECRET"`
	RefreshExpiry int    `json:"refresh_expiry" envconfig:"CONVOY_JWT_REFRESH_EXPIRY"`
}

type SMTPConfiguration struct {
	Provider string `json:"provider" envconfig:"CONVOY_SMTP_PROVIDER"`
	URL      string `json:"url" envconfig:"CONVOY_SMTP_URL"`
	Port     uint32 `json:"port" envconfig:"CONVOY_SMTP_PORT"`
	Username string `json:"username" envconfig:"CONVOY_SMTP_USERNAME"`
	Password string `json:"password" envconfig:"CONVOY_SMTP_PASSWORD"`
	From     string `json:"from" envconfig:"CONVOY_SMTP_FROM"`
	ReplyTo  string `json:"reply-to" envconfig:"CONVOY_SMTP_REPLY_TO"`
}

type LoggerConfiguration struct {
	Level string `json:"level" envconfig:"CONVOY_LOGGER_LEVEL"`
}

type TracerConfiguration struct {
	Type     TracerProvider        `json:"type" envconfig:"CONVOY_TRACER_PROVIDER"`
	NewRelic NewRelicConfiguration `json:"new_relic"`
}

type CacheConfiguration struct {
	Type  CacheProvider           `json:"type"  envconfig:"CONVOY_CACHE_PROVIDER"`
	Redis RedisCacheConfiguration `json:"redis"`
}

type RedisCacheConfiguration struct {
	Dsn string `json:"dsn" envconfig:"CONVOY_REDIS_DSN"`
}

type LimiterConfiguration struct {
	Type  LimiterProvider           `json:"type" envconfig:"CONVOY_LIMITER_TYPE"`
	Redis RedisLimiterConfiguration `json:"redis"`
}

type RedisLimiterConfiguration struct {
	Dsn string `json:"dsn" envconfig:"CONVOY_REDIS_DSN"`
}

type NewRelicConfiguration struct {
	AppName                  string `json:"app_name" envconfig:"CONVOY_NEWRELIC_APP_NAME"`
	LicenseKey               string `json:"license_key" envconfig:"CONVOY_NEWRELIC_LICENSE_KEY"`
	ConfigEnabled            bool   `json:"config_enabled" envconfig:"CONVOY_NEWRELIC_CONFIG_ENABLED"`
	DistributedTracerEnabled bool   `json:"distributed_tracer_enabled" envconfig:"CONVOY_NEWRELIC_DISTRIBUTED_TRACER_ENABLED"`
}

type SearchConfiguration struct {
	Type      SearchProvider         `json:"type" envconfig:"CONVOY_SEARCH_TYPE"`
	Typesense TypesenseConfiguration `json:"typesense"`
}

type TypesenseConfiguration struct {
	Host   string `json:"host" envconfig:"CONVOY_TYPESENSE_HOST"`
	ApiKey string `json:"api_key" envconfig:"CONVOY_TYPESENSE_API_KEY"`
}

type FeatureFlagConfiguration struct {
	Type  FeatureFlagProvider `json:"type" envconfig:"CONVOY_FEATURE_FLAG_TYPE"`
	Flipt FliptConfiguration  `json:"flipt"`
}

type FliptConfiguration struct {
	Host string `json:"host" envconfig:"CONVOY_FLIPT_HOST"`
}

const (
	envPrefix              string = "convoy"
	DevelopmentEnvironment string = "development"
	OSSEnvironment         string = "oss"
)

const (
	RedisQueueProvider                 QueueProvider           = "redis"
	DefaultStrategyProvider            StrategyProvider        = "linear"
	ExponentialBackoffStrategyProvider StrategyProvider        = "exponential"
	DefaultSignatureHeader             SignatureHeaderProvider = "X-Convoy-Signature"
	ConsoleLoggerProvider              LoggerProvider          = "console"
	NewRelicTracerProvider             TracerProvider          = "new_relic"
	RedisCacheProvider                 CacheProvider           = "redis"
	RedisLimiterProvider               LimiterProvider         = "redis"
	MongodbDatabaseProvider            DatabaseProvider        = "mongodb"
	InMemoryDatabaseProvider           DatabaseProvider        = "in-memory"
)

type (
	AuthProvider            string
	QueueProvider           string
	StrategyProvider        string
	SignatureHeaderProvider string
	LoggerProvider          string
	TracerProvider          string
	CacheProvider           string
	LimiterProvider         string
	DatabaseProvider        string
	SearchProvider          string
	FeatureFlagProvider     string
)

func (s SignatureHeaderProvider) String() string {
	return string(s)
}

type Configuration struct {
	Auth            AuthConfiguration        `json:"auth,omitempty"`
	Database        DatabaseConfiguration    `json:"database"`
	Queue           QueueConfiguration       `json:"queue"`
	Prometheus      PrometheusConfiguration  `json:"prometheus"`
	Server          ServerConfiguration      `json:"server"`
	MaxResponseSize uint64                   `json:"max_response_size" envconfig:"CONVOY_MAX_RESPONSE_SIZE"`
	SMTP            SMTPConfiguration        `json:"smtp"`
	Environment     string                   `json:"env" envconfig:"CONVOY_ENV"`
	MultipleTenants bool                     `json:"multiple_tenants"`
	Logger          LoggerConfiguration      `json:"logger"`
	Tracer          TracerConfiguration      `json:"tracer"`
	Cache           CacheConfiguration       `json:"cache"`
	Limiter         LimiterConfiguration     `json:"limiter"`
	Host            string                   `json:"host" envconfig:"CONVOY_HOST"`
	Search          SearchConfiguration      `json:"search"`
	FeatureFlag     FeatureFlagConfiguration `json:"feature_flag"`
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

// IsStringEmpty checks if the given string s is empty or not
func IsStringEmpty(s string) bool { return len(strings.TrimSpace(s)) == 0 }

func Override(newCfg *Configuration) error {
	c, err := Get()
	if err != nil {
		return err
	}

	ov := reflect.ValueOf(&c).Elem()
	nv := reflect.ValueOf(newCfg).Elem()

	for i := 0; i < ov.NumField(); i++ {
		if !ov.Field(i).CanInterface() {
			continue
		}

		fv := nv.Field(i).Interface()
		isZero := reflect.ValueOf(fv).IsZero()

		if isZero {
			continue
		}

		ov.Field(i).Set(reflect.ValueOf(fv))
	}

	cfgSingleton.Store(&c)
	return nil
}

// LoadConfig is used to load the configuration from either the json config file
// or the environment variables.
func LoadConfig(p string) error {
	c := DefaultConfiguration

	if _, err := os.Stat(p); err == nil {
		f, err := os.Open(p)
		if err != nil {
			return err
		}

		defer f.Close()

		// load config from config.json
		if err := json.NewDecoder(f).Decode(&c); err != nil {
			return err
		}
	} else if errors.Is(err, os.ErrNotExist) {
		log.Info("convoy.json not detected, will look for env vars or cli args")
	}

	// override config from environment variables
	err := envconfig.Process(envPrefix, &c)
	if err != nil {
		return err
	}

	if err = validate(&c); err != nil {
		return err
	}

	cfgSingleton.Store(&c)
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
		if queueCfg.Redis.Dsn == "" {
			return errors.New("redis queue dsn is empty")
		}

	default:
		return fmt.Errorf("unsupported queue type: %s", queueCfg.Type)
	}
	return nil
}

func ensureMaxResponseSize(c *Configuration) {
	bytes := c.MaxResponseSize * 1024

	if bytes == 0 {
		c.MaxResponseSize = MaxResponseSize
	} else if bytes > MaxResponseSize {
		log.Warnf("maximum response size of %dkb too large, using default value of %dkb", c.MaxResponseSize, c.MaxResponseSize/1024)
		c.MaxResponseSize = MaxResponseSize
	} else {
		c.MaxResponseSize = bytes
	}
}

func validate(c *Configuration) error {
	ensureMaxResponseSize(c)

	if err := ensureQueueConfig(c.Queue); err != nil {
		return err
	}

	if err := ensureSSL(c.Server); err != nil {
		return err
	}

	return nil
}
