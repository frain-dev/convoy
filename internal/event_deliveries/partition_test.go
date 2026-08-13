package event_deliveries

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/pkg/attach"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// The conversion's whole claim is that it does not copy the table. relfilenode
// is where that claim is falsifiable: it identifies the files on disk holding a
// relation, and any rewrite allocates a new one. Row counts alone cannot tell a
// copy from an attach, and the copy path this replaces destroyed 7,099
// acknowledged deliveries on a 165 GB table precisely because it did copy.
//
// Two projects, because a bounded partition covers exactly one project under a
// key that leads with project_id. Adopting a mixed heap is the reason it is
// attached as the DEFAULT partition, and a single-project test cannot fail on
// getting that wrong.
func TestPartitionEventDeliveriesTableAdoptsTheExistingTable(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	first := seedProjectWithDelivery(t, db, service)
	second := seedProjectWithDelivery(t, db, service)

	before := countDeliveries(t, db)
	require.Equal(t, 2, before)

	original := relfilenode(t, db, "event_deliveries")

	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	require.Equal(t, before, countDeliveries(t, db),
		"rows were lost or duplicated by the conversion")

	require.Equal(t, original, relfilenode(t, db, "event_deliveries_default"),
		"the adopted partition has a different relfilenode, so the table was rewritten, not attached")

	// Both projects' history in one partition is the point of using a default.
	for _, projectID := range []string{first, second} {
		var partition string
		require.NoError(t, db.GetDB().QueryRowxContext(ctx, `
            SELECT tableoid::regclass::TEXT FROM convoy.event_deliveries WHERE project_id = $1`,
			projectID).Scan(&partition))
		require.Equal(t, "convoy.event_deliveries_default", partition)
	}
}

// gopartman's importer adopts a default partition only when the name ends in
// _default. Under any other name it skips the partition, and the provisioner
// then tries to create its own <parent>_default and collides with the one
// already attached, which breaks partition maintenance for the whole table.
func TestPartitionEventDeliveriesTableNamesTheDefaultForRetention(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	seedProjectWithDelivery(t, db, service)
	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	var isDefault bool
	require.NoError(t, db.GetDB().QueryRowxContext(ctx, `
        SELECT pg_get_expr(c.relpartbound, c.oid) = 'DEFAULT'
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'convoy' AND c.relname = 'event_deliveries_default'`).Scan(&isDefault))
	require.True(t, isDefault, "convoy.event_deliveries_default is not the default partition")
}

// Every index the table had has to end up on the parent, because a partition
// created after the conversion inherits the parent's set and nothing else. An
// index missing there is absent from exactly the partitions serving live
// queries, and an index that failed to attach stays invalid, which the planner
// ignores silently: queries keep working and get slower, on the largest table in
// the schema.
//
// Asserted against the table's own index set rather than a list written here,
// because a list written here is the bug this is guarding: the conversion used
// to carry a hardcoded set that had drifted from the schema by one name and ten
// indexes.
func TestPartitionEventDeliveriesTableCarriesEveryIndexToTheParent(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	seedProjectWithDelivery(t, db, service)
	before := indexNamesOn(t, ctx, db, "event_deliveries")
	require.Greater(t, len(before), 10, "fixture has too few indexes to prove anything")

	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	after := indexNamesOn(t, ctx, db, "event_deliveries")
	require.Equal(t, before, after,
		"the partitioned parent does not carry the same indexes the table had, so partitions created from it will be missing them")

	var invalid []string
	require.NoError(t, db.GetDB().SelectContext(ctx, &invalid, `
        SELECT c.relname
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'convoy'
          AND t.relname IN ('event_deliveries', 'event_deliveries_default')
          AND NOT i.indisvalid
        ORDER BY c.relname`))
	require.Empty(t, invalid, "indexes did not attach and are invalid, so the planner will not use them")
}

// indexNamesOn returns the sorted index names on a relation, ignoring the
// primary key, whose name necessarily changes: an ordinary table keys on id and
// a partitioned one has to include the partition key columns.
func indexNamesOn(t *testing.T, ctx context.Context, db database.Database, table string) []string {
	t.Helper()

	var names []string
	require.NoError(t, db.GetDB().SelectContext(ctx, &names, `
        SELECT c.relname
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_class t ON t.oid = i.indrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE n.nspname = 'convoy' AND t.relname = $1 AND NOT i.indisprimary
        ORDER BY c.relname`, table))
	return names
}

