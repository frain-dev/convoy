package v20251124

import (
	"context"
	"errors"
)

type CreatePortalLinkMigration struct{}

func NewCreatePortalLinkMigration() *CreatePortalLinkMigration {
	return &CreatePortalLinkMigration{}
}

func (m *CreatePortalLinkMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	d, ok := data.(map[string]interface{})
	if !ok {
		return data, nil
	}

	ownerID, ok := d["owner_id"]
	if !ok {
		return nil, errors.New("owner_id is required")
	}

	if str, ok := ownerID.(string); ok && str == "" {
		return nil, errors.New("owner_id is required")
	}

	return d, nil
}

func (m *CreatePortalLinkMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	return data, nil
}
