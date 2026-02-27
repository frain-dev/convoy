package api

import (
	"github.com/subomi/requestmigrations/v2"

	v20240101 "github.com/frain-dev/convoy/api/migrations/v20240101"
	v20240401 "github.com/frain-dev/convoy/api/migrations/v20240401"
	v20251124 "github.com/frain-dev/convoy/api/migrations/v20251124"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func RegisterMigrations(rm *requestmigrations.RequestMigration) error {
	return rm.Register(
		requestmigrations.Migration[models.CreateEndpoint]("2024-01-01", &v20240101.CreateEndpointMigration{}),
		requestmigrations.Migration[models.UpdateEndpoint]("2024-01-01", &v20240101.UpdateEndpointMigration{}),
		requestmigrations.Migration[models.EndpointResponse]("2024-01-01", &v20240101.EndpointResponseMigration{}),
		requestmigrations.Migration[models.EndpointResponse]("2024-04-01", &v20240401.EndpointResponseMigration{}),
		requestmigrations.Migration[datastore.CreatePortalLinkRequest]("2025-11-24", v20251124.NewCreatePortalLinkMigration()),
		requestmigrations.Migration[datastore.UpdatePortalLinkRequest]("2025-11-24", v20251124.NewUpdatePortalLinkMigration()),
	).Build()
}
