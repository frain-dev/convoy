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
	Type TracerProvider `json:"type" envconfig:"CONVOY_TRACER_PROVIDER"`
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
	MaxResponseSize uint64                `json:"max_response_size"`
	GroupConfig     GroupConfig           `json:"group"`
	SMTP            SMTPConfiguration     `json:"smtp"`
	Environment     string                `json:"env" envconfig:"CONVOY_ENV" required:"true" default:"development"`
	MultipleTenants bool                  `json:"multiple_tenants"`
	Logger          LoggerConfiguration   `json:"logger"`
	Tracer          TracerConfiguration   `json:"tracer"`
	NewRelic        NewRelicConfiguration `json:"new_relic"`
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
)

type GroupConfig struct {
	Strategy        StrategyConfiguration  `json:"strategy"`
	Signature       SignatureConfiguration `json:"signature"`
	DisableEndpoint bool                   `json:"disable_endpoint" envconfig:"CONVOY_DISABLE_ENDPOINT"`
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

func overrideConfigWithEnvVars(c *Configuration, override *Configuration) {
	// CONVOY_ENV
	if !IsStringEmpty(override.Environment) {
		c.Environment = override.Environment
	}

	// CONVOY_BASE_URL
	if !IsStringEmpty(override.BaseUrl) {
		c.BaseUrl = override.BaseUrl
	}

	// CONVOY_DB_DSN
	if !IsStringEmpty(override.Database.Type) {
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

	// CONVOY_NEWRELIC_APP_NAME
	if !IsStringEmpty(override.NewRelic.AppName) {
		c.NewRelic.AppName = override.NewRelic.AppName
	}

	// CONVOY_NEWRELIC_LICENSE_KEY
	if !IsStringEmpty(override.NewRelic.LicenseKey) {
		c.NewRelic.LicenseKey = override.NewRelic.LicenseKey
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

	if _, ok := os.LookupEnv("CONVOY_NEWRELIC_CONFIG_ENABLED"); ok {
		c.NewRelic.ConfigEnabled = override.NewRelic.ConfigEnabled
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
func LoadConfig(p string, override *Configuration) error {
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

func LoadConfigFromCliFlags(cmd *cobra.Command, c *Configuration) error {
	// CONVOY_ENV
	env, err := cmd.Flags().GetString("env")
	if err != nil {
		return err
	}

	if !IsStringEmpty(env) {
		c.Environment = env
	}

	// CONVOY_BASE_URL
	baseUrl, err := cmd.Flags().GetString("base-url")
	if err != nil {
		return err
	}

	if !IsStringEmpty(baseUrl) {
		c.BaseUrl = baseUrl
	}

	// CONVOY_DB_DSN, CONVOY_DB_TYPE
	db, err := cmd.Flags().GetString("db")
	if err != nil {
		return err
	}

	if !IsStringEmpty(db) {
		c.Database.Type = "in-memory"

		parts := strings.Split(db, "://")
		if len(parts) == 2 && parts[0] == "mongodb" {
			c.Database.Type = "mongodb"
		}

		c.Database.Dsn = db
	}

	// CONVOY_SENTRY_DSN
	sentryDsn, err := cmd.Flags().GetString("sentry")
	if err != nil {
		return err
	}

	if !IsStringEmpty(sentryDsn) {
		c.Sentry.Dsn = sentryDsn
	}

	// CONVOY_MULTIPLE_TENANTS
	isMTSet := cmd.Flags().Changed("multi-tenant")
	if isMTSet {
		multipleTenants, err := cmd.Flags().GetBool("multi-tenant")
		if err != nil {
			return err
		}

		c.MultipleTenants = multipleTenants
	}

	// CONVOY_LIMITER_PROVIDER
	rateLimiter, err := cmd.Flags().GetString("limiter")
	if err != nil {
		return err
	}

	if !IsStringEmpty(rateLimiter) {
		c.Limiter.Type = LimiterProvider(rateLimiter)
	}

	// CONVOY_CACHE_PROVIDER
	cache, err := cmd.Flags().GetString("cache")
	if err != nil {
		return err
	}

	if !IsStringEmpty(cache) {
		c.Cache.Type = CacheProvider(cache)
	}

	// CONVOY_QUEUE_PROVIDER
	queue, err := cmd.Flags().GetString("queue")
	if err != nil {
		return err
	}

	if !IsStringEmpty(queue) {
		c.Queue.Type = QueueProvider(queue)
	}

	// CONVOY_REDIS_DSN
	redis, err := cmd.Flags().GetString("redis")
	if err != nil {
		return err
	}

	if !IsStringEmpty(redis) {
		c.Queue.Redis.Dsn = redis
	}

	// CONVOY_LOGGER_LEVEL
	logLevel, err := cmd.Flags().GetString("log-level")
	if err != nil {
		return err
	}

	if !IsStringEmpty(logLevel) {
		c.Logger.ServerLog.Level = logLevel
	}

	// CONVOY_LOGGER_PROVIDER
	logger, err := cmd.Flags().GetString("logger")
	if err != nil {
		return err
	}

	if !IsStringEmpty(logger) {
		c.Logger.Type = LoggerProvider(logger)
	}

	// SSL
	isSslSet := cmd.Flags().Changed("ssl")
	if isSslSet {
		ssl, err := cmd.Flags().GetBool("ssl")
		if err != nil {
			return err
		}

		c.Server.HTTP.SSL = ssl
	}

	// PORT
	port, err := cmd.Flags().GetUint32("port")
	if err != nil {
		return err
	}

	if port != 0 {
		c.Server.HTTP.Port = port
	}

	// WORKER_PORT
	workerPort, err := cmd.Flags().GetUint32("worker-port")
	if err != nil {
		return err
	}

	if workerPort != 0 {
		c.Server.HTTP.WorkerPort = workerPort
	}

	// CONVOY_SSL_KEY_FILE
	sslKeyFile, err := cmd.Flags().GetString("ssl-key-file")
	if err != nil {
		return err
	}

	if !IsStringEmpty(sslKeyFile) {
		c.Server.HTTP.SSLKeyFile = sslKeyFile
	}

	// CONVOY_SSL_CERT_FILE
	sslCertFile, err := cmd.Flags().GetString("ssl-cert-file")
	if err != nil {
		return err
	}

	if !IsStringEmpty(sslCertFile) {
		c.Server.HTTP.SSLCertFile = sslCertFile
	}

	// CONVOY_STRATEGY_TYPE
	retryStrategy, err := cmd.Flags().GetString("retry-strategy")
	if err != nil {
		return err
	}

	if !IsStringEmpty(retryStrategy) {
		c.GroupConfig.Strategy.Type = StrategyProvider(retryStrategy)
	}

	// CONVOY_SIGNATURE_HASH
	signatureHash, err := cmd.Flags().GetString("signature-hash")
	if err != nil {
		return err
	}

	if !IsStringEmpty(signatureHash) {
		c.GroupConfig.Signature.Hash = signatureHash
	}

	// CONVOY_SIGNATURE_HEADER
	signatureHeader, err := cmd.Flags().GetString("signature-header")
	if err != nil {
		return err
	}

	if !IsStringEmpty(signatureHeader) {
		c.GroupConfig.Signature.Header = SignatureHeaderProvider(signatureHeader)
	}

	// CONVOY_DISABLE_ENDPOINT
	isDESet := cmd.Flags().Changed("disable-endpoint")
	if isDESet {
		disableEndpoint, err := cmd.Flags().GetBool("disable-endpoint")
		if err != nil {
			return err
		}

		c.GroupConfig.DisableEndpoint = disableEndpoint
	}

	// CONVOY_INTERVAL_SECONDS
	retryInterval, err := cmd.Flags().GetUint64("retry-interval")
	if err != nil {
		return err
	}

	if retryInterval != 0 {
		c.GroupConfig.Strategy.Default.IntervalSeconds = retryInterval
	}

	// CONVOY_RETRY_LIMIT
	retryLimit, err := cmd.Flags().GetUint64("retry-limit")
	if err != nil {
		return err
	}
	if retryLimit != 0 {
		c.GroupConfig.Strategy.Default.RetryLimit = retryLimit
	}

	// CONVOY_SMTP_PROVIDER
	smtpProvider, err := cmd.Flags().GetString("smtp-provider")
	if err != nil {
		return err
	}

	if !IsStringEmpty(smtpProvider) {
		c.SMTP.Provider = smtpProvider
	}

	// CONVOY_SMTP_URL
	smtpUrl, err := cmd.Flags().GetString("smtp-url")
	if err != nil {
		return err
	}

	if !IsStringEmpty(smtpUrl) {
		c.SMTP.URL = smtpUrl
	}

	// CONVOY_SMTP_USERNAME
	smtpUsername, err := cmd.Flags().GetString("smtp-username")
	if err != nil {
		return err
	}

	if !IsStringEmpty(smtpUsername) {
		c.SMTP.Username = smtpUsername
	}

	// CONVOY_SMTP_PASSWORD
	smtpPassword, err := cmd.Flags().GetString("smtp-password")
	if err != nil {
		return err
	}

	if !IsStringEmpty(smtpPassword) {
		c.SMTP.Password = smtpPassword
	}

	// CONVOY_SMTP_FROM
	smtpFrom, err := cmd.Flags().GetString("smtp-from")
	if err != nil {
		return err
	}

	if !IsStringEmpty(smtpFrom) {
		c.SMTP.From = smtpFrom
	}

	// CONVOY_SMTP_REPLY_TO
	smtpReplyTo, err := cmd.Flags().GetString("smtp-reply-to")
	if err != nil {
		return err
	}

	if !IsStringEmpty(smtpReplyTo) {
		c.SMTP.ReplyTo = smtpReplyTo
	}

	// CONVOY_SMTP_PORT
	smtpPort, err := cmd.Flags().GetUint32("smtp-port")
	if err != nil {
		return err
	}
	if smtpPort != 0 {
		c.SMTP.Port = smtpPort
	}

	// CONVOY_NEWRELIC_APP_NAME
	newReplicApp, err := cmd.Flags().GetString("new-relic-app")
	if err != nil {
		return err
	}

	if !IsStringEmpty(newReplicApp) {
		c.NewRelic.AppName = newReplicApp
	}

	// CONVOY_NEWRELIC_LICENSE_KEY
	newReplicKey, err := cmd.Flags().GetString("new-relic-key")
	if err != nil {
		return err
	}

	if !IsStringEmpty(newReplicKey) {
		c.NewRelic.AppName = newReplicKey
	}

	// CONVOY_NEWRELIC_CONFIG_ENABLED
	isNRCESet := cmd.Flags().Changed("new-relic-config-enabled")
	if isNRCESet {
		newReplicConfigEnabled, err := cmd.Flags().GetBool("new-relic-config-enabled")
		if err != nil {
			return err
		}

		c.NewRelic.ConfigEnabled = newReplicConfigEnabled
	}

	// CONVOY_NEWRELIC_DISTRIBUTED_TRACER_ENABLED
	isNRTESet := cmd.Flags().Changed("new-relic-tracer-enabled")
	if isNRTESet {
		newReplicTracerEnabled, err := cmd.Flags().GetBool("new-relic-tracer-enabled")
		if err != nil {
			return err
		}

		c.NewRelic.DistributedTracerEnabled = newReplicTracerEnabled
	}

	// CONVOY_REQUIRE_AUTH
	isReqAuthSet := cmd.Flags().Changed("auth")
	if isReqAuthSet {
		requireAuth, err := cmd.Flags().GetBool("auth")
		if err != nil {
			return err
		}

		c.Auth.RequireAuth = requireAuth
	}

	// CONVOY_NATIVE_REALM_ENABLED
	isNativeRealmSet := cmd.Flags().Changed("native")
	if isNativeRealmSet {
		nativeRealmEnabled, err := cmd.Flags().GetBool("native")
		if err != nil {
			return err
		}

		c.Auth.Native.Enabled = nativeRealmEnabled
	}

	// CONVOY_API_KEY_CONFIG
	apiKeyAuthConfig, err := cmd.Flags().GetString("api-auth")
	if err != nil {
		return err
	}

	if !IsStringEmpty(apiKeyAuthConfig) {
		config := APIKeyAuthConfig{}
		err = config.Decode(apiKeyAuthConfig)
		if err != nil {
			return err
		}

		c.Auth.File.APIKey = config
	}

	// CONVOY_BASIC_AUTH_CONFIG
	basicAuthConfig, err := cmd.Flags().GetString("basic-auth")
	if err != nil {
		return err
	}

	if !IsStringEmpty(basicAuthConfig) {
		config := BasicAuthConfig{}
		err = config.Decode(basicAuthConfig)
		if err != nil {
			return err
		}

		c.Auth.File.Basic = config
	}

	return nil
}
