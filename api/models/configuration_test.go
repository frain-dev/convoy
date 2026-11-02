package models

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

func TestRetentionPolicyTransform_PeriodPreferred(t *testing.T) {
	r := &RetentionPolicyConfiguration{Period: "720h", Policy: "168h"}
	got := r.Transform()
	require.NotNil(t, got)
	require.Equal(t, "720h", got.Period)
	require.True(t, got.Enabled, "omitted enabled defaults to true")
}

func TestRetentionPolicyTransform_PolicyAlias(t *testing.T) {
	r := &RetentionPolicyConfiguration{Policy: "336h"}
	got := r.Transform()
	require.NotNil(t, got)
	require.Equal(t, "336h", got.Period)
}

func TestRetentionPolicyTransform_EnabledExplicit(t *testing.T) {
	off := false
	got := (&RetentionPolicyConfiguration{Period: "48h", Enabled: &off}).Transform()
	require.NotNil(t, got)
	require.False(t, got.Enabled)

	on := true
	got = (&RetentionPolicyConfiguration{Period: "48h", Enabled: &on}).Transform()
	require.True(t, got.Enabled)
}

func TestConfigurationValidate_PolicyAliasDuration(t *testing.T) {
	cfg := &Configuration{
		RetentionPolicy: &RetentionPolicyConfiguration{Policy: "not-a-duration"},
	}
	require.Error(t, cfg.Validate())

	cfg.RetentionPolicy = &RetentionPolicyConfiguration{Policy: "48h"}
	require.NoError(t, cfg.Validate())
}

func TestConfigurationValidateForUpdate_AllowsBlankStorageSecrets(t *testing.T) {
	cfg := &Configuration{
		RetentionPolicy: &RetentionPolicyConfiguration{Period: "48h"},
		StoragePolicy: &StoragePolicyConfiguration{
			Type: datastore.OnPrem,
			OnPrem: &OnPremStorage{
				Path: null.String{}, // blank = keep on update
			},
		},
	}
	require.NoError(t, cfg.ValidateForUpdate())

	cfg.RetentionPolicy = &RetentionPolicyConfiguration{Period: "nope"}
	require.Error(t, cfg.ValidateForUpdate())
}

func TestConfigurationValidateForUpdate_RejectsUnknownStorageType(t *testing.T) {
	cfg := &Configuration{
		StoragePolicy: &StoragePolicyConfiguration{Type: "gcs"},
	}
	require.Error(t, cfg.ValidateForUpdate())
}

func TestConfigurationValidateForUpdate_RejectsEmptyStorageType(t *testing.T) {
	// Empty type with a non-nil storage_policy would wipe stored columns on update.
	cfg := &Configuration{
		StoragePolicy: &StoragePolicyConfiguration{Type: ""},
	}
	require.Error(t, cfg.ValidateForUpdate())
}

func TestWebhookArchivingTransform(t *testing.T) {
	require.Nil(t, (*WebhookArchivingConfiguration)(nil).Transform())
	got := (&WebhookArchivingConfiguration{Enabled: true}).Transform()
	require.NotNil(t, got)
	require.True(t, got.Enabled)
}
