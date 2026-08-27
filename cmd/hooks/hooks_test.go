package hooks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

type migrationConfigurationStore struct {
	completedID      string
	managed          bool
	retentionEnabled bool
	retentionSeed    bool
	err              error
	order            *[]string
}

func (s *migrationConfigurationStore) CompleteAdminManagedMigration(
	_ context.Context,
	id string,
	retentionEnabled bool,
) (bool, bool, error) {
	if s.order != nil {
		*s.order = append(*s.order, "configuration")
	}
	s.completedID = id
	s.retentionSeed = retentionEnabled
	return s.managed, s.retentionEnabled, s.err
}

type migrationFeatureFlagStore struct {
	flag        *datastore.FeatureFlag
	fetchErr    error
	updateErr   error
	fetchCalls  int
	updateCalls int
	order       *[]string
}

func (s *migrationFeatureFlagStore) FetchFeatureFlagByKey(context.Context, string) (*datastore.FeatureFlag, error) {
	s.fetchCalls++
	return s.flag, s.fetchErr
}

func (s *migrationFeatureFlagStore) UpdateFeatureFlag(context.Context, string, bool) error {
	if s.order != nil {
		*s.order = append(*s.order, "circuit-breaker")
	}
	s.updateCalls++
	return s.updateErr
}

func TestCompleteAdminManagedMigration(t *testing.T) {
	t.Run("preserves existing configuration", func(t *testing.T) {
		updatedAt := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
		instanceConfig := &datastore.Configuration{
			UID:                "config-1",
			IsAnalyticsEnabled: false,
			IsSignupEnabled:    false,
			StoragePolicy: &datastore.StoragePolicyConfiguration{
				Type: datastore.OnPrem,
			},
			RetentionPolicy: &datastore.RetentionPolicyConfiguration{Period: "168h"},
			UpdatedAt:       updatedAt,
		}
		envConfig := config.Configuration{}
		envConfig.Analytics.IsEnabled = true
		envConfig.Auth.IsSignupEnabled = true
		envConfig.Retention.Enabled = true
		configStore := &migrationConfigurationStore{managed: true, retentionEnabled: true}
		flagStore := &migrationFeatureFlagStore{}

		err := completeAdminManagedMigration(
			context.Background(),
			envConfig,
			instanceConfig,
			configStore,
			flagStore,
		)

		require.NoError(t, err)
		assert.Equal(t, "config-1", configStore.completedID)
		assert.True(t, instanceConfig.AdminManaged)
		assert.True(t, instanceConfig.AdminManagedKnown)
		assert.False(t, instanceConfig.IsAnalyticsEnabled)
		assert.False(t, instanceConfig.IsSignupEnabled)
		assert.Equal(t, datastore.OnPrem, instanceConfig.StoragePolicy.Type)
		assert.True(t, instanceConfig.RetentionPolicy.Enabled)
		assert.True(t, instanceConfig.RetentionPolicy.EnabledKnown)
		assert.True(t, configStore.retentionSeed)
		assert.True(t, instanceConfig.UpdatedAt.After(updatedAt))
		assert.Zero(t, flagStore.fetchCalls)
	})

	t.Run("mirrors an env-enabled circuit breaker before selecting DB ownership", func(t *testing.T) {
		order := make([]string, 0, 2)
		instanceConfig := &datastore.Configuration{
			UID:             "config-1",
			RetentionPolicy: &datastore.RetentionPolicyConfiguration{},
		}
		envConfig := config.Configuration{
			EnableFeatureFlag: []string{"circuit-breaker"},
		}
		configStore := &migrationConfigurationStore{managed: true, order: &order}
		flagStore := &migrationFeatureFlagStore{
			flag:  &datastore.FeatureFlag{UID: "flag-1"},
			order: &order,
		}

		err := completeAdminManagedMigration(
			context.Background(),
			envConfig,
			instanceConfig,
			configStore,
			flagStore,
		)

		require.NoError(t, err)
		assert.Equal(t, []string{"circuit-breaker", "configuration"}, order)
		assert.Equal(t, 1, flagStore.fetchCalls)
		assert.Equal(t, 1, flagStore.updateCalls)
		assert.True(t, instanceConfig.AdminManaged)
		assert.True(t, instanceConfig.AdminManagedKnown)
	})

	t.Run("does not complete ownership migration when circuit breaker cannot be preserved", func(t *testing.T) {
		instanceConfig := &datastore.Configuration{
			UID:             "config-1",
			RetentionPolicy: &datastore.RetentionPolicyConfiguration{},
		}
		envConfig := config.Configuration{
			EnableFeatureFlag: []string{"circuit-breaker"},
		}
		configStore := &migrationConfigurationStore{}
		flagStore := &migrationFeatureFlagStore{
			fetchErr: errors.New("database unavailable"),
		}

		err := completeAdminManagedMigration(
			context.Background(),
			envConfig,
			instanceConfig,
			configStore,
			flagStore,
		)

		require.ErrorContains(t, err, "load circuit-breaker flag")
		assert.Empty(t, configStore.completedID)
		assert.False(t, instanceConfig.AdminManaged)
		assert.False(t, instanceConfig.AdminManagedKnown)
	})

	t.Run("does not complete ownership migration when circuit breaker update fails", func(t *testing.T) {
		instanceConfig := &datastore.Configuration{
			UID:             "config-1",
			RetentionPolicy: &datastore.RetentionPolicyConfiguration{},
		}
		envConfig := config.Configuration{
			EnableFeatureFlag: []string{"circuit-breaker"},
		}
		configStore := &migrationConfigurationStore{}
		flagStore := &migrationFeatureFlagStore{
			flag:      &datastore.FeatureFlag{UID: "flag-1"},
			updateErr: errors.New("database unavailable"),
		}

		err := completeAdminManagedMigration(
			context.Background(),
			envConfig,
			instanceConfig,
			configStore,
			flagStore,
		)

		require.ErrorContains(t, err, "enable circuit-breaker flag")
		assert.Empty(t, configStore.completedID)
		assert.False(t, instanceConfig.AdminManaged)
		assert.False(t, instanceConfig.AdminManagedKnown)
	})

	t.Run("does not change in-memory ownership when persistence fails", func(t *testing.T) {
		instanceConfig := &datastore.Configuration{
			UID:             "config-1",
			RetentionPolicy: &datastore.RetentionPolicyConfiguration{},
		}
		configStore := &migrationConfigurationStore{
			err: errors.New("database unavailable"),
		}

		err := completeAdminManagedMigration(
			context.Background(),
			config.Configuration{},
			instanceConfig,
			configStore,
			&migrationFeatureFlagStore{},
		)

		require.EqualError(t, err, "database unavailable")
		assert.False(t, instanceConfig.AdminManaged)
		assert.False(t, instanceConfig.AdminManagedKnown)
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
