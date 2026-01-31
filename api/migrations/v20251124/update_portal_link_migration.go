package v20251124

import "context"

type UpdatePortalLinkMigration struct{}

func NewUpdatePortalLinkMigration() *UpdatePortalLinkMigration {
	return &UpdatePortalLinkMigration{}
}

func (m *UpdatePortalLinkMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	return data, nil
}

func (m *UpdatePortalLinkMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	return data, nil
}
