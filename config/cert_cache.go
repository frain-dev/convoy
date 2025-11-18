package config

import (
	"crypto/tls"
	"sync"
	"time"
)

// CertCacheEntry represents a cached certificate with its parsed form and expiration time
type CertCacheEntry struct {
	Cert      *tls.Certificate
	ExpiresAt time.Time
	CachedAt  time.Time
}

// CertCache provides a thread-safe cache for parsed client certificates
type CertCache struct {
	mu    sync.RWMutex
	cache map[string]*CertCacheEntry
}

var (
	certCacheInstance *CertCache
	certCacheOnce     sync.Once
)

// GetCertCache returns the singleton certificate cache instance
func GetCertCache() *CertCache {
	certCacheOnce.Do(func() {
		certCacheInstance = &CertCache{
			cache: make(map[string]*CertCacheEntry),
		}
		// Start background cleanup goroutine
		go certCacheInstance.cleanupLoop()
	})
	return certCacheInstance
}

// Get retrieves a cached certificate by its cache key
// Returns nil if not found or if the entry has expired
func (cc *CertCache) Get(key string) *tls.Certificate {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	entry, exists := cc.cache[key]
	if !exists {
		return nil
	}

	// Check if certificate has expired
	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	return entry.Cert
}

// Set stores a parsed certificate in the cache with the given key
func (cc *CertCache) Set(key string, cert *tls.Certificate, expiresAt time.Time) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache[key] = &CertCacheEntry{
		Cert:      cert,
		ExpiresAt: expiresAt,
		CachedAt:  time.Now(),
	}
}

// Delete removes a certificate from the cache
func (cc *CertCache) Delete(key string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	delete(cc.cache, key)
}

// Clear removes all entries from the cache
func (cc *CertCache) Clear() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache = make(map[string]*CertCacheEntry)
}

// cleanupLoop runs periodically to remove expired certificates from the cache
func (cc *CertCache) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cc.cleanup()
	}
}

// cleanup removes expired entries from the cache
func (cc *CertCache) cleanup() {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	for key, entry := range cc.cache {
		if now.After(entry.ExpiresAt) {
			delete(cc.cache, key)
		}
	}
}

// Size returns the number of entries in the cache
func (cc *CertCache) Size() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return len(cc.cache)
}
