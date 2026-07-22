package retention

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/events"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}

	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
		os.Exit(1)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (database.Database, context.Context) {
	t.Helper()

	if testEnv == nil {
		t.Fatal("testEnv is nil - TestMain may not have run successfully")
	}

	ctx := context.Background()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	db := postgres.NewFromConnection(conn)

	return db, ctx
}

func TestUnpartitionedTables(t *testing.T) {
	db, ctx := setupTestDB(t)

	// Fresh migrations create plain tables; every retention table is missing.
	missing, err := UnpartitionedTables(ctx, db)
	require.NoError(t, err)
	require.ElementsMatch(t, RetentionTables, missing)

	// Partition the events tables; they must drop out of the missing list.
	eventsRepo := events.New(log.New("convoy", log.LevelInfo), db)
	require.NoError(t, eventsRepo.PartitionEventsTable(ctx))
	require.NoError(t, eventsRepo.PartitionEventsSearchTable(ctx))

	missing, err = UnpartitionedTables(ctx, db)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"event_deliveries", "delivery_attempts"}, missing)
}
