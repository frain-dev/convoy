package api

import (
	"github.com/subomi/requestmigrations/v2"

	v20240101 "github.com/frain-dev/convoy/api/migrations/v20240101"
	v20240401 "github.com/frain-dev/convoy/api/migrations/v20240401"
	v20251124 "github.com/frain-dev/convoy/api/migrations/v20251124"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

// RegisterMigrations registers all API version migrations with the RequestMigration instance.
func RegisterMigrations(rm *requestmigrations.RequestMigration) error {
	// v2024-01-01 migrations: http_timeout/rate_limit_duration string→uint64, advanced_signatures default
	if err := requestmigrations.Register[models.CreateEndpoint](rm, "2024-01-01", &v20240101.CreateEndpointMigration{}); err != nil {
		return err
	}
	if err := requestmigrations.Register[models.UpdateEndpoint](rm, "2024-01-01", &v20240101.UpdateEndpointMigration{}); err != nil {
		return err
	}
	if err := requestmigrations.Register[models.EndpointResponse](rm, "2024-01-01", &v20240101.EndpointResponseMigration{}); err != nil {
		return err
	}

	// v2024-04-01 migrations: url→target_url, name→title field renames
	if err := requestmigrations.Register[models.EndpointResponse](rm, "2024-04-01", &v20240401.EndpointResponseMigration{}); err != nil {
		return err
	}

	// v2025-11-24 migrations: owner_id validation for portal links
	if err := requestmigrations.Register[datastore.CreatePortalLinkRequest](rm, "2025-11-24", v20251124.NewCreatePortalLinkMigration()); err != nil {
		return err
	}
	if err := requestmigrations.Register[datastore.UpdatePortalLinkRequest](rm, "2025-11-24", v20251124.NewUpdatePortalLinkMigration()); err != nil {
		return err
	}

	return nil
}
