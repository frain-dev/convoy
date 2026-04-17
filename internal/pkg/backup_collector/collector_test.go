package backup_collector

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	blobstore "github.com/frain-dev/convoy/internal/pkg/blob-store"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var infra *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(
		context.Background(),
		testenv.WithMinIO(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}

	infra = res
	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup: %v\n", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()

	err := config.LoadConfig("")
	require.NoError(t, err)

	pool, err := infra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(_ context.Context, _ any, _ any) {})

	km, err := keys.NewLocalKeyManager("test")
	require.NoError(t, err)
	err = keys.Set(km)
	require.NoError(t, err)

	// Build DSN from pool config for the replication connection
	cfg := pool.Config().ConnConfig
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	// Create the publication in this cloned DB (template may not have it)
	_, err = pool.Exec(context.Background(), `
		DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = 'convoy_backup') THEN
				CREATE PUBLICATION convoy_backup FOR TABLE
					convoy.events, convoy.event_deliveries, convoy.delivery_attempts;
			END IF;
		END $$;
	`)
	require.NoError(t, err)

	return pool, dsn
}

// seedProject creates a project with all FK dependencies using testdb helpers.
func seedProject(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()

	db := postgres.NewFromConnection(pool)

	user, err := testdb.SeedDefaultUser(db)
	require.NoError(t, err)

	org, err := testdb.SeedDefaultOrganisation(db, user)
	require.NoError(t, err)

	project, err := testdb.SeedDefaultProject(db, org.UID)
	require.NoError(t, err)

	return project.UID
}

func TestCollector_StartStop(t *testing.T) {
	pool, dsn := setupTestDB(t)
	tmpDir := t.TempDir()

	logger := log.New("test", log.LevelInfo)
	store, err := blobstore.NewOnPremClient(blobstore.BlobStoreOptions{OnPremStorageDir: tmpDir}, logger)
	require.NoError(t, err)

	collector := NewBackupCollector(pool, dsn, store, 10*time.Second, logger)

	ctx := context.Background()
	err = collector.Start(ctx)
	require.NoError(t, err)

	// Verify slot exists
	var slotName string
	err = pool.QueryRow(ctx, "SELECT slot_name FROM pg_replication_slots WHERE slot_name = $1", defaultSlotName).Scan(&slotName)
	require.NoError(t, err)
	require.Equal(t, defaultSlotName, slotName)

	// Stop
	collector.Stop(ctx)

	// Slot should still exist (permanent)
	err = pool.QueryRow(ctx, "SELECT slot_name FROM pg_replication_slots WHERE slot_name = $1", defaultSlotName).Scan(&slotName)
	require.NoError(t, err)
	require.Equal(t, defaultSlotName, slotName)

	// Cleanup slot
	_, _ = pool.Exec(ctx, "SELECT pg_drop_replication_slot($1)", defaultSlotName)
}

func TestCollector_CaptureInserts(t *testing.T) {
	pool, dsn := setupTestDB(t)
	tmpDir := t.TempDir()

	logger := log.New("test", log.LevelInfo)
	store, err := blobstore.NewOnPremClient(blobstore.BlobStoreOptions{OnPremStorageDir: tmpDir}, logger)
	require.NoError(t, err)

	// Use short flush interval for tests
	collector := NewBackupCollector(pool, dsn, store, 3*time.Second, logger)

	ctx := context.Background()
	err = collector.Start(ctx)
	require.NoError(t, err)

	defer func() {
		collector.Stop(ctx)
		_, _ = pool.Exec(ctx, "SELECT pg_drop_replication_slot($1)", defaultSlotName)
	}()

	// Seed FK dependencies
	projectID := seedProject(t, pool)

	// Insert events
	for i := range 5 {
		_, err = pool.Exec(ctx, `
			INSERT INTO convoy.events (id, event_type, endpoints, project_id, headers, raw, data, status, created_at, updated_at)
			VALUES ($1, 'test.event', '{}', $2, '{}', '{}', '\x7b7d', 'Success', NOW(), NOW())
		`, fmt.Sprintf("evt_%d_%d", i, time.Now().UnixNano()), projectID)
		require.NoError(t, err)
	}

	// Wait for flush
	time.Sleep(5 * time.Second)

	// Verify files exist
	files := findGzipFiles(t, tmpDir)
	require.NotEmpty(t, files, "should have backup files after flush")

	// Find events file and count records
	var totalEvents int
	for _, f := range files {
		if containsPath(f, "events") {
			records := readJSONLFile(t, f)
			totalEvents += len(records)
		}
	}

	require.GreaterOrEqual(t, totalEvents, 5, "should have captured at least 5 events")
}

func TestCollector_IgnoreUpdatesDeletes(t *testing.T) {
	pool, dsn := setupTestDB(t)
	tmpDir := t.TempDir()

	logger := log.New("test", log.LevelInfo)
	store, err := blobstore.NewOnPremClient(blobstore.BlobStoreOptions{OnPremStorageDir: tmpDir}, logger)
	require.NoError(t, err)

	collector := NewBackupCollector(pool, dsn, store, 3*time.Second, logger)

	ctx := context.Background()
	err = collector.Start(ctx)
	require.NoError(t, err)

	defer func() {
		collector.Stop(ctx)
		_, _ = pool.Exec(ctx, "SELECT pg_drop_replication_slot($1)", defaultSlotName)
	}()

	projectID := seedProject(t, pool)

	// Insert 3 events
	eventIDs := make([]string, 3)
	for i := range 3 {
		eventIDs[i] = fmt.Sprintf("evt_ud_%d_%d", i, time.Now().UnixNano())
		_, err = pool.Exec(ctx, `
			INSERT INTO convoy.events (id, event_type, endpoints, project_id, headers, raw, data, status, created_at, updated_at)
			VALUES ($1, 'test.event', '{}', $2, '{}', '{}', '\x7b7d', 'Success', NOW(), NOW())
		`, eventIDs[i], projectID)
		require.NoError(t, err)
	}

	// Update one
	_, err = pool.Exec(ctx, `UPDATE convoy.events SET status = 'Failed' WHERE id = $1`, eventIDs[0])
	require.NoError(t, err)

	// Soft-delete one
	_, err = pool.Exec(ctx, `UPDATE convoy.events SET deleted_at = NOW() WHERE id = $1`, eventIDs[1])
	require.NoError(t, err)

	// Wait for flush
	time.Sleep(5 * time.Second)

	// Count all exported events — should be exactly 3 (only INSERTs)
	files := findGzipFiles(t, tmpDir)
	var totalEvents int
	for _, f := range files {
		if containsPath(f, "events") {
			records := readJSONLFile(t, f)
			totalEvents += len(records)
		}
	}

	require.Equal(t, 3, totalEvents, "should have exactly 3 events (inserts only, no updates/deletes)")
}

// --- Test helpers ---

func findGzipFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if e.IsDir() {
			sub := findGzipFiles(t, dir+"/"+e.Name())
			files = append(files, sub...)
		} else if len(e.Name()) > 3 && e.Name()[len(e.Name())-3:] == ".gz" {
			files = append(files, dir+"/"+e.Name())
		}
	}
	return files
}

func readJSONLFile(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	gr, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	defer gr.Close()

	raw, err := io.ReadAll(gr)
	require.NoError(t, err)

	var results []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var record map[string]any
		err = json.Unmarshal(line, &record)
		require.NoError(t, err)
		results = append(results, record)
	}
	return results
}

func containsPath(path, segment string) bool {
	return len(path) > 0 && bytes.Contains([]byte(path), []byte("/"+segment+"/"))
}
