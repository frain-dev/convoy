package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

func TestGetQueueOptions(t *testing.T) {
	t.Run("Standard Redis Configuration", func(t *testing.T) {
		cfg := &config.Configuration{
			Redis: config.RedisConfiguration{
				Scheme: "redis",
				Host:   "localhost",
				Port:   6379,
			},
		}

		var redis *rdb.Redis
		opts, err := getQueueOptions(cfg, redis)

		assert.NoError(t, err)
		assert.Nil(t, opts.RedisFailoverOpt)
		assert.Equal(t, []string{"redis://localhost:6379"}, opts.RedisAddress)
	})

	t.Run("Redis Sentinel Configuration", func(t *testing.T) {
		cfg := &config.Configuration{
			Redis: config.RedisConfiguration{
				Scheme:           "redis-sentinel",
				Addresses:        "sentinel1:26379,sentinel2:26379",
				MasterName:       "mymaster",
				Username:         "user",
				Password:         "pass",
				SentinelPassword: "sentinel_pass",
				Database:         "0",
			},
		}

		var redis *rdb.Redis
		opts, err := getQueueOptions(cfg, redis)

		assert.NoError(t, err)
		assert.NotNil(t, opts.RedisFailoverOpt)
		assert.Equal(t, "mymaster", opts.RedisFailoverOpt.MasterName)
		assert.Equal(t, []string{"sentinel1:26379", "sentinel2:26379"}, opts.RedisFailoverOpt.SentinelAddrs)
		assert.Equal(t, "user", opts.RedisFailoverOpt.Username)
		assert.Equal(t, "pass", opts.RedisFailoverOpt.Password)
		assert.Equal(t, "sentinel_pass", opts.RedisFailoverOpt.SentinelPassword)
		assert.Equal(t, 0, opts.RedisFailoverOpt.DB)
	})

	t.Run("Redis Sentinel Invalid Database", func(t *testing.T) {
		cfg := &config.Configuration{
			Redis: config.RedisConfiguration{
				Scheme:   "redis-sentinel",
				Database: "invalid",
			},
		}

		var redis *rdb.Redis
		_, err := getQueueOptions(cfg, redis)

		assert.Error(t, err)
	})
}

func TestApplyLicensePrecedence(t *testing.T) {
	t.Run("env license wins as effective without replacing the purchased checkout key", func(t *testing.T) {
		instCfg := &datastore.Configuration{CheckoutLicenseKey: "purchased-license"}
		cfg := &config.Configuration{LicenseKey: "server-qa-license"}

		changed := applyLicensePrecedence(instCfg, cfg)

		// env is the effective license and is persisted with env provenance, but the
		// purchased key stays in its own column so the override is reversible.
		assert.True(t, changed)
		assert.Equal(t, "server-qa-license", cfg.LicenseKey)
		assert.Equal(t, "server-qa-license", instCfg.LicenseKey)
		assert.Equal(t, config.LicenseSourceEnv, instCfg.LicenseKeySource)
		assert.Equal(t, "purchased-license", instCfg.CheckoutLicenseKey)
		assert.True(t, instCfg.LicenseSyncedAt.Valid)
	})

	t.Run("purchased checkout key is effective when env license is empty", func(t *testing.T) {
		instCfg := &datastore.Configuration{CheckoutLicenseKey: "purchased-license"}
		cfg := &config.Configuration{}

		changed := applyLicensePrecedence(instCfg, cfg)

		assert.True(t, changed)
		assert.Equal(t, "purchased-license", cfg.LicenseKey)
		assert.Equal(t, "purchased-license", instCfg.LicenseKey)
		assert.Equal(t, config.LicenseSourceGuestCheckout, instCfg.LicenseKeySource)
	})

	t.Run("already-resolved env license is a no-op", func(t *testing.T) {
		instCfg := &datastore.Configuration{
			LicenseKey:         "server-qa-license",
			LicenseKeySource:   config.LicenseSourceEnv,
			CheckoutLicenseKey: "purchased-license",
		}
		cfg := &config.Configuration{LicenseKey: "server-qa-license"}

		changed := applyLicensePrecedence(instCfg, cfg)

		assert.False(t, changed)
		assert.Equal(t, "server-qa-license", cfg.LicenseKey)
		assert.False(t, instCfg.LicenseSyncedAt.Valid)
	})

	t.Run("removing env reverts effective to the purchased checkout key", func(t *testing.T) {
		instCfg := &datastore.Configuration{
			LicenseKey:         "old-env-license",
			LicenseKeySource:   config.LicenseSourceEnv,
			CheckoutLicenseKey: "purchased-license",
		}
		cfg := &config.Configuration{}

		changed := applyLicensePrecedence(instCfg, cfg)

		assert.True(t, changed)
		assert.Equal(t, "purchased-license", cfg.LicenseKey)
		assert.Equal(t, "purchased-license", instCfg.LicenseKey)
		assert.Equal(t, config.LicenseSourceGuestCheckout, instCfg.LicenseKeySource)
		assert.Equal(t, "purchased-license", instCfg.CheckoutLicenseKey)
	})

	t.Run("legacy guest license without checkout column is preserved, not blanked", func(t *testing.T) {
		// Pre-migration/legacy row: the guest key lives only in license_key and the
		// checkout column is empty. Boot must keep the paid license and self-heal the
		// checkout column instead of resolving to an empty effective key.
		instCfg := &datastore.Configuration{
			LicenseKey:         "legacy-guest-license",
			LicenseKeySource:   config.LicenseSourceGuestCheckout,
			CheckoutLicenseKey: "",
		}
		cfg := &config.Configuration{}

		changed := applyLicensePrecedence(instCfg, cfg)

		assert.True(t, changed)
		assert.Equal(t, "legacy-guest-license", cfg.LicenseKey)
		assert.Equal(t, "legacy-guest-license", instCfg.LicenseKey)
		assert.Equal(t, config.LicenseSourceGuestCheckout, instCfg.LicenseKeySource)
		assert.Equal(t, "legacy-guest-license", instCfg.CheckoutLicenseKey)
	})

	t.Run("env license backfills the checkout column from a legacy guest key", func(t *testing.T) {
		// Env is added on an instance whose guest key was only in license_key. Env
		// wins as effective, but the purchased key is recovered into the checkout
		// column so the override stays reversible.
		instCfg := &datastore.Configuration{
			LicenseKey:         "legacy-guest-license",
			LicenseKeySource:   config.LicenseSourceGuestCheckout,
			CheckoutLicenseKey: "",
		}
		cfg := &config.Configuration{LicenseKey: "server-qa-license"}

		changed := applyLicensePrecedence(instCfg, cfg)

		assert.True(t, changed)
		assert.Equal(t, "server-qa-license", cfg.LicenseKey)
		assert.Equal(t, "server-qa-license", instCfg.LicenseKey)
		assert.Equal(t, config.LicenseSourceEnv, instCfg.LicenseKeySource)
		assert.Equal(t, "legacy-guest-license", instCfg.CheckoutLicenseKey)
	})
}
