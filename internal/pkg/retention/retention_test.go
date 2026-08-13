package retention

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
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

// Partitioning a table is only half of enabling retention: gopartman still has
// to adopt the existing children into partman.partitions, and it only adopts
// the ones whose name it can parse. Adoption failures are reported rather than
// returned as errors, so nothing fails loudly when this breaks. The observable
// symptom is that retention runs forever and drops nothing, which is why this
// asserts on partman's own bookkeeping rather than on the partition names.
func TestRegisterParentsAdoptsExistingPartitions(t *testing.T) {
	db, ctx := setupTestDB(t)

	projectID := seedProjectWithEvent(t, db)

	eventsRepo := events.New(log.New("convoy", log.LevelInfo), db)
	require.NoError(t, eventsRepo.PartitionEventsTable(ctx))

	policy, err := NewPartitionRetentionPolicy(db, log.New("convoy", log.LevelInfo), 24*time.Hour)
	require.NoError(t, err)

	policy.registerParents(ctx)

	var adopted int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
        SELECT count(*)
        FROM partman.partitions p
        JOIN partman.parent_tables t ON t.id = p.parent_table_id
        WHERE t.table_name = 'events' AND p.tenant_id = $1`, projectID).Scan(&adopted))

	require.NotZero(t, adopted,
		"retention adopted no partitions for convoy.events tenant %s, so it will never drop anything",
		projectID)
}

// seedProjectWithEvent inserts the minimum chain convoy.events requires
// (user, organisation, project configuration, project) plus one event, because
// the partition helpers only create children for days that already hold rows.
func seedProjectWithEvent(t *testing.T, db database.Database) string {
	t.Helper()

	ctx := context.Background()
	userID := ulid.Make().String()
	orgID := ulid.Make().String()
	configID := ulid.Make().String()
	projectID := ulid.Make().String()

	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.users (id, first_name, last_name, email, password, email_verified)
        VALUES ($1, 'retention', 'test', $2, 'x', true)`, userID, userID+"@example.com")
	require.NoError(t, err)

	_, err = db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.organisations (id, name, owner_id) VALUES ($1, 'retention-test', $2)`, orgID, userID)
	require.NoError(t, err)

	_, err = db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.project_configurations (
            id, max_payload_read_size, replay_attacks_prevention_enabled,
            ratelimit_count, ratelimit_duration, strategy_type, strategy_duration,
            strategy_retry_count, signature_header, signature_versions)
        VALUES ($1, 51200, false, 1000, 60, 'linear', 100, 10, 'X-Convoy-Signature', '[]'::jsonb)`, configID)
	require.NoError(t, err)

	_, err = db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.projects (id, name, type, organisation_id, project_configuration_id)
        VALUES ($1, 'retention-test', 'outgoing', $2, $3)`, projectID, orgID, configID)
	require.NoError(t, err)

	_, err = db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.events (id, event_type, project_id, raw, data, url_path)
        VALUES ($1, 'test.event', $2, '{}', '{}'::bytea, '/')`, ulid.Make().String(), projectID)
	require.NoError(t, err)

	return projectID
}
