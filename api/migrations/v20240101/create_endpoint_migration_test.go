package v20240101

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateEndpointMigration_AdvancedSignaturesDefaultFalse(t *testing.T) {
	migration := &CreateEndpointMigration{}
	ctx := context.Background()

	input := map[string]interface{}{
		"name": "test",
		"url":  "https://example.com",
	}

	result, err := migration.MigrateForward(ctx, input)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	require.Equal(t, false, data["advanced_signatures"])
}

func TestCreateEndpointMigration_AdvancedSignaturesPreserved(t *testing.T) {
	migration := &CreateEndpointMigration{}
	ctx := context.Background()

	input := map[string]interface{}{
		"name":                "test",
		"url":                 "https://example.com",
		"advanced_signatures": true,
	}

	result, err := migration.MigrateForward(ctx, input)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	require.Equal(t, true, data["advanced_signatures"])
}

func TestCreateEndpointMigration_DurationConversion(t *testing.T) {
	migration := &CreateEndpointMigration{}
	ctx := context.Background()

	input := map[string]interface{}{
		"name":                "test",
		"url":                 "https://example.com",
		"http_timeout":        "30s",
		"rate_limit_duration": "1m",
	}

	result, err := migration.MigrateForward(ctx, input)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	require.Equal(t, uint64(30), data["http_timeout"])
	require.Equal(t, uint64(60), data["rate_limit_duration"])
}

func TestCreateEndpointMigration_BackwardConversion(t *testing.T) {
	migration := &CreateEndpointMigration{}
	ctx := context.Background()

	input := map[string]interface{}{
		"name":                "test",
		"url":                 "https://example.com",
		"http_timeout":        float64(30),
		"rate_limit_duration": float64(60),
	}

	result, err := migration.MigrateBackward(ctx, input)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	require.Equal(t, "30s", data["http_timeout"])
	require.Equal(t, "1m0s", data["rate_limit_duration"])
}

func TestEndpointResponseMigration_BackwardConversion(t *testing.T) {
	migration := &EndpointResponseMigration{}
	ctx := context.Background()

	input := map[string]interface{}{
		"uid":                 "endpoint-123",
		"name":                "test",
		"url":                 "https://example.com",
		"http_timeout":        float64(30),
		"rate_limit_duration": float64(60),
	}

	result, err := migration.MigrateBackward(ctx, input)
	require.NoError(t, err)

	data := result.(map[string]interface{})
	require.Equal(t, "30s", data["http_timeout"])
	require.Equal(t, "1m0s", data["rate_limit_duration"])
}
