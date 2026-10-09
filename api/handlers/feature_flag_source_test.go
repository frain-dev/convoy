package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
)

func TestSetFeatureFlagSource(t *testing.T) {
	flag := &datastore.FeatureFlag{FeatureKey: string(fflag.CircuitBreaker)}
	envFlags := fflag.NewFFlag([]string{string(fflag.CircuitBreaker)})

	setFeatureFlagSource(flag, envFlags, true)

	require.True(t, flag.EnvEnabled)
	require.True(t, flag.AdminManaged)
}
