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
	"github.com/spf13/cobra"
)

const (
	MaxResponseSizeKb = 50                       // in kilobytes
	MaxResponseSize   = MaxResponseSizeKb * 1024 // in bytes
)

var cfgSingleton atomic.Value

type DatabaseConfiguration struct {
	Type DatabaseProvider `json:"type" envconfig:"CONVOY_DB_TYPE"`
	Dsn  string           `json:"dsn" envconfig:"CONVOY_DB_DSN"`
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
	WorkerPort  uint32 `json:"worker_port" envconfig:"WORKER_PORT"`
}

type QueueConfiguration struct {
	Type  QueueProvider           `json:"type" envconfig:"CONVOY_QUEUE_PROVIDER"`
	Redis RedisQueueConfiguration `json:"redis"`
}

type RedisQueueConfiguration struct {
	Dsn string `json:"dsn" envconfig:"CONVOY_REDIS_DSN"`
}

type FileRealmOption struct {
	Basic  BasicAuthConfig  `json:"basic" bson:"basic" envconfig:"CONVOY_BASIC_AUTH_CONFIG"`
	APIKey APIKeyAuthConfig `json:"api_key" envconfig:"CONVOY_API_KEY_CONFIG"`
}

type AuthConfiguration struct {
	RequireAuth bool               `json:"require_auth" envconfig:"CONVOY_REQUIRE_AUTH"`
	File        FileRealmOption    `json:"file"`
	Native      NativeRealmOptions `json:"native"`
}

