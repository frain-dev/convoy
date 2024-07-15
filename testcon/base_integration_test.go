package testcon

import (
	"fmt"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	pg "github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"os"
	"strconv"
	"sync"
	"testing"
)

var once sync.Once
var pDB *pg.Postgres

func setEnv(dbPort int, redisPort int) {
	_ = os.Setenv("CONVOY_REDIS_HOST", "localhost")
	_ = os.Setenv("CONVOY_REDIS_SCHEME", "redis")
	_ = os.Setenv("CONVOY_REDIS_PORT", strconv.Itoa(redisPort))

	_ = os.Setenv("CONVOY_DB_HOST", "localhost")
	_ = os.Setenv("CONVOY_DB_SCHEME", "postgres")
	_ = os.Setenv("CONVOY_DB_USERNAME", "convoy")
	_ = os.Setenv("CONVOY_DB_PASSWORD", "convoy")
	_ = os.Setenv("CONVOY_DB_DATABASE", "convoy")
	_ = os.Setenv("CONVOY_DB_PORT", strconv.Itoa(dbPort))
}

func getConfig() config.Configuration {
	err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
		log.Fatal(err)
	}

	return cfg
}

func getDB() database.Database {
	once.Do(func() {
		db, err := pg.NewDB(getConfig())
		if err != nil {
			panic(fmt.Sprintf("failed to connect to db: %v", err))
		}
		_ = os.Setenv("TZ", "") // Use UTC by default :)

		dbHooks := hooks.Init()
		dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}, changelog interface{}) {})

		pDB = db
	})
	return pDB
}

func setupSourceSubscription(t *testing.T, db database.Database, project *datastore.Project) *datastore.Subscription {
	sourceID := ulid.Make().String()
	vc := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}
	source, err := testdb.SeedSource(db, project, sourceID, ulid.Make().String(), "", vc, "", "")
	require.NoError(t, err)

	endpoint, err := testdb.SeedEndpoint(db, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(t, err)

	sub, err := testdb.SeedSubscription(db, project, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(t, err)

	return sub
}

func initRealmChain(t *testing.T, apiKeyRepo datastore.APIKeyRepository, userRepo datastore.UserRepository, portalLinkRepo datastore.PortalLinkRepository, cache cache.Cache) error {
	cfg, err := config.Get()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, cache)
	if err != nil {
		t.Errorf("failed to initialize realm chain : %v", err)
	}
	return err
}
