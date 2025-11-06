package net

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TestDispatcherMTLSIntegration tests the complete mTLS transport path:
// - Creates a test HTTPS server that requires and validates client certificates
// - Tests successful mTLS connection with valid client cert
// - Tests failure when no client cert is provided
// - Tests failure when invalid client cert is provided
func TestDispatcherMTLSIntegration(t *testing.T) {
	// Generate test CA certificate
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2025),
		Subject: pkix.Name{
			Organization: []string{"Convoy Test CA"},
			CommonName:   "Test CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	// Generate CA private key
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Self-sign the CA certificate
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	// Parse the CA certificate
	caCert, err := x509.ParseCertificate(caBytes)
	require.NoError(t, err)

	// Generate client certificate signed by CA
	clientCertTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2026),
		Subject: pkix.Name{
			Organization: []string{"Convoy Test Client"},
			CommonName:   "Test Client",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// Generate client private key
	clientPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Sign the client certificate with CA
	clientCertBytes, err := x509.CreateCertificate(rand.Reader, clientCertTemplate, caCert, &clientPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	// Convert to PEM format
	clientCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: clientCertBytes,
	})

	clientKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(clientPrivKey),
	})

	// Create CA cert pool for server to verify client certs
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	// Track whether the server received a valid client certificate
	receivedValidClientCert := false

	t.Run("should successfully connect with valid mTLS client certificate", func(t *testing.T) {
		receivedValidClientCert = false

		// Create HTTPS server that requires client certificates
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify client certificate was provided
			if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
				receivedValidClientCert = true
				// Verify it's the expected client cert
				clientCert := r.TLS.PeerCertificates[0]
				require.Equal(t, "Test Client", clientCert.Subject.CommonName)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"success"}`))
		}))

		// Configure server to require and verify client certificates
		server.TLS = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  caCertPool,
			MinVersion: tls.VersionTLS12,
		}
		server.StartTLS()
		defer server.Close()

		// Setup dispatcher
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		licenser := mocks.NewMockLicenser(ctrl)
		licenser.EXPECT().IpRules().AnyTimes().Return(false)

		// Configure dispatcher with InsecureSkipVerify to accept the test server's self-signed cert
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}

		dispatcher, err := NewDispatcher(
			licenser,
			fflag.NewFFlag([]string{}),
			LoggerOption(log.NewLogger(os.Stdout)),
			TLSConfigOption(true, licenser, tlsConfig),
		)
		require.NoError(t, err)

		// Load client certificate
		clientCert, err := config.LoadClientCertificate(string(clientCertPEM), string(clientKeyPEM))
		require.NoError(t, err)
		require.NotNil(t, clientCert)

		// Send request with mTLS
		resp, err := dispatcher.SendWebhookWithMTLS(
			context.Background(),
			server.URL,
			[]byte(`{"event":"test"}`),
			"X-Convoy-Signature",
			"test-signature",
			1024*1024,
			nil,
			"test-idempotency-key",
			10*time.Second,
			"application/json",
			clientCert,
		)

		// Verify successful request
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Contains(t, string(resp.Body), "success")

		// Verify server received and validated the client certificate
		require.True(t, receivedValidClientCert, "Server did not receive valid client certificate")
	})

	t.Run("should fail when server requires mTLS but no client cert provided", func(t *testing.T) {
		// Create HTTPS server that requires client certificates
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called when client cert is missing")
		}))

		server.TLS = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  caCertPool,
			MinVersion: tls.VersionTLS12,
		}
		server.StartTLS()
		defer server.Close()

		// Setup dispatcher
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		licenser := mocks.NewMockLicenser(ctrl)
		licenser.EXPECT().IpRules().AnyTimes().Return(false)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}

		dispatcher, err := NewDispatcher(
			licenser,
			fflag.NewFFlag([]string{}),
			LoggerOption(log.NewLogger(os.Stdout)),
			TLSConfigOption(true, licenser, tlsConfig),
		)
		require.NoError(t, err)

		// Send request WITHOUT client certificate
		resp, err := dispatcher.SendWebhookWithMTLS(
			context.Background(),
			server.URL,
			[]byte(`{"event":"test"}`),
			"X-Convoy-Signature",
			"test-signature",
			1024*1024,
			nil,
			"test-idempotency-key",
			10*time.Second,
			"application/json",
			nil, // No client certificate
		)

		// Should fail with TLS handshake error
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t,
			strings.Contains(errMsg, "tls") ||
				strings.Contains(errMsg, "connection reset") ||
				strings.Contains(errMsg, "closed network connection"),
			"expected TLS/connection error, got: %s", errMsg)
		if resp != nil {
			require.NotEqual(t, http.StatusOK, resp.StatusCode)
		}
	})

	t.Run("should fail with invalid/untrusted client certificate", func(t *testing.T) {
		// Create HTTPS server that requires client certificates
		server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("Handler should not be called with invalid client cert")
		}))

		server.TLS = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  caCertPool,
			MinVersion: tls.VersionTLS12,
		}
		server.StartTLS()
		defer server.Close()

		// Generate a different client cert NOT signed by our CA
		wrongClientCert := &x509.Certificate{
			SerialNumber: big.NewInt(3000),
			Subject: pkix.Name{
				Organization: []string{"Wrong Org"},
				CommonName:   "Wrong Client",
			},
			NotBefore:   time.Now(),
			NotAfter:    time.Now().Add(24 * time.Hour),
			ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			KeyUsage:    x509.KeyUsageDigitalSignature,
		}

		wrongPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)

		// Self-sign (not signed by our CA)
		wrongCertBytes, err := x509.CreateCertificate(rand.Reader, wrongClientCert, wrongClientCert, &wrongPrivKey.PublicKey, wrongPrivKey)
		require.NoError(t, err)

		wrongCertPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: wrongCertBytes,
		})

		wrongKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(wrongPrivKey),
		})

		// Setup dispatcher
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		licenser := mocks.NewMockLicenser(ctrl)
		licenser.EXPECT().IpRules().AnyTimes().Return(false)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}

		dispatcher, err := NewDispatcher(
			licenser,
			fflag.NewFFlag([]string{}),
			LoggerOption(log.NewLogger(os.Stdout)),
			TLSConfigOption(true, licenser, tlsConfig),
		)
		require.NoError(t, err)

		// Load the wrong client certificate
		wrongCert, err := config.LoadClientCertificate(string(wrongCertPEM), string(wrongKeyPEM))
		require.NoError(t, err)

		// Attempt to send request with wrong certificate
		resp, err := dispatcher.SendWebhookWithMTLS(
			context.Background(),
			server.URL,
			[]byte(`{"event":"test"}`),
			"X-Convoy-Signature",
			"test-signature",
			1024*1024,
			nil,
			"test-idempotency-key",
			10*time.Second,
			"application/json",
			wrongCert,
		)

		// Should fail - certificate not trusted by server
		// Note: Error can be "tls", "connection reset", or "closed network connection"
		// depending on platform and timing - all indicate certificate rejection
		require.Error(t, err)
		errMsg := err.Error()
		require.True(t,
			strings.Contains(errMsg, "tls") ||
				strings.Contains(errMsg, "connection reset") ||
				strings.Contains(errMsg, "closed network connection"),
			"expected TLS/connection error, got: %s", errMsg)
		if resp != nil {
			require.NotEqual(t, http.StatusOK, resp.StatusCode)
		}
	})

	t.Run("should work with SendWebhook when no mTLS cert provided (backward compatibility)", func(t *testing.T) {
		// Create normal HTTPS server (no client cert required)
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		licenser := mocks.NewMockLicenser(ctrl)
		licenser.EXPECT().IpRules().AnyTimes().Return(false)

		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			MinVersion:         tls.VersionTLS12,
		}

		dispatcher, err := NewDispatcher(
			licenser,
			fflag.NewFFlag([]string{}),
			LoggerOption(log.NewLogger(os.Stdout)),
			TLSConfigOption(true, licenser, tlsConfig),
		)
		require.NoError(t, err)

		// Use regular SendWebhook (no mTLS)
		resp, err := dispatcher.SendWebhook(
			context.Background(),
			server.URL,
			[]byte(`{"event":"test"}`),
			"X-Convoy-Signature",
			"test-signature",
			1024*1024,
			nil,
			"test-idempotency-key",
			10*time.Second,
			"application/json",
		)

		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
