package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/kelseyhightower/envconfig"
)

const (
	MaxResponseSizeKb                 = 50    // in kilobytes
	MaxResponseSize                   = 51200 // in bytes
	DefaultHost                       = "localhost:5005"
	DefaultSearchTokenizationInterval = 1
	DefaultCacheTTL                   = time.Minute * 10
	DefaultAPIVersion                 = "2024-04-01"
)

var cfgSingleton atomic.Value

var DefaultConfiguration = Configuration{
	APIVersion:      DefaultAPIVersion,
	Host:            DefaultHost,
	Environment:     OSSEnvironment,
	MaxResponseSize: MaxResponseSizeKb,

	Server: ServerConfiguration{
		HTTP: HTTPServerConfiguration{
			SSL:        false,
			Port:       5005,
			WorkerPort: 5006,
			AgentPort:  5008,
			IngestPort: 5009,
		},
	},
	Database: DatabaseConfiguration{
		Type:               PostgresDatabaseProvider,
		Scheme:             "postgres",
		Host:               "localhost",
		Username:           "postgres",
		Password:           "postgres",
		Database:           "convoy",
		Options:            "sslmode=disable&connect_timeout=30",
		Port:               5432,
		SetConnMaxLifetime: 3600,
	},
	Redis: RedisConfiguration{
		Scheme: "redis",
		Host:   "localhost",
		Port:   6379,
	},
	Logger: LoggerConfiguration{
		Level: "error",
	},
	Analytics: AnalyticsConfiguration{
		IsEnabled: true,
	},
	StoragePolicy: StoragePolicyConfiguration{
		Type: "on-prem",
		OnPrem: OnPremStorage{
			Path: convoy.DefaultOnPremDir,
		},
	},
	RetentionPolicy: RetentionPolicyConfiguration{
		Policy:                   "720h",
		IsRetentionPolicyEnabled: false,
	},
	CircuitBreaker: CircuitBreakerConfiguration{
		SampleRate:                  30,
		ErrorTimeout:                30,
		FailureThreshold:            70,
		FailureCount:                10,
		SuccessThreshold:            5,
		ObservabilityWindow:         5,
		MinimumRequestCount:         10,
		NotificationThresholds:      [3]uint64{10, 30, 50},
		ConsecutiveFailureThreshold: 10,
	},
	Auth: AuthConfiguration{
		IsSignupEnabled: true,
		Native: NativeRealmOptions{
			Enabled: true,
		},
		Jwt: JwtRealmOptions{
			Enabled: true,
		},
	},
	ConsumerPoolSize: 100,
	Tracer: TracerConfiguration{
		OTel: OTelConfiguration{
			SampleRate:         1.0,
			InsecureSkipVerify: true,
		},
	},
	EnableProfiling: false,
	Metrics: MetricsConfiguration{
		IsEnabled: false,
		Backend:   PrometheusMetricsProvider,
		Prometheus: PrometheusMetricsConfiguration{
			SampleTime: 5,
		},
	},
	InstanceIngestRate:  25,
	WorkerExecutionMode: DefaultExecutionMode,
}

type DatabaseConfiguration struct {
	Type DatabaseProvider `json:"type" envconfig:"CONVOY_DB_TYPE"`

	Scheme   string `json:"scheme" envconfig:"CONVOY_DB_SCHEME"`
	Host     string `json:"host" envconfig:"CONVOY_DB_HOST"`
	Username string `json:"username" envconfig:"CONVOY_DB_USERNAME"`
	Password string `json:"password" envconfig:"CONVOY_DB_PASSWORD"`
	Database string `json:"database" envconfig:"CONVOY_DB_DATABASE"`
	Options  string `json:"options" envconfig:"CONVOY_DB_OPTIONS"`
	Port     int    `json:"port" envconfig:"CONVOY_DB_PORT"`

	SetMaxOpenConnections int `json:"max_open_conn" envconfig:"CONVOY_DB_MAX_OPEN_CONN"`
	SetMaxIdleConnections int `json:"max_idle_conn" envconfig:"CONVOY_DB_MAX_IDLE_CONN"`
	SetConnMaxLifetime    int `json:"conn_max_lifetime" envconfig:"CONVOY_DB_CONN_MAX_LIFETIME"`
}