type NativeRealmOptions struct {
	Enabled bool `json:"enabled" envconfig:"CONVOY_NATIVE_REALM_ENABLED"`
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

type ServerLogger struct {
	Level string `json:"level" envconfig:"CONVOY_LOGGER_LEVEL"`
}

type LoggerConfiguration struct {
	Type      LoggerProvider `json:"type" envconfig:"CONVOY_LOGGER_PROVIDER"`
	ServerLog ServerLogger   `json:"server_log"`
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

type Configuration struct {
	Auth            AuthConfiguration     `json:"auth,omitempty"`
	Database        DatabaseConfiguration `json:"database"`
	Sentry          SentryConfiguration   `json:"sentry"`
	Queue           QueueConfiguration    `json:"queue"`
	Server          ServerConfiguration   `json:"server"`
	MaxResponseSize uint64                `json:"max_response_size" envconfig:"CONVOY_MAX_RESPONSE_SIZE"`
	GroupConfig     GroupConfig           `json:"group"`
	SMTP            SMTPConfiguration     `json:"smtp"`
	Environment     string                `json:"env" envconfig:"CONVOY_ENV" required:"true" default:"development"`
	MultipleTenants bool                  `json:"multiple_tenants"`
	Logger          LoggerConfiguration   `json:"logger"`
	Tracer          TracerConfiguration   `json:"tracer"`
	Cache           CacheConfiguration    `json:"cache"`
	Limiter         LimiterConfiguration  `json:"limiter"`
	BaseUrl         string                `json:"base_url" envconfig:"CONVOY_BASE_URL"`
}

const (
	envPrefix string = "convoy"

	DevelopmentEnvironment string = "development"
)

const (
	RedisQueueProvider                 QueueProvider           = "redis"
	InMemoryQueueProvider              QueueProvider           = "in-memory"
	DefaultStrategyProvider            StrategyProvider        = "default"
	ExponentialBackoffStrategyProvider StrategyProvider        = "exponential-backoff"
	DefaultSignatureHeader             SignatureHeaderProvider = "X-Convoy-Signature"
	ConsoleLoggerProvider              LoggerProvider          = "console"
	NewRelicTracerProvider             TracerProvider          = "new_relic"
	RedisCacheProvider                 CacheProvider           = "redis"
	RedisLimiterProvider               LimiterProvider         = "redis"
	MongodbDatabaseProvider            DatabaseProvider        = "mongodb"
	InMemoryDatabaseProvider           DatabaseProvider        = "in-memory"
)

type GroupConfig struct {
	Strategy        StrategyConfiguration  `json:"strategy"`
	Signature       SignatureConfiguration `json:"signature"`
	DisableEndpoint bool                   `json:"disable_endpoint" envconfig:"CONVOY_DISABLE_ENDPOINT"`
	ReplayAttacks   bool                   `json:"replay_attacks" envconfig:"CONVOY_REPLAY_ATTACKS"`
}

type StrategyConfiguration struct {
	Type               StrategyProvider                        `json:"type" envconfig:"CONVOY_STRATEGY_TYPE"`
	Default            DefaultStrategyConfiguration            `json:"default"`
	ExponentialBackoff ExponentialBackoffStrategyConfiguration `json:"exponentialBackoff,omitempty"`
}

type DefaultStrategyConfiguration struct {
	IntervalSeconds uint64 `json:"intervalSeconds" envconfig:"CONVOY_INTERVAL_SECONDS"`
	RetryLimit      uint64 `json:"retryLimit" envconfig:"CONVOY_RETRY_LIMIT"`
}

type ExponentialBackoffStrategyConfiguration struct {
	RetryLimit uint64 `json:"retryLimit" envconfig:"CONVOY_RETRY_LIMIT"`
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
type LimiterProvider string
type DatabaseProvider string

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

// IsStringEmpty checks if the given string s is empty or not
func IsStringEmpty(s string) bool { return len(strings.TrimSpace(s)) == 0 }

func OverrideConfigWithCliFlags(cmd *cobra.Command, cfg *Configuration) error {
	// CONVOY_DB_DSN, CONVOY_DB_TYPE
	dbDsn, err := cmd.Flags().GetString("db")
	if err != nil {
		return err
	}

	if !IsStringEmpty(dbDsn) {
		cfg.Database.Type = InMemoryDatabaseProvider

		parts := strings.Split(dbDsn, "://")
		if len(parts) == 2 {
			// parts[0] must be either "mongodb" or "mongodb+srv"
			if parts[0] == string(MongodbDatabaseProvider) || parts[0] == string(MongodbDatabaseProvider)+"+srv" {
				cfg.Database.Type = MongodbDatabaseProvider
			}
		}

		cfg.Database.Dsn = dbDsn
	}

	// CONVOY_REDIS_DSN
	redisDsn, err := cmd.Flags().GetString("redis")
	if err != nil {
		return err
	}

	// CONVOY_QUEUE_PROVIDER
	queueDsn, err := cmd.Flags().GetString("queue")
	if err != nil {
		return err
	}

	if !IsStringEmpty(queueDsn) {
		cfg.Queue.Type = QueueProvider(queueDsn)
		if queueDsn == "redis" && !IsStringEmpty(redisDsn) {
			cfg.Queue.Redis.Dsn = redisDsn
		}
	}

	cfgSingleton.Store(cfg)

	return nil
}

func overrideConfigWithEnvVars(c *Configuration, override *Configuration) {
	// CONVOY_ENV
	if !IsStringEmpty(override.Environment) {
		c.Environment = override.Environment
	}

	// CONVOY_BASE_URL
	if !IsStringEmpty(override.BaseUrl) {
		c.BaseUrl = override.BaseUrl
	}

	// CONVOY_DB_TYPE
	if !IsStringEmpty(string(override.Database.Type)) {
		c.Database.Type = override.Database.Type
	}

	// CONVOY_DB_DSN
	if !IsStringEmpty(override.Database.Dsn) {
		c.Database.Dsn = override.Database.Dsn
	}

	// CONVOY_LIMITER_TYPE
	if !IsStringEmpty(override.Sentry.Dsn) {
		c.Sentry.Dsn = override.Sentry.Dsn
	}

	// CONVOY_LIMITER_TYPE
	if !IsStringEmpty(string(override.Limiter.Type)) {
		c.Limiter.Type = override.Limiter.Type
	}

	// CONVOY_REDIS_DSN
	if !IsStringEmpty(override.Limiter.Redis.Dsn) {
		c.Limiter.Redis.Dsn = override.Limiter.Redis.Dsn
	}

	// CONVOY_CACHE_PROVIDER
	if !IsStringEmpty(string(override.Cache.Type)) {
		c.Cache.Type = override.Cache.Type
	}

	// CONVOY_REDIS_DSN
	if !IsStringEmpty(override.Cache.Redis.Dsn) {
		c.Cache.Redis.Dsn = override.Cache.Redis.Dsn
	}

	// CONVOY_QUEUE_PROVIDER
	if !IsStringEmpty(string(override.Queue.Type)) {
		c.Queue.Type = override.Queue.Type
	}

	// CONVOY_REDIS_DSN
	if !IsStringEmpty(override.Queue.Redis.Dsn) {
		c.Queue.Redis.Dsn = override.Queue.Redis.Dsn
	}

	// CONVOY_REDIS_DSN
	if !IsStringEmpty(override.Queue.Redis.Dsn) {
		c.Queue.Redis.Dsn = override.Queue.Redis.Dsn
	}

	// CONVOY_LOGGER_PROVIDER
	if !IsStringEmpty(string(override.Logger.Type)) {
		c.Logger.Type = override.Logger.Type
	}

	// CONVOY_LOGGER_LEVEL
	if !IsStringEmpty(override.Logger.ServerLog.Level) {
		c.Logger.ServerLog.Level = override.Logger.ServerLog.Level
	}

	// PORT
	if override.Server.HTTP.Port != 0 {
		c.Server.HTTP.Port = override.Server.HTTP.Port
	}

	// WORKER_PORT
	if override.Server.HTTP.WorkerPort != 0 {
		c.Server.HTTP.WorkerPort = override.Server.HTTP.WorkerPort
	}

	// CONVOY_SSL_CERT_FILE
	if !IsStringEmpty(override.Server.HTTP.SSLCertFile) {
		c.Server.HTTP.SSLCertFile = override.Server.HTTP.SSLCertFile
	}

	// CONVOY_SSL_KEY_FILE
	if !IsStringEmpty(override.Server.HTTP.SSLKeyFile) {
		c.Server.HTTP.SSLKeyFile = override.Server.HTTP.SSLKeyFile
	}

	// CONVOY_STRATEGY_TYPE
	if !IsStringEmpty(string(override.GroupConfig.Strategy.Type)) {
		c.GroupConfig.Strategy.Type = override.GroupConfig.Strategy.Type
	}

	// CONVOY_SIGNATURE_HASH
	if !IsStringEmpty(override.GroupConfig.Signature.Hash) {
		c.GroupConfig.Signature.Hash = override.GroupConfig.Signature.Hash
	}

	// CONVOY_SIGNATURE_HEADER
	if !IsStringEmpty(string(override.GroupConfig.Signature.Header)) {
		c.GroupConfig.Signature.Header = override.GroupConfig.Signature.Header
	}

	// CONVOY_INTERVAL_SECONDS
	if override.GroupConfig.Strategy.Default.IntervalSeconds != 0 {
		c.GroupConfig.Strategy.Default.IntervalSeconds = override.GroupConfig.Strategy.Default.IntervalSeconds
	}

	// CONVOY_RETRY_LIMIT
	if override.GroupConfig.Strategy.Default.RetryLimit != 0 {
		c.GroupConfig.Strategy.Default.RetryLimit = override.GroupConfig.Strategy.Default.RetryLimit
	}

	// CONVOY_RETRY_LIMIT
	if override.GroupConfig.Strategy.ExponentialBackoff.RetryLimit != 0 {
		c.GroupConfig.Strategy.ExponentialBackoff.RetryLimit = override.GroupConfig.Strategy.ExponentialBackoff.RetryLimit
	}

	// CONVOY_SMTP_PROVIDER
	if !IsStringEmpty(override.SMTP.Provider) {
		c.SMTP.Provider = override.SMTP.Provider
	}

	// CONVOY_SMTP_FROM
	if !IsStringEmpty(override.SMTP.From) {
		c.SMTP.From = override.SMTP.From
	}

	// CONVOY_SMTP_PASSWORD
	if !IsStringEmpty(override.SMTP.Password) {
		c.SMTP.Password = override.SMTP.Password
	}

	// CONVOY_SMTP_REPLY_TO
	if !IsStringEmpty(override.SMTP.ReplyTo) {
		c.SMTP.ReplyTo = override.SMTP.ReplyTo
	}

	// CONVOY_SMTP_USERNAME
	if !IsStringEmpty(override.SMTP.URL) {
		c.SMTP.URL = override.SMTP.URL
	}

	// CONVOY_SMTP_USERNAME
	if !IsStringEmpty(override.SMTP.Username) {
		c.SMTP.Username = override.SMTP.Username
	}

	// CONVOY_SMTP_PORT
	if override.SMTP.Port != 0 {
		c.SMTP.Port = override.SMTP.Port
	}

	// CONVOY_MAX_RESPONSE_SIZE
	if override.MaxResponseSize != 0 {
		c.MaxResponseSize = override.MaxResponseSize
	}

	// CONVOY_NEWRELIC_APP_NAME
	if !IsStringEmpty(override.Tracer.NewRelic.AppName) {
		c.Tracer.NewRelic.AppName = override.Tracer.NewRelic.AppName
	}

	// CONVOY_NEWRELIC_LICENSE_KEY
	if !IsStringEmpty(override.Tracer.NewRelic.LicenseKey) {
		c.Tracer.NewRelic.LicenseKey = override.Tracer.NewRelic.LicenseKey
	}

	// CONVOY_API_KEY_CONFIG
	if override.Auth.File.APIKey != nil {
		c.Auth.File.APIKey = override.Auth.File.APIKey
	}

	// CONVOY_BASIC_AUTH_CONFIG
	if override.Auth.File.Basic != nil {
		c.Auth.File.Basic = override.Auth.File.Basic
	}

	// boolean values are weird; we have to check if they are actually set

	if _, ok := os.LookupEnv("CONVOY_MULTIPLE_TENANTS"); ok {
		c.MultipleTenants = override.MultipleTenants
	}

	if _, ok := os.LookupEnv("SSL"); ok {
		c.Server.HTTP.SSL = override.Server.HTTP.SSL
	}

	if _, ok := os.LookupEnv("CONVOY_DISABLE_ENDPOINT"); ok {
		c.GroupConfig.DisableEndpoint = override.GroupConfig.DisableEndpoint
	}

	if _, ok := os.LookupEnv("CONVOY_REPLAY_ATTACKS"); ok {
		c.GroupConfig.ReplayAttacks = override.GroupConfig.ReplayAttacks
	}

	if _, ok := os.LookupEnv("CONVOY_NEWRELIC_CONFIG_ENABLED"); ok {
		c.Tracer.NewRelic.ConfigEnabled = override.Tracer.NewRelic.ConfigEnabled
	}

	if _, ok := os.LookupEnv("CONVOY_REQUIRE_AUTH"); ok {
		c.Auth.RequireAuth = override.Auth.RequireAuth
	}

	if _, ok := os.LookupEnv("CONVOY_NATIVE_REALM_ENABLED"); ok {
		c.Auth.Native.Enabled = override.Auth.Native.Enabled
	}
}

// LoadConfig is used to load the configuration from either the json config file
// or the environment variables.
func LoadConfig(p string) error {
	c := &Configuration{}

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
		log.Info("convoy config.json not detected, will look for env vars or cli args")
	}

	ec := &Configuration{}

	// load config from environment variables
	err := envconfig.Process(envPrefix, ec)
	if err != nil {
		return err
	}

	overrideConfigWithEnvVars(c, ec)

	cfgSingleton.Store(c)
	return nil
}

func SetServerConfigDefaults(c *Configuration) error {
	// if it's still empty, set it to development
	if c.Environment == "" {
		c.Environment = DevelopmentEnvironment
	}

	if c.Server.HTTP.Port == 0 {
		return errors.New("http port cannot be zero")
	}

	err := ensureSSL(c.Server)
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
		log.Warnf("maximum response size of %dkb too large, using default value of %dkb", c.MaxResponseSize, c.MaxResponseSize/1024)
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
		if queueCfg.Redis.Dsn == "" {
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
	case ExponentialBackoffStrategyProvider:
		if strategyCfg.ExponentialBackoff.RetryLimit == 0 {
			return errors.New("retry limit is required for exponential backoff retry strategy configuration")
		}
	default:
		return fmt.Errorf("unsupported strategy type: %s", strategyCfg.Type)
	}
	return nil
}
