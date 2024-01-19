package api

import (
	v20240101 "github.com/frain-dev/convoy/api/migrations/v20240101"
	"github.com/subomi/requestmigrations"
)

var migrations = requestmigrations.MigrationStore{
	"2024-01-01": requestmigrations.Migrations{
		&v20240101.CreateEndpointRequestMigration{},
		&v20240101.CreateEndpointResponseMigration{},
	},
}
