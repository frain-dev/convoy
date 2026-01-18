package v20240401

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpointResponseMigration_BackwardFieldRename(t *testing.T) {
	migration := &EndpointResponseMigration{}
	ctx := context.Background()

	// Current format with url and name fields
	input := map[string]interface{}{
		"uid":  "endpoint-123",
		"url":  "https://google.com",
		"name": "test-endpoint",
	}

	result, err := migration.MigrateBackward(ctx, input)
	require.NoError(t, err)

	data := result.(map[string]interface{})

	// url should be renamed to target_url
	require.Equal(t, "https://google.com", data["target_url"])
	require.Nil(t, data["url"])

	// name should be renamed to title
	require.Equal(t, "test-endpoint", data["title"])
	require.Nil(t, data["name"])
}

func TestEndpointResponseMigration_ForwardNoOp(t *testing.T) {
	migration := &EndpointResponseMigration{}
	ctx := context.Background()

	// Forward migration should be a no-op for responses
	input := map[string]interface{}{
		"uid":        "endpoint-123",
		"target_url": "https://google.com",
		"title":      "test-endpoint",
	}

	result, err := migration.MigrateForward(ctx, input)
	require.NoError(t, err)

	// Should return unchanged
	data := result.(map[string]interface{})
	require.Equal(t, "https://google.com", data["target_url"])
	require.Equal(t, "test-endpoint", data["title"])
}
