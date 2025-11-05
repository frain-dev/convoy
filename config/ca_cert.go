package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"
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

// LoadClientCertificate loads a client certificate and key from PEM strings.
// It returns a tls.Certificate that can be used for mTLS client authentication.
// It validates that the certificate and key match, and checks for certificate expiration.
// This function does not use caching.
func LoadClientCertificate(certString, keyString string) (*tls.Certificate, error) {
	if certString == "" {
		return nil, errors.New("client certificate must be provided")
	}

	if keyString == "" {
		return nil, errors.New("client key must be provided")
	}

	// Parse the certificate and key
	// This function validates that the cert and key correspond to each other
	cert, err := tls.X509KeyPair([]byte(certString), []byte(keyString))
	if err != nil {
		return nil, fmt.Errorf("failed to parse client certificate and key: %w", err)
	}

	// Parse the X.509 certificate to check expiration
	if len(cert.Certificate) > 0 {
		x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			return nil, fmt.Errorf("failed to parse X.509 certificate: %w", err)
		}

		// Check if certificate has expired
		now := time.Now()
		if now.Before(x509Cert.NotBefore) {
			return nil, fmt.Errorf("certificate is not yet valid (valid from %s)", x509Cert.NotBefore.Format(time.RFC3339))
		}

		if now.After(x509Cert.NotAfter) {
			return nil, fmt.Errorf("certificate has expired (expired on %s)", x509Cert.NotAfter.Format(time.RFC3339))
		}
	}

	return &cert, nil
}

// LoadClientCertificateWithCache loads a client certificate with caching support.
// It first checks the cache for a valid parsed certificate. If not found or expired,
// it parses the certificate, validates it, and stores it in the cache.
// The cacheKey should be a unique identifier for the certificate (e.g., endpoint ID).
func LoadClientCertificateWithCache(cacheKey, certString, keyString string) (*tls.Certificate, error) {
	cache := GetCertCache()

	// Try to get from cache first
	if cachedCert := cache.Get(cacheKey); cachedCert != nil {
		return cachedCert, nil
	}

	// Not in cache or expired, load and validate
	cert, err := LoadClientCertificate(certString, keyString)
	if err != nil {
		return nil, err
	}

	// Determine expiration time for cache
	var expiresAt time.Time
	if len(cert.Certificate) > 0 {
		x509Cert, parseErr := x509.ParseCertificate(cert.Certificate[0])
		if parseErr == nil {
			expiresAt = x509Cert.NotAfter
		}
	}

	// If we couldn't determine expiration, cache for 1 hour
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(1 * time.Hour)
	}

	// Store in cache
	cache.Set(cacheKey, cert, expiresAt)

	return cert, nil
}