func (dc DatabaseConfiguration) BuildDsn() string {
	if dc.Scheme == "" {
		return ""
	}

	authPart := ""
	if dc.Username != "" || dc.Password != "" {
		authPrefix := url.UserPassword(dc.Username, dc.Password)
		authPart = fmt.Sprintf("%s@", authPrefix)
	}

	dbPart := ""
	if dc.Database != "" {
		dbPart = fmt.Sprintf("/%s", dc.Database)
	}

	optPart := ""
	if dc.Options != "" {
		optPart = fmt.Sprintf("?%s", dc.Options)
	}

	return fmt.Sprintf("%s://%s%s:%d%s%s", dc.Scheme, authPart, dc.Host, dc.Port, dbPart, optPart)
}

type ServerConfiguration struct {
	HTTP HTTPServerConfiguration `json:"http"`
}

type HTTPServerConfiguration struct {
	SSL         bool   `json:"ssl" envconfig:"SSL"`
	SSLCertFile string `json:"ssl_cert_file" envconfig:"CONVOY_SSL_CERT_FILE"`
	SSLKeyFile  string `json:"ssl_key_file" envconfig:"CONVOY_SSL_KEY_FILE"`
	Port        uint32 `json:"port" envconfig:"PORT"`
	AgentPort   uint32 `json:"agent_port" envconfig:"AGENT_PORT"`
	IngestPort  uint32 `json:"ingest_port" envconfig:"INGEST_PORT"`
	WorkerPort  uint32 `json:"worker_port" envconfig:"WORKER_PORT"`
	SocketPort  uint32 `json:"socket_port" envconfig:"SOCKET_PORT"`
	DomainPort  uint32 `json:"domain_port" envconfig:"DOMAIN_PORT"`
	HttpProxy   string `json:"proxy" envconfig:"HTTP_PROXY"`
}

type PrometheusConfiguration struct {
	Dsn string `json:"dsn" envconfig:"CONVOY_PROM_DSN"`
}

type RedisConfiguration struct {
	Scheme    string `json:"scheme" envconfig:"CONVOY_REDIS_SCHEME"`
	Host      string `json:"host" envconfig:"CONVOY_REDIS_HOST"`
	Username  string `json:"username" envconfig:"CONVOY_REDIS_USERNAME"`
	Password  string `json:"password" envconfig:"CONVOY_REDIS_PASSWORD"`
	Database  string `json:"database" envconfig:"CONVOY_REDIS_DATABASE"`
	Port      int    `json:"port" envconfig:"CONVOY_REDIS_PORT"`
	Addresses string `json:"addresses" envconfig:"CONVOY_REDIS_CLUSTER_ADDRESSES"`
}

func (rc RedisConfiguration) BuildDsn() []string {
	if len(strings.TrimSpace(rc.Addresses)) != 0 {
		return strings.Split(rc.Addresses, ",")
	}

	if rc.Scheme == "" {
		return []string{}
	}

	authPart := ""
	if rc.Username != "" || rc.Password != "" {
		authPart = fmt.Sprintf("%s:%s@", rc.Username, rc.Password)
	}

	dbPart := ""
	if rc.Database != "" {
		dbPart = fmt.Sprintf("/%s", rc.Database)
	}

	return []string{fmt.Sprintf("%s://%s%s:%d%s", rc.Scheme, authPart, rc.Host, rc.Port, dbPart)}
}

