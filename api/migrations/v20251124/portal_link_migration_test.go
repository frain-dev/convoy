package v20251124

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreatePortalLinkMigration_OwnerIDRequired(t *testing.T) {
	migration := NewCreatePortalLinkMigration()
	ctx := context.Background()

	input := map[string]interface{}{"owner_id": "owner-123"}
	result, err := migration.MigrateForward(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, result)

	input = map[string]interface{}{"name": "test"}
	_, err = migration.MigrateForward(ctx, input)
	require.ErrorContains(t, err, "owner_id is required")

	input = map[string]interface{}{"owner_id": ""}
	_, err = migration.MigrateForward(ctx, input)
	require.ErrorContains(t, err, "owner_id is required")
}

func TestUpdatePortalLinkMigration_PassThrough(t *testing.T) {
	migration := NewUpdatePortalLinkMigration()
	ctx := context.Background()

	input := map[string]interface{}{"name": "test"}
	result, err := migration.MigrateForward(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, result)

	data := result.(map[string]interface{})
	require.Equal(t, "test", data["name"])
}
