package config

import (
	"encoding/json"
	"errors"
	"fmt"
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
)

var cfgSingleton atomic.Value

var DefaultConfiguration = Configuration{
	Host:            DefaultHost,
	Environment:     OSSEnvironment,
	MaxResponseSize: MaxResponseSizeKb,

	Server: ServerConfiguration{
		HTTP: HTTPServerConfiguration{
			SSL:        false,
			Port:       5005,
			WorkerPort: 5006,
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
		authPart = fmt.Sprintf("%s:%s@", dc.Username, dc.Password)
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
	Type     TracerProvider        `json:"type" envconfig:"CONVOY_TRACER_PROVIDER"`
	NewRelic NewRelicConfiguration `json:"new_relic"`
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

type AnalyticsConfiguration struct {
	IsEnabled bool `json:"enabled" envconfig:"CONVOY_ANALYTICS_ENABLED"`
}

type StoragePolicyConfiguration struct {
	Type   string        `json:"type" envconfig:"CONVOY_STORAGE_POLICY_TYPE"`
	S3     S3Storage     `json:"s3"`
	OnPrem OnPremStorage `json:"on_prem"`
}

type S3Storage struct {
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

const (
	envPrefix      string = "convoy"
	OSSEnvironment string = "oss"
)

const (
	RedisQueueProvider       QueueProvider           = "redis"
	DefaultSignatureHeader   SignatureHeaderProvider = "X-Convoy-Signature"
	NewRelicTracerProvider   TracerProvider          = "new_relic"
	PostgresDatabaseProvider DatabaseProvider        = "postgres"
	TypesenseSearchProvider  SearchProvider          = "typesense"
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
	FeatureFlagProvider     string
)

func (s SignatureHeaderProvider) String() string {
	return string(s)
}

type Configuration struct {
	Auth               AuthConfiguration          `json:"auth,omitempty"`
	Database           DatabaseConfiguration      `json:"database"`
	Redis              RedisConfiguration         `json:"redis"`
	Prometheus         PrometheusConfiguration    `json:"prometheus"`
	Server             ServerConfiguration        `json:"server"`
	MaxResponseSize    uint64                     `json:"max_response_size" envconfig:"CONVOY_MAX_RESPONSE_SIZE"`
	SMTP               SMTPConfiguration          `json:"smtp"`
	Environment        string                     `json:"env" envconfig:"CONVOY_ENV"`
	Logger             LoggerConfiguration        `json:"logger"`
	Tracer             TracerConfiguration        `json:"tracer"`
	Host               string                     `json:"host" envconfig:"CONVOY_HOST"`
	CustomDomainSuffix string                     `json:"custom_domain_suffix" envconfig:"CONVOY_CUSTOM_DOMAIN_SUFFIX"`
	Search             SearchConfiguration        `json:"search"`
	FeatureFlag        FeatureFlagConfiguration   `json:"feature_flag"`
	Analytics          AnalyticsConfiguration     `json:"analytics"`
	StoragePolicy      StoragePolicyConfiguration `json:"storage_policy"`
	ConsumerPoolSize   int                        `json:"consumer_pool_size" envconfig:"CONVOY_CONSUMER_POOL_SIZE"`
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

	return nil
}
