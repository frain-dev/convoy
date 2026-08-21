package indexes

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// These are what pg_get_indexdef returns for real indexes in this schema. A
// partial index and a unique partial one are the two shapes a rebuild has to
// reproduce exactly: drop either the WHERE or the UNIQUE and the index built is
// not the index that was dropped.
const (
	statusCreatedDef = `CREATE INDEX idx_event_deliveries_endpoint_status_created ON convoy.event_deliveries ` +
		`USING btree (project_id, endpoint_id, status, created_at) WHERE (deleted_at IS NULL)`
	uniqueDef = `CREATE UNIQUE INDEX idx_partition_runs_single_active ON convoy.partition_runs ` +
		`USING btree (status) WHERE (status = 'running'::text)`
)

func TestConcurrently(t *testing.T) {
	stmt, err := concurrently(statusCreatedDef)
	require.NoError(t, err)
	require.Equal(t, `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_event_deliveries_endpoint_status_created `+
		`ON convoy.event_deliveries USING btree (project_id, endpoint_id, status, created_at) `+
		`WHERE (deleted_at IS NULL)`, stmt)
}

func TestConcurrentlyKeepsUnique(t *testing.T) {
	stmt, err := concurrently(uniqueDef)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(stmt, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS "), stmt)
}

// Uniqueness is the one thing a dropped index costs beyond speed, so the report
// has to be able to tell those apart.
func TestUniqueIsReadFromTheDefinition(t *testing.T) {
	require.True(t, Dropped{Definition: uniqueDef}.Unique())
	require.False(t, Dropped{Definition: statusCreatedDef}.Unique())
}

// A definition that cannot be read must stop the rebuild. Guessing at it would
// build an index that is not the one that was dropped.
func TestConcurrentlyRejectsUnreadableDefinition(t *testing.T) {
	_, err := concurrently(`ALTER TABLE convoy.events ADD COLUMN x TEXT`)
	require.Error(t, err)
}

func TestParentStatementCoversNoRowsYet(t *testing.T) {
	stmt, err := parentStatement("idx_event_deliveries_endpoint_status_created", "event_deliveries", statusCreatedDef)
	require.NoError(t, err)
	require.Equal(t, `CREATE INDEX IF NOT EXISTS "idx_event_deliveries_endpoint_status_created" `+
		`ON ONLY "convoy"."event_deliveries" USING btree (project_id, endpoint_id, status, created_at) `+
		`WHERE (deleted_at IS NULL)`, stmt)

	// ON ONLY is what keeps this from recursing into every partition under an
	// exclusive lock, and CONCURRENTLY is not supported here.
	require.Contains(t, stmt, " ON ONLY ")
	require.NotContains(t, stmt, "CONCURRENTLY")
}

func TestChildStatementsBuildConcurrentlyThenAttach(t *testing.T) {
	const partition = "event_deliveries_01J8ZQ4T6K9WCS0M3XN7HB2VDR_20260819"

	create, attach, err := childStatements("idx_event_deliveries_endpoint_status_created", partition, statusCreatedDef)
	require.NoError(t, err)

	child := childName("idx_event_deliveries_endpoint_status_created", partition)
	require.Contains(t, create, "CREATE INDEX CONCURRENTLY IF NOT EXISTS ")
	require.Contains(t, create, `ON "convoy".`+`"`+partition+`"`)
	require.Contains(t, create, "WHERE (deleted_at IS NULL)")
	require.Equal(t, `ALTER INDEX "convoy"."idx_event_deliveries_endpoint_status_created" `+
		`ATTACH PARTITION "convoy"."`+child+`"`, attach)
}

func TestChildStatementsCarryUnique(t *testing.T) {
	create, _, err := childStatements("idx_partition_runs_single_active", "partition_runs_20260819", uniqueDef)
	require.NoError(t, err)

	// A child index of a unique parent has to be unique too, or the attach is
	// rejected and the parent never turns valid.
	require.Contains(t, create, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ")
}

func TestChildNameFitsAnIdentifier(t *testing.T) {
	long := "idx_event_deliveries_project_endpoint_status_created_at_partial_covering"
	require.Greater(t, len(long), maxIdentifier)

	name := childName(long, "event_deliveries_01J8ZQ4T6K9WCS0M3XN7HB2VDR_20260819")
	require.LessOrEqual(t, len(name), maxIdentifier)
}

// Partition names differ at the end, in the date, which is exactly what a
// truncated name loses. Two partitions of one table must not derive the same
// child name: IF NOT EXISTS would then skip the second build and attach the
// first partition's index in its place.
func TestChildNameSeparatesPartitionsThatShareAPrefix(t *testing.T) {
	const index = "idx_event_deliveries_endpoint_status_created_at_partial_covering"

	first := childName(index, "event_deliveries_01J8ZQ4T6K9WCS0M3XN7HB2VDR_20260819")
	second := childName(index, "event_deliveries_01J8ZQ4T6K9WCS0M3XN7HB2VDR_20260820")

	require.NotEqual(t, first, second)
	require.LessOrEqual(t, len(first), maxIdentifier)
	require.LessOrEqual(t, len(second), maxIdentifier)
}

// A resumed rebuild derives names again rather than reading them back, so the
// same inputs have to give the same name.
func TestChildNameIsStable(t *testing.T) {
	first := childName("idx_events_created_at", "events_01J8ZQ4T6K9WCS0M3XN7HB2VDR_20260819")
	second := childName("idx_events_created_at", "events_01J8ZQ4T6K9WCS0M3XN7HB2VDR_20260819")
	require.Equal(t, first, second)
}

func TestIndexShapeRejectsUnreadableDefinition(t *testing.T) {
	_, _, err := indexShape(`CREATE INDEX broken ON convoy.events (created_at)`)
	require.Error(t, err)
}

func TestPayloadGINDefinitionIsRebuildable(t *testing.T) {
	stmt, err := concurrently(PayloadGINDefinition)
	require.NoError(t, err)
	require.Equal(t, `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_events_payload_gin `+
		`ON convoy.events USING gin (convoy.event_payload_jsonb(data) jsonb_path_ops) `+
		`WHERE (deleted_at IS NULL)`, stmt)

	parent, err := parentStatement(PayloadGIN, "events", PayloadGINDefinition)
	require.NoError(t, err)
	require.Contains(t, parent, " ON ONLY ")
	require.NotContains(t, parent, "CONCURRENTLY")
}

func TestPayloadGINDefinitionMatchesMigration(t *testing.T) {
	body, err := os.ReadFile("../../../sql/1787200001.sql")
	require.NoError(t, err)
	require.Contains(t, string(body), PayloadGINDefinition)
	require.NotRegexp(t, `(?m)^CREATE INDEX`, string(body))
}

func TestEventDeliveriesProjectCreatedDefinitionIsRebuildable(t *testing.T) {
	stmt, err := concurrently(EventDeliveriesProjectCreatedDefinition)
	require.NoError(t, err)
	require.Equal(t, `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_event_deliveries_project_created_id_deleted `+
		`ON convoy.event_deliveries USING btree (project_id, created_at DESC, id DESC) `+
		`WHERE (deleted_at IS NULL)`, stmt)

	parent, err := parentStatement(EventDeliveriesProjectCreated, "event_deliveries", EventDeliveriesProjectCreatedDefinition)
	require.NoError(t, err)
	require.Contains(t, parent, " ON ONLY ")
	require.NotContains(t, parent, "CONCURRENTLY")
}

func TestEventDeliveriesProjectCreatedDefinitionMatchesMigration(t *testing.T) {
	body, err := os.ReadFile("../../../sql/1787251200.sql")
	require.NoError(t, err)
	require.Contains(t, string(body), EventDeliveriesProjectCreatedDefinition)
	require.NotRegexp(t, `(?m)^CREATE INDEX`, string(body))
}

func TestEventPayloadJsonbIsParallelUnsafe(t *testing.T) {
	for _, name := range []string{"../../../sql/1787200000.sql", "../../../sql/1787200002.sql"} {
		body, err := os.ReadFile(name)
		require.NoError(t, err)
		up := strings.SplitN(string(body), "-- +migrate Down", 2)[0]
		require.Contains(t, up, "PARALLEL UNSAFE")
		require.NotRegexp(t, `(?m)^\s*PARALLEL SAFE\s*$`, up)
	}
}
