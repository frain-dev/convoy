package models

import (
	"testing"

	"github.com/stretchr/testify/require"
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

func TestWebhookArchivingTransform(t *testing.T) {
	require.Nil(t, (*WebhookArchivingConfiguration)(nil).Transform())
	got := (&WebhookArchivingConfiguration{Enabled: true}).Transform()
	require.NotNil(t, got)
	require.True(t, got.Enabled)
}
