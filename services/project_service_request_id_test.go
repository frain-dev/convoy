package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

func TestValidateHTTPHeaderToken(t *testing.T) {
	require.NoError(t, validateHTTPHeaderToken("Split-Request-ID"))
	require.Error(t, validateHTTPHeaderToken("bad header"))
	require.Error(t, validateHTTPHeaderToken(""))
}

func TestValidateRequestIDHeaderForProject_RejectsInvalidToken(t *testing.T) {
	cfg := &datastore.ProjectConfig{
		RequestIDHeader: config.RequestIDHeaderProvider("bad header"),
	}
	err := validateRequestIDHeaderForProject(datastore.OutgoingProject, cfg)
	require.ErrorIs(t, err, ErrInvalidRequestIDHeaderName)
}

func TestApplyProjectConfigPatch_PreservesUnsetFields(t *testing.T) {
	existing := &datastore.ProjectConfig{
		ReplayAttacks: true,
		RateLimit: &datastore.RateLimitConfiguration{
			Count:    1000,
			Duration: 60,
		},
		RequestIDHeader: config.DefaultRequestIDHeader,
	}
	patch := &models.ProjectConfig{
		RequestIDHeader: config.RequestIDHeaderProvider("Split-Request-ID"),
	}
	merged := applyProjectConfigPatch(existing, patch)
	require.True(t, merged.ReplayAttacks)
	require.Equal(t, 1000, merged.RateLimit.Count)
	require.Equal(t, config.RequestIDHeaderProvider("Split-Request-ID"), merged.RequestIDHeader)
}
