package api

import (
	v20240101 "github.com/frain-dev/convoy/api/migrations/v20240101"
	v20240306 "github.com/frain-dev/convoy/api/migrations/v20240306"
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
	"2024-03-06": requestmigrations.Migrations{
		&v20240306.CreateEndpointResponseMigration{},
		&v20240306.GetEndpointResponseMigration{},
		&v20240306.GetEndpointsResponseMigration{},
		&v20240306.UpdateEndpointResponseMigration{},
	},
}
