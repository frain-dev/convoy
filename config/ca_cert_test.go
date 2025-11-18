package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Helper function to generate a test certificate
func generateTestCert(notBefore, notAfter time.Time) (certPEM, keyPEM string, err error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "test.example.com",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", err
	}

	// Encode certificate to PEM
	certPEMBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	// Encode private key to PEM
	keyPEMBlock := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return string(certPEMBlock), string(keyPEMBlock), nil
}

func TestGetCACertTLSCfg_CustomCA(t *testing.T) {
	caCert := `-----BEGIN CERTIFICATE-----
MIIFtzCCA5+gAwIBAgIUIYdQv74LF6tTl3VXgrlOtmP4ISIwDQYJKoZIhvcNAQEL
BQAwazELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcM
DVNhbiBGcmFuY2lzY28xFzAVBgNVBAoMDlByaXZhdGVDQSBJbmMuMRYwFAYDVQQD
DA1NeSBQcml2YXRlIENBMB4XDTI1MDMxNTE4MzMxNVoXDTM1MDMxMzE4MzMxNVow
azELMAkGA1UEBhMCVVMxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDVNh
biBGcmFuY2lzY28xFzAVBgNVBAoMDlByaXZhdGVDQSBJbmMuMRYwFAYDVQQDDA1N
eSBQcml2YXRlIENBMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEA334N
ELHlK/HeaG/lIu7cnsxKEmJO21QPzrMkLPiMn3dIDwEs4bAeD36la69YdsZ+Siii
A3thiv+NG+prLtgRyn+zIvaTlBi+DZ8ItyCbKaYxMHx4LIjW0LE3T91UufnK1cCJ
6Qir6Yk18q/sYmoaOQvXVrC4y08dqpSJBYDTc9VSPUFHHeyL/DWdAIRpUcRYIZhE
a7wp7p3LOfuBkYfrSd2uj5feEcr26ghQvzraz1pTexhgrmqA6Onu1FN2YeUP+RkP
Oig0DLs3G0yYF3gLdyorwyVkYW5eMSD3DTM7ogYUFt2AZ6rBbIYQm9JmOtKZT9Wh
8axzQj6vWCGp6mDc84cOkZFlIpkyhUABVcXEdIWvWZmrMFJojYbYLYuVn/KIjoZW
C23x4DvvCxWF52FkyQ4o4bECX7/C888f/DhJYHQ+ZiGZkcZTygqiF+9gK3cq04Kf
/y0LvjY09XZp5SGanSmTTnoKIl2o/v9fNr/rQn1BrJgUoUF4zGG95C3Jv3VFOQxP
XcJ7LDmYcXS3LFt3v/rF8m6kNIkivX4xKVI1buHjRdhm/maJkI7rUqUXVMq/fCmI
31d4wdScSsiKcIthXGdDUn0WqRyc3w4QI+H6lTgk+mVAaBoWzB+lKqf89jatKS2J
nVzglbKFOmQTLibojsaNLHLYN8vIE0dDnVAaT3cCAwEAAaNTMFEwHQYDVR0OBBYE
FP/gxFHW1XFWYSSPphU/Oymj30PWMB8GA1UdIwQYMBaAFP/gxFHW1XFWYSSPphU/
Oymj30PWMA8GA1UdEwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggIBADDcQpxJ
VPWWlIFEXW1SXbq3Cy1w6NBuc7hmGhBB7FJVcbIpkuz6PiWyXBneHFAf0g0kB68g
NEpoiLVtbnA1VzMGLffex1fWrZRDV8I/70hNKc+p4vvbaOHusMMkGGgNjiPIV6k8
3J6eL+X72nsY8TwP+W4OzrQH+H4xTOZr+5tZYSa28a6pa56m983zvhQpfivIsS8g
G0Jz7ixS/cdUEnrdvvcTHdk5QWZYx2NFW48/Uzp7u6eFnRf7cpC7UzdsYARp0/2M
P1v8qLkSreXqky860wYimd2WhFSvJ9n55w0jdaksIXzGJ5oy58Bht+80c/cvac32
O2unLOsgIAFyfIymTzAf/1Vu1w63Ls0py80/Vz/dse8sSAaHw4PF3UynWZprxzVl
a2pU8O0hpxlRRnk8UrVPgFqV51qifnHun8tz0aJi6rlq7sUusouw7OZUdblrDGDe
yuNz+YfuKIxrPE0KYYROEjsJHXHNVuFESaJT++LyfGcbRvouSsjHCdUaLEkGpFHD
DUQIoqXl4rlrvAaB3jn1P9Wh7uER/8+N9AUCk/cTv9rh15l8gogtNeWgHkeyTLIs
xcu+6WRAnK9eu0vSM7zMM3y8b0pph1UhgbQTow3NajP4u5HVoIzKjfD/Mc2pMIKR
gNzJgq2rBh+ytZgv31JGEcG/DwfPrC7eANAy
-----END CERTIFICATE-----
`

	t.Run("should load custom CA from string", func(t *testing.T) {
		tlsCfg, err := getCACertTLSCfg(caCert, "")
		require.NoError(t, err)
		require.NotNil(t, tlsCfg)
		require.NotNil(t, tlsCfg.RootCAs)
	})

	t.Run("should load custom CA from file", func(t *testing.T) {
		tmpFile, err := os.CreateTemp("", "ca-cert-*.pem")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		_, err = tmpFile.Write([]byte(caCert))
		require.NoError(t, err)
		tmpFile.Close()

		tlsCfg, err := getCACertTLSCfg("", tmpFile.Name())
		require.NoError(t, err)
		require.NotNil(t, tlsCfg)
		require.NotNil(t, tlsCfg.RootCAs)
	})

	t.Run("should return nil if no CA is provided", func(t *testing.T) {
		tlsCfg, err := getCACertTLSCfg("", "")
		require.NoError(t, err)

		systemCertPool, err := x509.SystemCertPool()
		require.NoError(t, err)

		systemTlsCfg := &tls.Config{
			RootCAs:    systemCertPool,
			MinVersion: tls.VersionTLS12,
		}

		require.Equal(t, len(systemTlsCfg.Certificates), len(tlsCfg.Certificates))
	})
}