// A row past the cutoff that matches no partition must fail rather than be
// absorbed by the default. Without the bounds constraint a default partition
// takes everything unclaimed, so live rows would accumulate in the partition
// retention is built to drop whole, and the failure would surface as data loss
// at retention time instead of an error at write time.
func TestPartitionEventDeliveriesTableRejectsRowsBeyondTheLastPartition(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	projectID := seedProjectWithDelivery(t, db, service)
	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	beyond := attach.Cutoff(time.Now()).AddDate(0, 0, attach.PremakeDays+5)
	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.event_deliveries
            (id, status, description, project_id, event_id, subscription_id, metadata, created_at)
        VALUES ('beyond', 'Success', '', $1, 'e', 's', '{}'::jsonb, $2)`, projectID, beyond)
	require.Error(t, err, "a delivery beyond the last partition was accepted")
}

// Each forward partition Postgres adds must prove no row in the default belongs
// to it, and proves that either from the default's constraints or by scanning
// it. On the adopted heap the difference is between a catalog change and reading
// the whole table, once per partition per project, so the constraint doing the
// refuting is load-bearing rather than an optimisation.
// The history CHECK is a UTC timestamptz. Deriving the first forward day with
// `'…'::TIMESTAMPTZ::DATE` follows the session TimeZone and lands on the
// previous calendar day in America/New_York, so CREATE PARTITION cannot refute
// overlap from the CHECK and scans the adopted heap.
func TestForwardPartitionDayFollowsUTCCutoff(t *testing.T) {
	_, db := setupTestDB(t)
	ctx := context.Background()

	bound := attach.Cutoff(time.Date(2026, 8, 15, 3, 0, 0, 0, time.UTC)).Format(time.RFC3339)
	_, err := db.GetDB().ExecContext(ctx, `SET TIME ZONE 'America/New_York'`)
	require.NoError(t, err)

	var naive, utc string
	require.NoError(t, db.GetDB().QueryRowxContext(ctx, `
        SELECT
            ($1::TIMESTAMPTZ)::DATE::TEXT,
            ($1::TIMESTAMPTZ AT TIME ZONE 'UTC')::DATE::TEXT`, bound).Scan(&naive, &utc))

	require.Equal(t, "2026-08-28", naive)
	require.Equal(t, "2026-08-29", utc)
}

func TestCreatingPartitionsDoesNotScanTheAdoptedTable(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	projectID := seedProjectWithDelivery(t, db, service)
	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	before := seqScans(t, db, "event_deliveries_default")

	day := attach.Cutoff(time.Now()).AddDate(0, 0, attach.PremakeDays+1)
	_, err := db.GetDB().ExecContext(ctx, fmt.Sprintf(`
        CREATE TABLE convoy.%s PARTITION OF convoy.event_deliveries
            FOR VALUES FROM ('%s', '%s') TO ('%s', '%s')`,
		"event_deliveries_scan_probe", projectID, day.Format(time.DateOnly),
		projectID, day.AddDate(0, 0, 1).Format(time.DateOnly)))
	require.NoError(t, err)

	require.Equal(t, before, seqScans(t, db, "event_deliveries_default"),
		"adding a partition scanned the adopted table, so the bounds constraint stopped refuting the new range")
}

func TestPartitionEventDeliveriesTableInstallsEventStandIn(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	seedProjectWithDelivery(t, db, service)
	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	var triggers int
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
        SELECT count(*)
        FROM pg_catalog.pg_trigger t
        JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid
        JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'convoy'
          AND c.relname = 'event_deliveries'
          AND t.tgname = 'event_fk_check'`).Scan(&triggers))
	require.Equal(t, 1, triggers, "event_fk_check was not installed after attach")
}

// The swap drops delivery_attempts' foreign key, because it points at a primary
// key that a partitioned table cannot keep, and installs a trigger in its place
// in the same transaction. Any gap between the two is a window where an attempt
// can be written against a delivery that does not exist, and nothing later in
// the conversion would notice.
func TestPartitionEventDeliveriesTableKeepsDeliveryAttemptEnforcement(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, delivery))

	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	require.Error(t, insertAttempt(t, db, project.UID, endpoint.UID, ulid.Make().String()),
		"an attempt referencing a nonexistent delivery was accepted, so the conversion left delivery_attempts unenforced")

	// A real one still goes in, so the test cannot pass on a mechanism that
	// rejects everything.
	require.NoError(t, insertAttempt(t, db, project.UID, endpoint.UID, delivery.UID))
}

// Operators convert tables one at a time. Unpartitioning event_deliveries while
// delivery_attempts is still a partitioned parent cannot restore a real FK on
// (event_delivery_id) alone; that ALTER fails, and the run is stuck.
func TestUnPartitionEventDeliveriesTableWhileAttemptsArePartitioned(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, delivery))

	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))
	require.NoError(t, delivery_attempts.New(log.New("convoy", log.LevelError), db).PartitionDeliveryAttemptsTable(ctx))
	require.NoError(t, service.UnPartitionEventDeliveriesTable(ctx))

	require.Error(t, insertAttempt(t, db, project.UID, endpoint.UID, ulid.Make().String()),
		"an attempt referencing a nonexistent delivery was accepted after unpartitioning event_deliveries")
	require.NoError(t, insertAttempt(t, db, project.UID, endpoint.UID, delivery.UID))
}

