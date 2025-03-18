package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
)

var caCertSingleton atomic.Value

func LoadCaCert(caCertString, caCertPath string) error {
	cfg, err := getCACertTLSCfg(caCertString, caCertPath)
	if err != nil {
		return err
	}
	caCertSingleton.Store(cfg)
	return nil
}

// getCACertTLSCfg returns a TLS configuration that includes both system CA certificates and optionally a custom CA certificate.
// It first loads the system certificates, then if a custom certificate is provided either via string or file path,
// it appends that to the certificate pool. This allows for verification of both public endpoints and custom CA signed endpoints.
func getCACertTLSCfg(caCertString, caCertPath string) (*tls.Config, error) {
	// Start with system certificates
	caCertPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to load system cert pool: %w", err)
	}

	// If no custom cert is provided, return config with system certs
	if caCertString == "" && caCertPath == "" {
		return &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
		}, nil
	}

	// Load custom CA cert if provided
	var caCertData []byte
	if caCertPath != "" {
		caCertData, err = os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
	}

	if caCertString != "" {
		caCertData = []byte(caCertString)
	}

	// Append custom cert to the pool
	if !caCertPool.AppendCertsFromPEM(caCertData) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	return &tls.Config{
		RootCAs:    caCertPool,
		MinVersion: tls.VersionTLS12,
	}, nil
}

// GetCaCert fetches the CaCert at runtime. LoadCaCert must have been called previously for this to work.
func GetCaCert() (*tls.Config, error) {
	cert, ok := caCertSingleton.Load().(*tls.Config)
	if !ok {
		return nil, errors.New("call Set CaCert before this function")
	}

	return cert, nil
}
