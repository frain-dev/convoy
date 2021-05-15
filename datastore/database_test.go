// +build integration

package datastore

import (
	"database/sql"
	"log"
	"os"
	"testing"

	testfixtures "github.com/go-testfixtures/testfixtures/v3"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var (
	db       *sql.DB
	fixtures *testfixtures.Loader
)

func getDSN() string {
	return os.Getenv("TEST_DATABASE_DSN")
}

func getConfig() config.Configuration {

	var t config.DatabaseProvider

	switch os.Getenv("TEST_DB_TYPE") {
	case "postgres":
		t = config.PostgresDatabaseProvider
	case "mysql":
		t = config.MysqlDatabaseProvider
	default:
		t = config.DatabaseProvider(os.Getenv("TEST_DB_TYPE"))
	}

	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Type: t,
			Dsn:  getDSN(),
		},
	}
}

func TestMain(m *testing.M) {
	var err error

	cfg := getConfig()

	db, err := New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// if err := db.Ping(); err != nil {
	// 	log.Fatalf("error occurred while pinging DB... %v", err)
	// }

	err = db.AutoMigrate(hookcamp.Organisation{},
		hookcamp.Application{}, hookcamp.Endpoint{})
	if err != nil {
		log.Fatal(err)
	}

	fn, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}

	fixtures, err = testfixtures.New(
		testfixtures.Database(fn),
		testfixtures.Dialect(cfg.Database.Type.String()),
		testfixtures.Directory("testdata"),
	)

	if err != nil {
		log.Fatal(err)
	}

	os.Exit(m.Run())
}

func prepareTestDatabase(t *testing.T) {
	t.Helper()
	require.NoError(t, fixtures.Load())
}

func getDB(t *testing.T) (*gorm.DB, func()) {
	prepareTestDatabase(t)

	db, err := New(getConfig())
	require.NoError(t, err)

	return db, func() {

		inner, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}

		require.NoError(t, inner.Close())
	}
}
