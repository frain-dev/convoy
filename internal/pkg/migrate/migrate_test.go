//go:build integration
// +build integration

package migrate

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	migrations = []*Migration{
		{
			ID: "201608301400",
			Migrate: func(db *mongo.Database) error {
				return nil
			},
			Rollback: func(db *mongo.Database) error {
				return nil
			},
		},
		{
			ID: "201608301430",
			Migrate: func(db *mongo.Database) error {
				return nil
			},
			Rollback: func(db *mongo.Database) error {
				return nil
			},
		},
	}

	extendedMigrations = append(migrations, &Migration{
		ID: "201807221927",
		Migrate: func(db *mongo.Database) error {
			return nil
		},
		Rollback: func(db *mongo.Database) error {
			return nil
		},
	})

	failingMigration = []*Migration{
		{
			ID: "201904231300",
			Migrate: func(db *mongo.Database) error {
				return nil
			},
			Rollback: func(db *mongo.Database) error {
				return nil
			},
		},
	}

	fakeInitSchema = func(ctx context.Context, db *mongo.Database) (bool, error) {
		return false, nil
	}
)

type Person struct {
	Name string
}

type Pet struct {
	Name     string
	PersonID int
}

type Book struct {
	Name     string
	PersonID int
}

func TestMigration(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	opts := &Options{DatabaseName: getDBName(t)}
	m := NewMigrator(db.Client(), opts, migrations, fakeInitSchema)

	err := m.Migrate(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(2), tableCount(t, db, opts.CollectionName))

	err = m.RollbackLast(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(1), tableCount(t, db, opts.CollectionName))

	err = m.RollbackLast(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(0), tableCount(t, db, opts.CollectionName))
}

func TestMigrateTo(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	opts := &Options{DatabaseName: getDBName(t)}
	m := NewMigrator(db.Client(), opts, extendedMigrations, fakeInitSchema)

	err := m.MigrateTo(context.Background(), "201608301430")
	assert.NoError(t, err)
	assert.Equal(t, int64(3), tableCount(t, db, opts.CollectionName))
}

func TestRollbackTo(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	opts := &Options{DatabaseName: getDBName(t)}
	m := NewMigrator(db.Client(), opts, extendedMigrations, fakeInitSchema)

	// First, apply all migrations.
	err := m.Migrate(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(3), tableCount(t, db, opts.CollectionName))

	// Rollback to the first migrations: only the last 2 migrations are expected to be rolled back.
	err = m.RollbackTo(context.Background(), "201608301400")
	assert.NoError(t, err)
	assert.Equal(t, int64(1), tableCount(t, db, opts.CollectionName))
}

func TestMigrationIDDoesNotExist(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	opts := &Options{DatabaseName: getDBName(t)}
	m := NewMigrator(db.Client(), opts, extendedMigrations, fakeInitSchema)
	ctx := context.Background()

	assert.Equal(t, ErrMigrationIDDoesNotExist, m.MigrateTo(ctx, "1234"))
	assert.Equal(t, ErrMigrationIDDoesNotExist, m.RollbackTo(ctx, "1234"))
	assert.Equal(t, ErrMigrationIDDoesNotExist, m.MigrateTo(ctx, ""))
	assert.Equal(t, ErrMigrationIDDoesNotExist, m.RollbackTo(ctx, ""))
}

func TestMissingID(t *testing.T) {
	migrationsMissingID := []*Migration{
		{
			Migrate: func(db *mongo.Database) error {
				return nil
			},
		},
	}

	db, closeFn := getDB(t)
	defer closeFn()

	opts := &Options{DatabaseName: getDBName(t)}
	m := NewMigrator(db.Client(), opts, migrationsMissingID, fakeInitSchema)
	assert.Equal(t, ErrMissingID, m.Migrate(context.Background()))
}

func TestDuplicatedID(t *testing.T) {
	migrationsDuplicatedID := []*Migration{
		{
			ID: "201705061500",
			Migrate: func(db *mongo.Database) error {
				return nil
			},
		},
		{
			ID: "201705061500",
			Migrate: func(db *mongo.Database) error {
				return nil
			},
		},
	}

	db, closeFn := getDB(t)
	defer closeFn()

	opts := &Options{DatabaseName: getDBName(t)}
	m := NewMigrator(db.Client(), opts, migrationsDuplicatedID, fakeInitSchema)
	_, isDuplicatedIDError := m.Migrate(context.Background()).(*DuplicatedIDError)
	assert.True(t, isDuplicatedIDError)
}

func TestCheckPendingMigrations(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	opts := &Options{DatabaseName: getDBName(t)}
	m := NewMigrator(db.Client(), opts, migrations, fakeInitSchema)

	// First apply migrations
	err := m.Migrate(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(2), tableCount(t, db, opts.CollectionName))

	// Check against pending migrations
	pm := NewMigrator(db.Client(), opts, extendedMigrations, fakeInitSchema)
	pendingMigrations, err := pm.CheckPendingMigrations(context.Background())
	assert.NoError(t, err)
	assert.True(t, pendingMigrations)

	// Apply pending migrations
	err = pm.Migrate(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, int64(3), tableCount(t, db, opts.CollectionName))

	// Check pending status again
	pendingMigrations, err = pm.CheckPendingMigrations(context.Background())
	assert.NoError(t, err)
	assert.False(t, pendingMigrations)

}

func tableCount(t *testing.T, db *mongo.Database, collectionName string) int64 {
	store := datastore.New(db)
	filter := map[string]interface{}{}
	count, err := store.Count(context.Background(), filter)
	assert.NoError(t, err)

	return count
}

func getDSN() string {
	return os.Getenv("TEST_MONGO_DSN")
}

func getDBName(t *testing.T) string {
	u, err := url.Parse(getDSN())
	assert.NoError(t, err)

	return strings.TrimPrefix(u.Path, "/")
}

func getConfig() config.Configuration {

	return config.Configuration{
		Database: config.DatabaseConfiguration{
			Type: config.MongodbDatabaseProvider,
			Dsn:  getDSN(),
		},
	}
}

func getDB(t *testing.T) (*mongo.Database, func()) {
	db, err := cm.New(getConfig())
	require.NoError(t, err)

	client := db.Client().(*mongo.Database)

	return client, func() {
		require.NoError(t, client.Drop(context.TODO()))
		require.NoError(t, db.Disconnect(context.Background()))
	}
}
