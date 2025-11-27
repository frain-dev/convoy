package api

import (
	v20240101 "github.com/frain-dev/convoy/api/migrations/v20240101"
	v20240401 "github.com/frain-dev/convoy/api/migrations/v20240401"
	v20251124 "github.com/frain-dev/convoy/api/migrations/v20251124"
	"github.com/subomi/requestmigrations"
)

var migrations = requestmigrations.MigrationStore{
	"2024-01-01": requestmigrations.Migrations{
		&v20240101.CreateEndpointRequestMigration{},
		&v20240101.CreateEndpointResponseMigration{},
		&v20240101.GetEndpointResponseMigration{},
		&v20240101.GetEndpointsResponseMigration{},
		&v20240101.UpdateEndpointRequestMigration{},
		&v20240101.UpdateEndpointResponseMigration{},
	},
	"2024-04-01": requestmigrations.Migrations{
		&v20240401.CreateEndpointResponseMigration{},
		&v20240401.GetEndpointResponseMigration{},
		&v20240401.GetEndpointsResponseMigration{},
		&v20240401.UpdateEndpointResponseMigration{},
	},
	"2025-11-24": requestmigrations.Migrations{
		v20251124.NewCreatePortalLinkRequestMigration(),
		v20251124.NewUpdatePortalLinkRequestMigration(),
	},
}
