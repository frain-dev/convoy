package config

import (
	"crypto/tls"
	"testing"
	"time"
)

func TestCertCache_GetSet(t *testing.T) {
	cache := GetCertCache()
	cache.Clear() // Start with clean cache

	// Generate test certificate valid for 1 hour
	notBefore := time.Now().Add(-1 * time.Minute)
	notAfter := time.Now().Add(1 * time.Hour)
	certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	// Parse certificate
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Test Set and Get
	key := "test-endpoint-1"
	cache.Set(key, &cert, notAfter)

	retrieved := cache.Get(key)
	if retrieved == nil {
		t.Fatal("Expected to retrieve cached certificate, got nil")
	}

	if len(retrieved.Certificate) != len(cert.Certificate) {
		t.Errorf("Certificate mismatch: expected %d certs, got %d", len(cert.Certificate), len(retrieved.Certificate))
	}
}

func TestCertCache_GetExpired(t *testing.T) {
	cache := GetCertCache()
	cache.Clear()

	// Generate test certificate that expires in the past
	notBefore := time.Now().Add(-2 * time.Hour)
	notAfter := time.Now().Add(-1 * time.Hour)
	certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Set expired certificate
	key := "test-endpoint-2"
	cache.Set(key, &cert, notAfter)

	// Try to retrieve - should return nil because it's expired
	retrieved := cache.Get(key)
	if retrieved != nil {
		t.Error("Expected nil for expired certificate, got a certificate")
	}
}

func TestCertCache_Delete(t *testing.T) {
	cache := GetCertCache()
	cache.Clear()

	notBefore := time.Now().Add(-1 * time.Minute)
	notAfter := time.Now().Add(1 * time.Hour)
	certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	key := "test-endpoint-3"
	cache.Set(key, &cert, notAfter)

	// Verify it's in cache
	if cache.Get(key) == nil {
		t.Fatal("Certificate should be in cache")
	}

	// Delete it
	cache.Delete(key)

	// Verify it's gone
	if cache.Get(key) != nil {
		t.Error("Certificate should be deleted from cache")
	}
}

func TestCertCache_Clear(t *testing.T) {
	cache := GetCertCache()
	cache.Clear()

	notBefore := time.Now().Add(-1 * time.Minute)
	notAfter := time.Now().Add(1 * time.Hour)
	certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Add multiple entries
	for i := 0; i < 5; i++ {
		key := "test-endpoint-" + string(rune(i))
		cache.Set(key, &cert, notAfter)
	}

	if cache.Size() != 5 {
		t.Errorf("Expected cache size 5, got %d", cache.Size())
	}

	// Clear cache
	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", cache.Size())
	}
}

func TestCertCache_Cleanup(t *testing.T) {
	cache := GetCertCache()
	cache.Clear()

	// Add expired certificate
	notBefore := time.Now().Add(-2 * time.Hour)
	notAfter := time.Now().Add(-1 * time.Hour)
	expiredCertPEM, expiredKeyPEM, err := generateTestCert(notBefore, notAfter)
	if err != nil {
		t.Fatalf("Failed to generate expired cert: %v", err)
	}

	expiredCert, err := tls.X509KeyPair([]byte(expiredCertPEM), []byte(expiredKeyPEM))
	if err != nil {
		t.Fatalf("Failed to parse expired certificate: %v", err)
	}

	cache.Set("expired-cert", &expiredCert, notAfter)

	// Add valid certificate
	validNotBefore := time.Now().Add(-1 * time.Minute)
	validNotAfter := time.Now().Add(1 * time.Hour)
	validCertPEM, validKeyPEM, err := generateTestCert(validNotBefore, validNotAfter)
	if err != nil {
		t.Fatalf("Failed to generate valid cert: %v", err)
	}

	validCert, err := tls.X509KeyPair([]byte(validCertPEM), []byte(validKeyPEM))
	if err != nil {
		t.Fatalf("Failed to parse valid certificate: %v", err)
	}

	cache.Set("valid-cert", &validCert, validNotAfter)

	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}

	// Run cleanup
	cache.cleanup()

	// Expired cert should be removed
	if cache.Size() != 1 {
		t.Errorf("Expected cache size 1 after cleanup, got %d", cache.Size())
	}

	// Valid cert should still be there
	if cache.Get("valid-cert") == nil {
		t.Error("Valid certificate should still be in cache")
	}

	// Expired cert should be gone
	if cache.Get("expired-cert") != nil {
		t.Error("Expired certificate should be removed from cache")
	}
}

func TestCertCache_ConcurrentAccess(t *testing.T) {
	cache := GetCertCache()
	cache.Clear()

	notBefore := time.Now().Add(-1 * time.Minute)
	notAfter := time.Now().Add(1 * time.Hour)
	certPEM, keyPEM, err := generateTestCert(notBefore, notAfter)
	if err != nil {
		t.Fatalf("Failed to generate test cert: %v", err)
	}

	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Test concurrent reads and writes
	done := make(chan bool)

	// Writers
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "concurrent-cert-" + string(rune(id))
			cache.Set(key, &cert, notAfter)
			done <- true
		}(i)
	}

	// Readers
	for i := 0; i < 10; i++ {
		go func(id int) {
			key := "concurrent-cert-" + string(rune(id))
			cache.Get(key) // May or may not find it, that's ok
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// No race conditions should occur (run with -race flag to verify)
}