func TestLoadClientCertificate(t *testing.T) {
	// Test certificate and key (self-signed for testing)
	clientCert := `-----BEGIN CERTIFICATE-----
MIIDXjCCAkagAwIBAgIBATANBgkqhkiG9w0BAQsFADBTMQswCQYDVQQGEwJVUzEO
MAwGA1UECAwFU3RhdGUxDTALBgNVBAcMBENpdHkxETAPBgNVBAoMCENsaWVudENB
MRIwEAYDVQQDDAlDbGllbnQtQ0EwHhcNMjUxMDI0MDc1MjM4WhcNMjgwNzIwMDc1
MjM4WjBOMQswCQYDVQQGEwJVUzEOMAwGA1UECAwFU3RhdGUxDTALBgNVBAcMBENp
dHkxDzANBgNVBAoMBkNsaWVudDEPMA0GA1UEAwwGY2xpZW50MIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEA9Bv0GzAzt8ijkjlVP+E66KaNk0f67T5UFiiT
ij4w9hPOzRPlyXhjsixlqNqkm5ASbycWKhHP67SO7Xn+IeKEXdk/N0BHR0pNlh9k
lXpetKnzvrSwm6ldPD9OrXxjYqouvQpEJ/pkKZsUaH5S5Si6tW+KqczPN9JerjIU
OTAPwDr7KN/MwF+Q2de+7UaZ7Chja41NB0lCIQAU18jGtqQpMISNtA2O3YcaXY8J
0DNdjh/yczu7ii3VKvzFNHDGUbkC7VXJbLziGxCFjDBev9IhMxzmpfQS8IsMWeic
iOBD/8Be9ENW0I2YEZfvMubH/rvJPgxIMSgq9jIE1LKuPAGRQwIDAQABo0IwQDAd
BgNVHQ4EFgQUZFs54K2y3wBzj8S8g9aN2ERcElAwHwYDVR0jBBgwFoAUBhhmTRZN
fHlRYCwezjxoQMKQxgswDQYJKoZIhvcNAQELBQADggEBAGV4PSkztoi1vd26oruO
4b11Ylrx0qON9nXj0RJpARoGr3NY674jBITe8ZhSQUc178z08BeaBD1s9joXsMx4
pmWCJSPLWL/h7d5VcT3x+HxOFXgek1q/L4CzbkExPkzu2655JzYcsI18KWSziZ5i
gJDE82c2rYBwMzbKW5yZERPib/EDJP5I1FckApNZepHIp0zaxdgbsSj72nq7YWEs
cGNwawU8GNLRl4b7a87FDoJj5UG9Yh3CRQejz7CVNsmney0bhmNmoB7T4W5NzUsP
S8+eiZZouOkyMjDYK+piAfzKSttLOW2jbFDASqp4EGXzKqG4tM8oVXYsIWfIReRu
ldg=
-----END CERTIFICATE-----`

	clientKey := `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQD0G/QbMDO3yKOS
OVU/4Tropo2TR/rtPlQWKJOKPjD2E87NE+XJeGOyLGWo2qSbkBJvJxYqEc/rtI7t
ef4h4oRd2T83QEdHSk2WH2SVel60qfO+tLCbqV08P06tfGNiqi69CkQn+mQpmxRo
flLlKLq1b4qpzM830l6uMhQ5MA/AOvso38zAX5DZ177tRpnsKGNrjU0HSUIhABTX
yMa2pCkwhI20DY7dhxpdjwnQM12OH/JzO7uKLdUq/MU0cMZRuQLtVclsvOIbEIWM
MF6/0iEzHOal9BLwiwxZ6JyI4EP/wF70Q1bQjZgRl+8y5sf+u8k+DEgxKCr2MgTU
sq48AZFDAgMBAAECggEAB38vE7SuKe520Fm3Aga424Z3iGnoZwFuhxDijLDU3Rts
bJG4Kv22n7UYWPazoalrjE+/F2l21FTPvOa6hMmwA5fVhqz2ydXNpMojBl1jOJlg
yHiqu3Hlajr0suqqiYNGrmL5BxQVoAEVjKrKGr3E+iewsph9I3twKyZgYGwKJJhu
9nrCccyDZHkOjW0KfaL2ppP6WRSSMl4LotBJnc8C//dDxX+zZkYoksII6jRzvPM2
CeVxXILa43AP5rifguG+/wTyjP4PG2c42Ra+Ac4DzHkMwQrwa0gIHQYswN5CRU3G
6Aha8KFt1jjwdEm4muV8Db8ZyeWneUz1mWZdRw/emQKBgQD+2U2zRpb1WNOE9j4w
a9/TgHih1+OYUdFkV9u/4Zc1oCEhjBTUzpnxGlhAlco6Cjw0RjGmrYQ9gCscPdh/
Oz8dPfZZcxSCuw4PFYjGu8OOoYNNeLfj6V8aqAFhROxICkL5EkRv5mZ8YWaPOqIn
MbEBcSaezdk7cVPBbLFUH/2DaQKBgQD1NjssV/fzE+EUeIp6E75LA06nAX2xX1L2
2uDMt/IGEscZLjUhcYS+M+LkuNtW2Yjgy/fzAlw1bMFjHIH9TYsLqAc7u3WcNAXW
7L3DksBPhvknjoZ+i8nkaLFeUG3XnLQaI3drk/vhL5q+dR+KK9PALXeixjHmvFum
Ry57kgTVywKBgQD9u5ky3xs5l4CxJwHv79dfms+AQ5QkeYGC6D6wIokMKSwTXIb5
AeIfPN2VIA3CD6K1YRXaH3RETzGc4q6ErpY+JQz7LirDpj1vIz+Urikb/w7duU1N
K3M29QK6t4aQizb3CQr+ZmSvfcJA5F3BrCXRi7ip78VS+5gqQm+jlF4x0QKBgGMN
AgAalLzq9cuYGY/Qc9jHQDkz3/sLH2854P6w+yG66hPg13Nn8JAIU4nCpk9B1gnA
Oqs989Nc2A1aEaQpc5ZEzI8zXQG4/fbgcJMUr3wwcGqrJubtPqN2KteHM6eZ1CKO
2wlooKFI4oA2vYPJymJhu2bUGooy4e6b6EngJPXbAoGBAJG3A9IYS01CIEe3Aqch
JvevQh041JhSVv78fVtY0YJNE3WZQ5M1GM0PLHIKRJ54DqFq979XVhzSw/t4TeNk
POZjvwZTtrr1jOLClXXnNaM9y/Fo+fVcdEU1M2yEITJOxPfEmejB/4Qeji2ARKtm
C6azzwqUOSsfDcuAS5sfJp/6
-----END PRIVATE KEY-----`

	t.Run("should load client certificate from strings", func(t *testing.T) {
		cert, err := LoadClientCertificate(clientCert, clientKey)
		require.NoError(t, err)
		require.NotNil(t, cert)
		require.NotEmpty(t, cert.Certificate)
		require.NotNil(t, cert.PrivateKey)
	})

	t.Run("should return error if no cert provided", func(t *testing.T) {
		cert, err := LoadClientCertificate("", "")
		require.Error(t, err)
		require.Nil(t, cert)
		require.Contains(t, err.Error(), "client certificate must be provided")
	})

	t.Run("should return error if no key provided", func(t *testing.T) {
		cert, err := LoadClientCertificate(clientCert, "")
		require.Error(t, err)
		require.Nil(t, cert)
		require.Contains(t, err.Error(), "client key must be provided")
	})

	t.Run("should return error for invalid cert/key pair", func(t *testing.T) {
		invalidCert := "invalid certificate"
		cert, err := LoadClientCertificate(invalidCert, clientKey)
		require.Error(t, err)
		require.Nil(t, cert)
		require.Contains(t, err.Error(), "failed to parse client certificate and key")
	})

	t.Run("should reject mismatched certificate and private key", func(t *testing.T) {
		// Generate two different cert/key pairs
		notBefore := time.Now().Add(-1 * time.Hour)
		notAfter := time.Now().Add(24 * time.Hour)

		// First cert/key pair
		_, key1PEM, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		// Second cert/key pair (different keys, different cert)
		cert2PEM, _, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		// Try to use cert2 with key1 (mismatched) - should fail
		// This proves that tls.X509KeyPair validates the cert and key correspond to each other
		cert, err := LoadClientCertificate(cert2PEM, key1PEM)
		require.Error(t, err)
		require.Nil(t, cert)
		require.Contains(t, err.Error(), "failed to parse client certificate and key")
		// The underlying error from tls.X509KeyPair indicates the public key in cert2
		// does not match the private key key1
	})
}

