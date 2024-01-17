package api

import (
	v20242501 "github.com/frain-dev/convoy/api/migrations/v20242501"
	"github.com/subomi/requestmigrations"
)

var migrations = requestmigrations.MigrationStore{
	"2024-25-01": requestmigrations.Migrations{
		&v20242501.CreateEndpointRequestMigration{},
	},
}
