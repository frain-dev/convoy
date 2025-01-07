package circuit_breaker

import (
	"sync"
	"time"
)

type TenantCache struct {
	cache       map[string]CircuitBreakerConfig // key is TenantId
	mu          sync.Mutex
	lastChecked time.Time
}

func NewTenantCache() *TenantCache {
	return &TenantCache{
		cache:       make(map[string]CircuitBreakerConfig),
		lastChecked: time.Time{},
	}
}

func (tc *TenantCache) GetConfig(tenantId string) (*CircuitBreakerConfig, bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	config, exists := tc.cache[tenantId]
	return &config, exists
}

func (tc *TenantCache) UpdateConfigs(configs map[string]CircuitBreakerConfig) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	for tenantId, config := range configs {
		tc.cache[tenantId] = config
	}
}
