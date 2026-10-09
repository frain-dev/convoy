package config

import (
	"github.com/frain-dev/convoy/pkg/configmigrate"
)

// jsonRetentionArchivingMigrations rewrites legacy retention_policy keys into
// retention + webhook_archiving before Configuration decode.
func jsonRetentionArchivingMigrations() *configmigrate.Runner {
	return configmigrate.New(
		configmigrate.RenameJSONString(
			[]string{"retention_policy", "policy"},
			[]string{"retention", "period"},
		),
		configmigrate.RenameJSONBool(
			[]string{"retention_policy", "enabled"},
			[]string{"webhook_archiving", "enabled"},
		),
		configmigrate.MoveJSONValue(
			[]string{"retention_policy", "backup_interval"},
			[]string{"webhook_archiving", "interval"},
		),
		configmigrate.MoveJSONValue(
			[]string{"retention_policy", "cdc_backup_enabled"},
			[]string{"webhook_archiving", "cdc_enabled"},
		),
		configmigrate.MoveJSONValue(
			[]string{"retention_policy", "replication_dsn"},
			[]string{"webhook_archiving", "replication_dsn"},
		),
		dropLegacyRetentionPolicyObject(),
	)
}

func dropLegacyRetentionPolicyObject() configmigrate.Step {
	return func(_ configmigrate.Env, root map[string]any) ([]configmigrate.Deprecation, error) {
		if root == nil {
			return nil, nil
		}
		if _, ok := root["retention_policy"]; !ok {
			return nil, nil
		}
		delete(root, "retention_policy")
		return nil, nil
	}
}

// envRetentionArchivingMigrations maps deprecated env vars when the new keys
// are unset. Call after envconfig.Process.
func envRetentionArchivingMigrations(c *Configuration) *configmigrate.Runner {
	return configmigrate.New(
		configmigrate.RenameEnvString("CONVOY_RETENTION_POLICY", "CONVOY_RETENTION_PERIOD", func(v string) {
			c.Retention.Period = v
		}),
		configmigrate.RenameEnvBool("CONVOY_RETENTION_POLICY_ENABLED", "CONVOY_WEBHOOK_ARCHIVING_ENABLED", func(v bool) {
			c.WebhookArchiving.Enabled = v
		}),
	)
}