// Same one-at-a-time order as events-after-deliveries: attach keeps the real
// FK on delivery_attempts_default, so Swap has to drop that child too.
func TestPartitionEventDeliveriesTableDropsAdoptedAttemptFK(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
	delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, delivery))

	require.NoError(t, delivery_attempts.New(log.New("convoy", log.LevelError), db).PartitionDeliveryAttemptsTable(ctx))
	require.Equal(t, 1, countNamedConstraint(t, db, "delivery_attempts_default", "delivery_attempts_event_delivery_id_fkey"),
		"precondition: attach left the attempt FK on the adopted child")

	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))
	require.Zero(t, countNamedConstraint(t, db, "delivery_attempts_default", "delivery_attempts_event_delivery_id_fkey"),
		"partitioning event_deliveries left the attempt FK on delivery_attempts_default")
}

// Detach swaps the adopted heap back under the live name before it copies
// rows written since conversion. Those rows sit on <table>_partitioned for
// the whole drain. The stand-in must find them there, or a delivery for a
// post-conversion event is rejected until drain finishes.
func TestEventFKAcceptsEventOnlyOnPartitionedDuringDrain(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
	require.NoError(t, service.CreateEventDelivery(ctx,
		createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)))

	require.NoError(t, events.New(log.New("convoy", log.LevelError), db).PartitionEventsTable(ctx))

	recentID := ulid.Make().String()
	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.events (id, event_type, project_id, source_id, raw, data, created_at, updated_at)
        VALUES ($1, 'test.event', $2, $3, '{}', '{}', $4, $4)`,
		recentID, project.UID, source.UID, attach.Cutoff(time.Now()))
	require.NoError(t, err)

	simulateDetachSwap(t, db, "events")
	_, err = db.GetDB().ExecContext(ctx, attach.EventFKSQL)
	require.NoError(t, err)

	var home string
	require.NoError(t, db.GetDB().QueryRowContext(ctx, `
        SELECT tableoid::regclass::TEXT FROM convoy.events_partitioned WHERE id = $1`, recentID).Scan(&home))
	require.NotEqual(t, "convoy.events", home)

	require.NoError(t, service.CreateEventDelivery(ctx,
		createTestEventDelivery(t, project.UID, recentID, endpoint.UID, subscription.UID)),
		"delivery for an event that only exists on events_partitioned was rejected")
	require.Error(t, service.CreateEventDelivery(ctx,
		createTestEventDelivery(t, project.UID, ulid.Make().String(), endpoint.UID, subscription.UID)),
		"delivery referencing a nonexistent event was accepted during drain")
}

// Same drain window on event_deliveries: an attempt for a post-conversion
// delivery must land, and the live table must reject an orphan.
func TestAttemptFKAcceptsDeliveryOnlyOnPartitionedDuringDrain(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
	delivery := createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)
	require.NoError(t, service.CreateEventDelivery(ctx, delivery))

	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	recentID := ulid.Make().String()
	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.event_deliveries
            (id, status, description, project_id, event_id, endpoint_id, subscription_id, metadata, created_at, delivery_mode)
        VALUES ($1, 'Success', '', $2, $3, $4, $5, '{}'::jsonb, $6, 'at_least_once')`,
		recentID, project.UID, event.UID, endpoint.UID, subscription.UID, attach.Cutoff(time.Now()))
	require.NoError(t, err)

	simulateDetachSwap(t, db, "event_deliveries")
	_, err = db.GetDB().ExecContext(ctx, attach.EventFKSQL)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, attach.AttemptFKSQL)
	require.NoError(t, err)

	require.NoError(t, insertAttempt(t, db, project.UID, endpoint.UID, recentID),
		"attempt for a delivery that only exists on event_deliveries_partitioned was rejected")
	require.Error(t, insertAttempt(t, db, project.UID, endpoint.UID, ulid.Make().String()),
		"attempt referencing a nonexistent delivery was accepted during drain")
}

func simulateDetachSwap(t *testing.T, db database.Database, table string) {
	t.Helper()

	_, err := db.GetDB().ExecContext(context.Background(), fmt.Sprintf(`
        ALTER TABLE convoy.%[1]s RENAME TO %[1]s_partitioned;
        ALTER TABLE convoy.%[1]s_partitioned DETACH PARTITION convoy.%[1]s_default;
        ALTER TABLE convoy.%[1]s_default RENAME TO %[1]s;
        ALTER TABLE convoy.%[1]s DROP CONSTRAINT IF EXISTS %[1]s_default_bounds;`, table))
	require.NoError(t, err)
}

