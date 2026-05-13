//go:build integration

// Package integration tests exercise the postgres job repository against a real database.
// They TRUNCATE most convoy schema tables (see truncateTables). Do not point TEST_DB_* at
// production or any database you care about. To run:
//
//   CONVOY_POSTGRES_INTEGRATION_ALLOW_DESTRUCTIVE=1 go test -tags=integration ./database/postgres -v
//
// Use a disposable Postgres (clone, docker test instance, or CI service), not your live instance.

package postgres

import (
	"context"
	"fmt"
	stdlog "log"
	"os"
	"sync"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/users"
	applog "github.com/frain-dev/convoy/pkg/logger"
	"github.com/kelseyhightower/envconfig"
)

// integrationLogger is used by integration seed helpers (same package as job tests).
var integrationLogger = applog.New("convoy", applog.LevelError)

func getConfig() config.Configuration {
	_ = os.Setenv("CONVOY_DB_HOST", os.Getenv("TEST_REDIS_HOST"))
	_ = os.Setenv("CONVOY_REDIS_SCHEME", os.Getenv("TEST_REDIS_SCHEME"))
	_ = os.Setenv("CONVOY_REDIS_PORT", os.Getenv("TEST_REDIS_PORT"))

	_ = os.Setenv("CONVOY_DB_HOST", os.Getenv("TEST_DB_HOST"))
	_ = os.Setenv("CONVOY_DB_SCHEME", os.Getenv("TEST_DB_SCHEME"))
	_ = os.Setenv("CONVOY_DB_USERNAME", os.Getenv("TEST_DB_USERNAME"))
	_ = os.Setenv("CONVOY_DB_PASSWORD", os.Getenv("TEST_DB_PASSWORD"))
	_ = os.Setenv("CONVOY_DB_DATABASE", os.Getenv("TEST_DB_DATABASE"))
	_ = os.Setenv("CONVOY_DB_PORT", os.Getenv("TEST_DB_PORT"))
	if opt := os.Getenv("TEST_DB_OPTIONS"); opt != "" {
		_ = os.Setenv("CONVOY_DB_OPTIONS", opt)
	} else if os.Getenv("CONVOY_DB_OPTIONS") == "" {
		_ = os.Setenv("CONVOY_DB_OPTIONS", "sslmode=disable&connect_timeout=30")
	}

	_ = os.Setenv("CONVOY_LOCAL_ENCRYPTION_KEY", "test-key")

	err := config.LoadConfig("", func(c *config.Configuration) error {
		// Merge CONVOY_* into config (LoadConfig alone only applies JSON + defaults).
		return envconfig.Process("convoy", c)
	})
	if err != nil {
		stdlog.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
		stdlog.Fatal(err)
	}

	km, err := keys.NewLocalKeyManager("test")
	if err != nil {
		stdlog.Fatal(err)
	}
	if km.IsSet() {
		if _, err = km.GetCurrentKeyFromCache(); err != nil {
			stdlog.Fatal(err)
		}
	}
	if err = keys.Set(km); err != nil {
		stdlog.Fatal(err)
	}

	return cfg
}

var (
	once = sync.Once{}
	_db  *Postgres
)

func getDB(t *testing.T) (database.Database, func()) {
	once.Do(func() {
		var err error

		dbHooks := hooks.Init()
		dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

		_db, err = NewDB(getConfig())
		require.NoError(t, err)
	})

	return _db, func() {
		if _db != nil {
			require.NoError(t, _db.truncateTables())
		}
	}
}

func (p *Postgres) truncateTables() error {
	tables := `
		convoy.event_deliveries,
		convoy.events,
		convoy.api_keys,
		convoy.subscriptions,
		convoy.source_verifiers,
		convoy.sources,
		convoy.configurations,
		convoy.devices,
		convoy.portal_links,
		convoy.organisation_invites,
		convoy.applications,
        convoy.endpoints,
		convoy.projects,
		convoy.project_configurations,
		convoy.organisation_members,
		convoy.organisations,
		convoy.users
	`

	_, err := p.dbx.ExecContext(context.Background(), fmt.Sprintf("TRUNCATE %s CASCADE;", tables))
	if err != nil {
		return err
	}

	return nil
}

func seedUser(t *testing.T, db database.Database) *datastore.User {
	t.Helper()
	userRepo := users.New(integrationLogger, db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     fmt.Sprintf("test-%s@example.com", ulid.Make().String()),
	}
	require.NoError(t, userRepo.CreateUser(context.Background(), user))
	return user
}

func seedOrg(t *testing.T, db database.Database) *datastore.Organisation {
	t.Helper()
	user := seedUser(t, db)
	orgSvc := organisations.New(integrationLogger, db)
	org := &datastore.Organisation{
		UID:            ulid.Make().String(),
		Name:           fmt.Sprintf("Test Org %s", ulid.Make().String()),
		OwnerID:        user.UID,
		CustomDomain:   null.String{},
		AssignedDomain: null.String{},
	}
	require.NoError(t, orgSvc.CreateOrganisation(context.Background(), org))
	return org
}

func seedProject(t *testing.T, db database.Database) *datastore.Project {
	t.Helper()
	org := seedOrg(t, db)
	p := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           fmt.Sprintf("job-test-project-%s", ulid.Make().String()),
		OrganisationID: org.UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}
	ps := projects.New(integrationLogger, db)
	require.NoError(t, ps.CreateProject(context.Background(), p))
	return p
}
