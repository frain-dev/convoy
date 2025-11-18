package rdb

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/util"
)

// Redis is our wrapper logic to instrument redis calls
type Redis struct {
	addresses []string
	client    redis.UniversalClient
}

// TLSConfig holds TLS configuration options for Redis
type TLSConfig struct {
	SkipVerify bool
	CACertFile string
	CertFile   string
	KeyFile    string
}

// NewClient is used to create new Redis type. This type
// encapsulates our interaction with redis and provides instrumentation with new relic.
func NewClient(addresses []string) (*Redis, error) {
	return NewClientWithTLS(addresses, nil)
}

// NewClientWithTLS creates a new Redis client with optional TLS configuration
func NewClientWithTLS(addresses []string, tlsConfig *TLSConfig) (*Redis, error) {
	if len(addresses) == 0 {
		return nil, errors.New("redis addresses list cannot be empty")
	}

	for _, dsn := range addresses {
		if util.IsStringEmpty(dsn) {
			return nil, errors.New("dsn cannot be empty")
		}
	}

	var client redis.UniversalClient

	if len(addresses) == 1 {
		opts, err := redis.ParseURL(addresses[0])
		if err != nil {
			return nil, err
		}

		// Apply TLS configuration if provided (for rediss:// URLs)
		if tlsConfig != nil {
			// Preserve the original ServerName from ParseURL
			var serverName string
			if opts.TLSConfig != nil {
				serverName = opts.TLSConfig.ServerName
			}

			tlsCfg, err := buildTLSConfig(tlsConfig)
			if err != nil {
				return nil, err
			}

			// Restore ServerName if it was set
			if serverName != "" && tlsCfg.ServerName == "" {
				tlsCfg.ServerName = serverName
			}

			opts.TLSConfig = tlsCfg
		}

		client = redis.NewClient(opts)
	} else {
		tlsCfg := &tls.Config{}

		// Apply TLS configuration if provided
		if tlsConfig != nil {
			var err error
			tlsCfg, err = buildTLSConfig(tlsConfig)
			if err != nil {
				return nil, err
			}
		}

		client = redis.NewUniversalClient(&redis.UniversalOptions{
			TLSConfig: tlsCfg,
			Addrs:     addresses,
		})
	}

	// Enable tracing instrumentation.
	if err := redisotel.InstrumentTracing(client); err != nil {
		return nil, err
	}

	return &Redis{addresses: addresses, client: client}, nil
}

func buildTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.SkipVerify,
	}

	// Load CA certificate if provided
	if cfg.CACertFile != "" {
		caCert, err := os.ReadFile(cfg.CACertFile)
		if err != nil {
			return nil, errors.New("failed to read CA certificate: " + err.Error())
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate and key if provided
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, errors.New("failed to load client certificate: " + err.Error())
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// NewClientFromConfig creates a Redis client from a RedisConfiguration
// This is a convenience function that builds DSN and TLS config from the configuration
func NewClientFromConfig(addresses []string, tlsSkipVerify bool, caCertFile, certFile, keyFile string) (*Redis, error) {
	var tlsConfig *TLSConfig

	// Only create TLS config if TLS options are provided
	if tlsSkipVerify || caCertFile != "" || (certFile != "" && keyFile != "") {
		tlsConfig = &TLSConfig{
			SkipVerify: tlsSkipVerify,
			CACertFile: caCertFile,
			CertFile:   certFile,
			KeyFile:    keyFile,
		}
	}

	return NewClientWithTLS(addresses, tlsConfig)
}

// Client is to return underlying redis interface
func (r *Redis) Client() redis.UniversalClient {
	return r.client
}

// MakeRedisClient is used to fulfill asynq's interface
func (r *Redis) MakeRedisClient() interface{} {
	return r.client
}