func TestUnPartitionEventDeliveriesTableKeepsPostConversionDelivery(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
	require.NoError(t, service.CreateEventDelivery(ctx,
		createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)))
	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	recentID := ulid.Make().String()
	_, err := db.GetDB().ExecContext(ctx, `
        INSERT INTO convoy.event_deliveries
            (id, status, description, project_id, event_id, endpoint_id, subscription_id, metadata, created_at, delivery_mode)
        VALUES ($1, 'Success', '', $2, $3, $4, $5, '{}'::jsonb, $6, 'at_least_once')`,
		recentID, project.UID, event.UID, endpoint.UID, subscription.UID, attach.Cutoff(time.Now()))
	require.NoError(t, err)

	require.NoError(t, service.UnPartitionEventDeliveriesTable(ctx))
	require.NoError(t, insertAttempt(t, db, project.UID, endpoint.UID, recentID))
	require.Error(t, insertAttempt(t, db, project.UID, endpoint.UID, ulid.Make().String()))
}

func TestUnPartitionEventDeliveriesTableRecoversInvalidIdIndex(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()

	seedProjectWithDelivery(t, db, service)
	require.NoError(t, service.PartitionEventDeliveriesTable(ctx))

	_, err := db.GetDB().ExecContext(ctx, `
        CREATE UNIQUE INDEX event_deliveries_id_key ON convoy.event_deliveries_default (id)`)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx, `
        UPDATE pg_index SET indisvalid = false
        WHERE indexrelid = 'convoy.event_deliveries_id_key'::regclass`)
	require.NoError(t, err)

	require.NoError(t, service.UnPartitionEventDeliveriesTable(ctx))
}

func countNamedConstraint(t *testing.T, db database.Database, table, name string) int {
	t.Helper()

	var n int
	require.NoError(t, db.GetDB().QueryRowContext(context.Background(), `
        SELECT count(*)
        FROM pg_constraint con
        JOIN pg_class c ON c.oid = con.conrelid
        JOIN pg_namespace ns ON ns.oid = c.relnamespace
        WHERE ns.nspname = 'convoy' AND c.relname = $1 AND con.conname = $2`,
		table, name).Scan(&n))
	return n
}

func insertAttempt(t *testing.T, db database.Database, projectID, endpointID, deliveryID string) error {
	t.Helper()

	_, err := db.GetDB().ExecContext(context.Background(), `
        INSERT INTO convoy.delivery_attempts (id, url, method, api_version, project_id, endpoint_id, event_delivery_id)
        VALUES ($1, 'https://example.com', 'POST', '2024-04-01', $2, $3, $4)`,
		ulid.Make().String(), projectID, endpointID, deliveryID)
	return err
}

func seedProjectWithDelivery(t *testing.T, db database.Database, service *Service) string {
	t.Helper()

	project := seedTestProject(t, db)
	endpoint := seedTestEndpoint(t, db, project.UID)
	source := seedTestSource(t, db, project.UID)
	subscription := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
	event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)

	require.NoError(t, service.CreateEventDelivery(context.Background(),
		createTestEventDelivery(t, project.UID, event.UID, endpoint.UID, subscription.UID)))

	return project.UID
}

func countDeliveries(t *testing.T, db database.Database) int {
	t.Helper()

	var count int
	require.NoError(t, db.GetDB().QueryRowxContext(context.Background(),
		`SELECT count(*) FROM convoy.event_deliveries`).Scan(&count))
	return count
}

func relfilenode(t *testing.T, db database.Database, table string) int64 {
	t.Helper()

	var node int64
	require.NoError(t, db.GetDB().QueryRowxContext(context.Background(), `
        SELECT c.relfilenode
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'convoy' AND c.relname = $1`, table).Scan(&node))
	return node
}

// seqScans reads how many sequential scans a relation has taken, once the
// number has stopped moving.
//
// A backend reports its statistics when its transaction ends, and at most once a
// second, and reports them to shared memory that another connection reads. A
// value read immediately after some work can therefore be missing that work
// entirely, and the work then lands in whatever is measured next. Waiting for
// the counter to settle is what keeps a scan done by the conversion from being
// attributed to the statement under test.
func seqScans(t *testing.T, db database.Database, table string) int64 {
	t.Helper()

	const interval = 1200 * time.Millisecond
	last := int64(-1)

	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); {
		var scans int64
		require.NoError(t, db.GetDB().QueryRowxContext(context.Background(),
			`SELECT pg_stat_get_numscans(('convoy.' || $1)::regclass)`, table).Scan(&scans))

		if scans == last {
			return scans
		}
		last = scans
		time.Sleep(interval)
	}

	t.Fatalf("scan count for convoy.%s never settled", table)
	return 0
}
