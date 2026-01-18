package v20251124

import (
	"context"
	"errors"
)

// CreatePortalLinkMigration handles request migration for datastore.CreatePortalLinkRequest
// This migration validates that owner_id is provided.
type CreatePortalLinkMigration struct{}

func NewCreatePortalLinkMigration() *CreatePortalLinkMigration {
	return &CreatePortalLinkMigration{}
}

func (m *CreatePortalLinkMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	// Validate owner_id is provided
	ownerID, ok := d["owner_id"]
	if !ok {
		return nil, errors.New("owner_id is required")
	}

	// Check if owner_id is empty string
	if str, ok := ownerID.(string); ok && str == "" {
		return nil, errors.New("owner_id is required")
	}

	return d, nil
}

func (m *CreatePortalLinkMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	// No backward migration needed
	return data, nil
}