type FileRealmOption struct {
	Basic  BasicAuthConfig  `json:"basic" bson:"basic" envconfig:"CONVOY_BASIC_AUTH_CONFIG"`
	APIKey APIKeyAuthConfig `json:"api_key" envconfig:"CONVOY_API_KEY_CONFIG"`
}

type AuthConfiguration struct {
	File            FileRealmOption    `json:"file"`
	Native          NativeRealmOptions `json:"native"`
	Jwt             JwtRealmOptions    `json:"jwt"`
	IsSignupEnabled bool               `json:"is_signup_enabled" envconfig:"CONVOY_SIGNUP_ENABLED"`
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
	SSL      bool   `json:"ssl" envconfig:"CONVOY_SMTP_SSL"`
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
	Type   TracerProvider      `json:"type" envconfig:"CONVOY_TRACER_PROVIDER"`
	OTel   OTelConfiguration   `json:"otel"`
	Sentry SentryConfiguration `json:"sentry"`
}

type OTelConfiguration struct {
	OTelAuth           OTelAuthConfiguration `json:"otel_auth"`
	SampleRate         float64               `json:"sample_rate" envconfig:"CONVOY_OTEL_SAMPLE_RATE"`
	CollectorURL       string                `json:"collector_url" envconfig:"CONVOY_OTEL_COLLECTOR_URL"`
	InsecureSkipVerify bool                  `json:"insecure_skip_verify" envconfig:"CONVOY_OTEL_INSECURE_SKIP_VERIFY"`
}

type OTelAuthConfiguration struct {
	HeaderName  string `json:"header_name" envconfig:"CONVOY_OTEL_AUTH_HEADER_NAME"`
	HeaderValue string `json:"header_value" envconfig:"CONVOY_OTEL_AUTH_HEADER_VALUE"`
}

type SentryConfiguration struct {
	DSN string `json:"dsn" envconfig:"CONVOY_SENTRY_DSN"`
}

type RetentionPolicyConfiguration struct {
	Policy                   string `json:"policy" envconfig:"CONVOY_RETENTION_POLICY"`
	IsRetentionPolicyEnabled bool   `json:"enabled" envconfig:"CONVOY_RETENTION_POLICY_ENABLED"`
}

type CircuitBreakerConfiguration struct {
	SampleRate                  uint64    `json:"sample_rate" envconfig:"CONVOY_CIRCUIT_BREAKER_SAMPLE_RATE"`
	FailureCount                uint64    `json:"failure_count" envconfig:"CONVOY_CIRCUIT_BREAKER_ERROR_COUNT"`
	ErrorTimeout                uint64    `json:"error_timeout" envconfig:"CONVOY_CIRCUIT_BREAKER_ERROR_TIMEOUT"`
	FailureThreshold            uint64    `json:"failure_threshold" envconfig:"CONVOY_CIRCUIT_BREAKER_FAILURE_THRESHOLD"`
	SuccessThreshold            uint64    `json:"success_threshold" envconfig:"CONVOY_CIRCUIT_BREAKER_SUCCESS_THRESHOLD"`
	MinimumRequestCount         uint64    `json:"minimum_request_count" envconfig:"CONVOY_MINIMUM_REQUEST_COUNT"`
	ObservabilityWindow         uint64    `json:"observability_window" envconfig:"CONVOY_CIRCUIT_BREAKER_OBSERVABILITY_WINDOW"`
	NotificationThresholds      [3]uint64 `json:"notification_thresholds" envconfig:"CONVOY_CIRCUIT_BREAKER_NOTIFICATION_THRESHOLDS"`
	ConsecutiveFailureThreshold uint64    `json:"consecutive_failure_threshold" envconfig:"CONVOY_CIRCUIT_BREAKER_CONSECUTIVE_FAILURE_THRESHOLD"`
}

type AnalyticsConfiguration struct {
	IsEnabled bool `json:"enabled" envconfig:"CONVOY_ANALYTICS_ENABLED"`
}

