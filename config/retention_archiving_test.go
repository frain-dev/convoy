package config

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/pkg/configmigrate"
)

func TestOverride_SkipsFalseRetentionEnabled(t *testing.T) {
	require.NoError(t, LoadConfig("./testdata/Config/valid-convoy.json"))

	before, err := Get()
	require.NoError(t, err)
	require.True(t, before.Retention.Enabled, "fixture/default should leave retention enabled")

	require.NoError(t, Override(&Configuration{
		Retention: RetentionConfiguration{Enabled: false},
	}))

	after, err := Get()
	require.NoError(t, err)
	require.True(t, after.Retention.Enabled, "Override must not apply false bools (zero value)")
}

func TestForceBools_DisablesRetentionEnabled(t *testing.T) {
	require.NoError(t, LoadConfig("./testdata/Config/valid-convoy.json"))

	require.NoError(t, ForceBools(func(c *Configuration) {
		c.Retention.Enabled = false
		c.WebhookArchiving.Enabled = true
	}))

	got, err := Get()
	require.NoError(t, err)
	require.False(t, got.Retention.Enabled)
	require.True(t, got.WebhookArchiving.Enabled)
}

func TestJSONRetentionArchivingMigrations(t *testing.T) {
	root := map[string]any{
		"retention_policy": map[string]any{
			"policy":          "336h",
			"enabled":         true,
			"backup_interval": "2h",
		},
	}

	deps, err := jsonRetentionArchivingMigrations().Apply(configmigrate.OSEnv{}, root)
	require.NoError(t, err)
	require.NotEmpty(t, deps)

	retention, ok := root["retention"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "336h", retention["period"])

	archiving, ok := root["webhook_archiving"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, archiving["enabled"])
	require.Equal(t, "2h", archiving["interval"])
	_, stillLegacy := root["retention_policy"]
	require.False(t, stillLegacy)
}

func TestEnvRetentionArchivingMigrations(t *testing.T) {
	c := DefaultConfiguration
	env := configmigrate.MapEnv{
		"CONVOY_RETENTION_POLICY":         "168h",
		"CONVOY_RETENTION_POLICY_ENABLED": "true",
	}

	deps, err := envRetentionArchivingMigrations(&c).Apply(env, nil)
	require.NoError(t, err)
	require.Len(t, deps, 2)
	require.Equal(t, "168h", c.Retention.Period)
	require.True(t, c.WebhookArchiving.Enabled)
}

func TestEnvRetentionArchivingMigrations_NewWins(t *testing.T) {
	c := DefaultConfiguration
	c.Retention.Period = "240h"
	c.WebhookArchiving.Enabled = false
	env := configmigrate.MapEnv{
		"CONVOY_RETENTION_POLICY":          "168h",
		"CONVOY_RETENTION_PERIOD":          "240h",
		"CONVOY_RETENTION_POLICY_ENABLED":  "true",
		"CONVOY_WEBHOOK_ARCHIVING_ENABLED": "false",
	}

	deps, err := envRetentionArchivingMigrations(&c).Apply(env, nil)
	require.NoError(t, err)
	require.Empty(t, deps)
	require.Equal(t, "240h", c.Retention.Period)
	require.False(t, c.WebhookArchiving.Enabled)
}

func TestLoadConfig_MigratesLegacyRetentionJSON(t *testing.T) {
	// valid-convoy.json still uses retention_policy; migrate must rewrite into
	// Retention.Period + WebhookArchiving before validate.
	require.NoError(t, LoadConfig("./testdata/Config/valid-convoy.json"))

	got, err := Get()
	require.NoError(t, err)
	require.Equal(t, "720h", got.Retention.Period)
	require.True(t, got.WebhookArchiving.Enabled)
	require.Equal(t, "1h", got.WebhookArchiving.Interval)
}
