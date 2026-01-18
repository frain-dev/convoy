package v20251124

import "context"

// UpdatePortalLinkMigration handles request migration for portal link updates.
// This is a pass-through migration - no transformations needed.
type UpdatePortalLinkMigration struct{}

func NewUpdatePortalLinkMigration() *UpdatePortalLinkMigration {
	return &UpdatePortalLinkMigration{}
}

func (m *UpdatePortalLinkMigration) MigrateForward(ctx context.Context, data any) (any, error) {
	// Pass-through - no transformation needed
	return data, nil
}

func (m *UpdatePortalLinkMigration) MigrateBackward(ctx context.Context, data any) (any, error) {
	// Pass-through - no transformation needed
	return data, nil
}