func TestLoadClientCertificate_Expiration(t *testing.T) {
	t.Run("should reject expired certificate", func(t *testing.T) {
		// Generate certificate that expired yesterday
		notBefore := time.Now().Add(-48 * time.Hour)
		notAfter := time.Now().Add(-24 * time.Hour)
		certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		cert, err := LoadClientCertificate(certPEM, keyPEM)
		require.Error(t, err)
		require.Nil(t, cert)
		require.Contains(t, err.Error(), "certificate has expired")
	})

	t.Run("should reject not-yet-valid certificate", func(t *testing.T) {
		// Generate certificate valid starting tomorrow
		notBefore := time.Now().Add(24 * time.Hour)
		notAfter := time.Now().Add(48 * time.Hour)
		certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		cert, err := LoadClientCertificate(certPEM, keyPEM)
		require.Error(t, err)
		require.Nil(t, cert)
		require.Contains(t, err.Error(), "certificate is not yet valid")
	})

	t.Run("should accept valid certificate", func(t *testing.T) {
		// Generate certificate valid now
		notBefore := time.Now().Add(-1 * time.Minute)
		notAfter := time.Now().Add(1 * time.Hour)
		certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		cert, err := LoadClientCertificate(certPEM, keyPEM)
		require.NoError(t, err)
		require.NotNil(t, cert)
	})
}