type StoragePolicyConfiguration struct {
	Type   string        `json:"type" envconfig:"CONVOY_STORAGE_POLICY_TYPE"`
	S3     S3Storage     `json:"s3"`
	OnPrem OnPremStorage `json:"on_prem"`
}

type S3Storage struct {
	Prefix       string `json:"prefix" envconfig:"CONVOY_STORAGE_AWS_PREFIX"`
	Bucket       string `json:"bucket" envconfig:"CONVOY_STORAGE_AWS_BUCKET"`
	AccessKey    string `json:"access_key" envconfig:"CONVOY_STORAGE_AWS_ACCESS_KEY"`
	SecretKey    string `json:"secret_key" envconfig:"CONVOY_STORAGE_AWS_SECRET_KEY"`
	Region       string `json:"region" envconfig:"CONVOY_STORAGE_AWS_REGION"`
	SessionToken string `json:"session_token" envconfig:"CONVOY_STORAGE_AWS_SESSION_TOKEN"`
	Endpoint     string `json:"endpoint" envconfig:"CONVOY_STORAGE_AWS_ENDPOINT"`
}

type OnPremStorage struct {
	Path string `json:"path" envconfig:"CONVOY_STORAGE_PREM_PATH"`
}

type MetricsConfiguration struct {
	IsEnabled  bool                           `json:"enabled" envconfig:"CONVOY_METRICS_ENABLED"`
	Backend    MetricsBackend                 `json:"metrics_backend" envconfig:"CONVOY_METRICS_BACKEND"`
	Prometheus PrometheusMetricsConfiguration `json:"prometheus_metrics"`
}

type PrometheusMetricsConfiguration struct {
	SampleTime uint64 `json:"sample_time" envconfig:"CONVOY_METRICS_SAMPLE_TIME"`
}

const (
	envPrefix      string = "convoy"
	OSSEnvironment string = "oss"
)

const (
	OTelTracerProvider   TracerProvider = "otel"
	SentryTracerProvider TracerProvider = "sentry"
)

const (
	RedisQueueProvider       QueueProvider           = "redis"
	DefaultSignatureHeader   SignatureHeaderProvider = "X-Convoy-Signature"
	PostgresDatabaseProvider DatabaseProvider        = "postgres"
)

const (
	PrometheusMetricsProvider MetricsBackend = "prometheus"
)

type (
	AuthProvider            string
	QueueProvider           string
	SignatureHeaderProvider string
	TracerProvider          string
	CacheProvider           string
	LimiterProvider         string
	DatabaseProvider        string
	SearchProvider          string
	MetricsBackend          string
)

func (s SignatureHeaderProvider) String() string {
	return string(s)
}

type ExecutionMode string

const (
	EventsExecutionMode  ExecutionMode = "events"
	RetryExecutionMode   ExecutionMode = "retry"
	DefaultExecutionMode ExecutionMode = "default"
)

