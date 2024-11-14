//go:build integration
// +build integration

package sqlite3

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"os"
	"sync"
	"testing"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"

	"github.com/stretchr/testify/require"
)

var (
	once = sync.Once{}
	_db  *Sqlite
)

func getDB(t *testing.T) (database.Database, func()) {
	once.Do(func() {
		var err error

		dbHooks := hooks.Init()
		dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}, changelog interface{}) {})

		_db, err = NewDB("test.db?cache=shared", log.NewLogger(os.Stdout))
		require.NoError(t, err)

		// run migrations
		m := migrator.New(_db, "sqlite3")
		err = m.Up()
		require.NoError(t, err)
	})

	return _db, func() {
		require.NoError(t, _db.truncateTables())
	}
}

func (s *Sqlite) truncateTables() error {
	tables := []string{
		"event_deliveries",
		"events",
		"api_keys",
		"subscriptions",
		"source_verifiers",
		"sources",
		"configurations",
		"devices",
		"portal_links",
		"organisation_invites",
		"applications",
		"endpoints",
		"projects",
		"project_configurations",
		"organisation_members",
		"organisations",
		"users",
	}

	for _, table := range tables {
		_, err := s.dbx.ExecContext(context.Background(), fmt.Sprintf("delete from %s;", table))
		if err != nil {
			return err
		}
	}

	return nil
}
