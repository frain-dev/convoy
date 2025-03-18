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

// getCACertTLSCfg returns a TLS configuration that includes a custom CA certificate.
// It first tries to load the certificate from the provided string (caCertString).
// If the string is empty, it attempts to read the certificate from the specified file path (caCertPath).
// If no valid certificate is provided, it returns nil.
// The function ensures that the loaded CA certificate is appended to a certificate pool used for TLS verification.
func getCACertTLSCfg(caCertString, caCertPath string) (*tls.Config, error) {
	var caCertData []byte

	if caCertString != "" {
		caCertData = []byte(caCertString)
	} else if caCertPath != "" {
		var err error
		caCertData, err = os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}
	}

	if len(caCertData) == 0 {
		return nil, nil
	}

	caCertPool := x509.NewCertPool()
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