type Configuration struct {
	InstanceId          string                       `json:"instance_id"`
	APIVersion          string                       `json:"api_version" envconfig:"CONVOY_API_VERSION"`
	Auth                AuthConfiguration            `json:"auth,omitempty"`
	Database            DatabaseConfiguration        `json:"database"`
	Redis               RedisConfiguration           `json:"redis"`
	Prometheus          PrometheusConfiguration      `json:"prometheus"`
	Server              ServerConfiguration          `json:"server"`
	MaxResponseSize     uint64                       `json:"max_response_size" envconfig:"CONVOY_MAX_RESPONSE_SIZE"`
	SMTP                SMTPConfiguration            `json:"smtp"`
	Environment         string                       `json:"env" envconfig:"CONVOY_ENV"`
	Logger              LoggerConfiguration          `json:"logger"`
	Tracer              TracerConfiguration          `json:"tracer"`
	Host                string                       `json:"host" envconfig:"CONVOY_HOST"`
	Pyroscope           PyroscopeConfiguration       `json:"pyroscope"`
	CustomDomainSuffix  string                       `json:"custom_domain_suffix" envconfig:"CONVOY_CUSTOM_DOMAIN_SUFFIX"`
	EnableFeatureFlag   []string                     `json:"enable_feature_flag" envconfig:"CONVOY_ENABLE_FEATURE_FLAG"`
	RetentionPolicy     RetentionPolicyConfiguration `json:"retention_policy"`
	CircuitBreaker      CircuitBreakerConfiguration  `json:"circuit_breaker"`
	Analytics           AnalyticsConfiguration       `json:"analytics"`
	StoragePolicy       StoragePolicyConfiguration   `json:"storage_policy"`
	ConsumerPoolSize    int                          `json:"consumer_pool_size" envconfig:"CONVOY_CONSUMER_POOL_SIZE"`
	EnableProfiling     bool                         `json:"enable_profiling" envconfig:"CONVOY_ENABLE_PROFILING"`
	Metrics             MetricsConfiguration         `json:"metrics" envconfig:"CONVOY_METRICS"`
	InstanceIngestRate  int                          `json:"instance_ingest_rate" envconfig:"CONVOY_INSTANCE_INGEST_RATE"`
	WorkerExecutionMode ExecutionMode                `json:"worker_execution_mode" envconfig:"CONVOY_WORKER_EXECUTION_MODE"`
	MaxRetrySeconds     uint64                       `json:"max_retry_seconds,omitempty" envconfig:"CONVOY_MAX_RETRY_SECONDS"`
	LicenseKey          string                       `json:"license_key" envconfig:"CONVOY_LICENSE_KEY"`
}

type PyroscopeConfiguration struct {
	EnableProfiling bool   `json:"enabled" envconfig:"CONVOY_ENABLE_PYROSCOPE_PROFILING"`
	URL             string `json:"url" envconfig:"CONVOY_PYROSCOPE_URL"`
	Username        string `json:"username" envconfig:"CONVOY_PYROSCOPE_USERNAME"`
	Password        string `json:"password" envconfig:"CONVOY_PYROSCOPE_PASSWORD"`
	ProfileID       string `json:"profile_id" envconfig:"CONVOY_PYROSCOPE_PROFILE_ID"`
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

	overrideFields(ov, nv)

	cfgSingleton.Store(&c)
	return nil
}

func overrideFields(ov, nv reflect.Value) {
	for i := 0; i < ov.NumField(); i++ {
		ovField := ov.Field(i)
		if !ovField.CanInterface() {
			continue
		}

		nvField := nv.Field(i)

		if nvField.Kind() == reflect.Struct {
			overrideFields(ovField, nvField)
		} else {
			fv := nvField.Interface()
			isZero := reflect.ValueOf(fv).IsZero()

			if isZero {
				continue
			}

			ovField.Set(reflect.ValueOf(fv))
		}
	}
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
	} else if !errors.Is(err, os.ErrNotExist) {
		log.WithError(err).Fatal("failed to check if config file exists")
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

func ensureQueueConfig(queueCfg RedisConfiguration) error {
	if len(queueCfg.BuildDsn()) == 0 {
		return errors.New("redis queue dsn is empty")
	}

	return nil
}

func ensureMaxResponseSize(c *Configuration) {
	bytes := c.MaxResponseSize * 1024

	if bytes == 0 {
		c.MaxResponseSize = MaxResponseSize
	} else {
		c.MaxResponseSize = bytes
	}
}

func validate(c *Configuration) error {
	ensureMaxResponseSize(c)

	if err := ensureQueueConfig(c.Redis); err != nil {
		return err
	}

	if err := ensureSSL(c.Server); err != nil {
		return err
	}

	if c.Metrics.IsEnabled {
		backend := c.Metrics.Backend
		switch backend {
		case PrometheusMetricsProvider:
			break
		default:
			c.Metrics.IsEnabled = false
		}
	}

	return nil
}