func TestLoadClientCertificateWithCache(t *testing.T) {
	cache := GetCertCache()
	cache.Clear()

	t.Run("should cache certificate on first load", func(t *testing.T) {
		notBefore := time.Now().Add(-1 * time.Minute)
		notAfter := time.Now().Add(1 * time.Hour)
		certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		cacheKey := "endpoint-123"

		// First load - should parse and cache
		cert1, err := LoadClientCertificateWithCache(cacheKey, certPEM, keyPEM)
		require.NoError(t, err)
		require.NotNil(t, cert1)

		// Verify it's in cache
		cachedCert := cache.Get(cacheKey)
		require.NotNil(t, cachedCert)

		// Second load - should return cached version
		cert2, err := LoadClientCertificateWithCache(cacheKey, certPEM, keyPEM)
		require.NoError(t, err)
		require.NotNil(t, cert2)

		// Should be the same instance (pointer equality)
		require.Equal(t, cert1, cert2)
	})

	t.Run("should not cache invalid certificate", func(t *testing.T) {
		cache.Clear()
		invalidCert := "invalid"
		invalidKey := "invalid"
		cacheKey := "endpoint-invalid"

		cert, err := LoadClientCertificateWithCache(cacheKey, invalidCert, invalidKey)
		require.Error(t, err)
		require.Nil(t, cert)

		// Should not be in cache
		cachedCert := cache.Get(cacheKey)
		require.Nil(t, cachedCert)
	})

	t.Run("should not cache expired certificate", func(t *testing.T) {
		cache.Clear()
		notBefore := time.Now().Add(-2 * time.Hour)
		notAfter := time.Now().Add(-1 * time.Hour)
		certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		cacheKey := "endpoint-expired"

		cert, err := LoadClientCertificateWithCache(cacheKey, certPEM, keyPEM)
		require.Error(t, err)
		require.Nil(t, cert)
		require.Contains(t, err.Error(), "expired")

		// Should not be in cache
		cachedCert := cache.Get(cacheKey)
		require.Nil(t, cachedCert)
	})

	t.Run("should use different cache entries for different keys", func(t *testing.T) {
		cache.Clear()

		// Generate two different certificates
		notBefore := time.Now().Add(-1 * time.Minute)
		notAfter := time.Now().Add(1 * time.Hour)

		cert1PEM, key1PEM, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		cert2PEM, key2PEM, err := generateTestCert(notBefore, notAfter)
		require.NoError(t, err)

		// Load with different keys
		loadedCert1, err := LoadClientCertificateWithCache("endpoint-1", cert1PEM, key1PEM)
		require.NoError(t, err)

		loadedCert2, err := LoadClientCertificateWithCache("endpoint-2", cert2PEM, key2PEM)
		require.NoError(t, err)

		// Should be different certificates
		require.NotEqual(t, loadedCert1, loadedCert2)

		// Both should be cached
		require.NotNil(t, cache.Get("endpoint-1"))
		require.NotNil(t, cache.Get("endpoint-2"))
		require.Equal(t, 2, cache.Size())
	})
}
