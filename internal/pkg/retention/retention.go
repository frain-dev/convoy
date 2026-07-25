package retention

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	partman "github.com/jirevwe/gopartman"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/projects"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// RetentionTables are the tables the partition retention policy manages.
// They must be converted to partitioned parents (`convoy partition`) before
// retention can run.
var RetentionTables = []string{"events", "events_search", "event_deliveries", "delivery_attempts"}

const (
	retentionSchema      = "convoy"
	retentionTenantCol   = "project_id"
	retentionPartitionBy = "created_at"
	retentionPremake     = 10
)

// UnpartitionedTables returns the retention-managed tables that are not yet
// declared as partitioned parents (relkind 'p') in Postgres. Retention is
// partition-drop based, so a non-empty result means retention cannot run.
func UnpartitionedTables(ctx context.Context, db database.Database) ([]string, error) {
	rows, err := db.GetDB().QueryContext(ctx, `
		SELECT c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'convoy'
		  AND c.relkind = 'p'
		  AND c.relname IN ('events', 'events_search', 'event_deliveries', 'delivery_attempts')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	partitioned := make(map[string]bool, len(RetentionTables))
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			return nil, scanErr
		}
		partitioned[name] = true
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	var missing []string
	for _, t := range RetentionTables {
		if !partitioned[t] {
			missing = append(missing, t)
		}
	}
	return missing, nil
}

type Retentioner interface {
	Perform(context.Context) error
	Start(context.Context, time.Duration)
}

// DisabledRetentionPolicy is installed when the license includes retention
// but the tables have not been converted to partitioned parents yet. It never
// deletes anything; each nightly run logs the actionable error instead, so
// the scheduled task neither fails nor goes unhandled. Run `convoy partition`
// and restart the workers to activate real retention.
type DisabledRetentionPolicy struct {
	missing []string
	logger  log.Logger
}

func NewDisabledRetentionPolicy(missing []string, logger log.Logger) *DisabledRetentionPolicy {
	return &DisabledRetentionPolicy{missing: missing, logger: logger}
}

func (d *DisabledRetentionPolicy) Perform(context.Context) error {
	d.logger.Error(fmt.Sprintf("retention is licensed but skipped: tables are not partitioned: %v. Run `convoy partition` and restart the workers to activate retention", d.missing))
	return nil
}

func (d *DisabledRetentionPolicy) Start(_ context.Context, _ time.Duration) {}

type TestRetentionPolicy struct {
	manager *partman.Manager
}

func (t *TestRetentionPolicy) Perform(ctx context.Context) error {
	return t.manager.Maintain(ctx)
}

func (t *TestRetentionPolicy) Start(_ context.Context, _ time.Duration) {}

func NewTestRetentionPolicy(manager *partman.Manager) *TestRetentionPolicy {
	return &TestRetentionPolicy{manager: manager}
}

type PartitionRetentionPolicy struct {
	retentionPeriod time.Duration
	manager         *partman.Manager
	logger          log.Logger
	db              database.Database
}

func NewPartitionRetentionPolicy(db database.Database, logger log.Logger, period time.Duration) (*PartitionRetentionPolicy, error) {
	if err := applyPartmanMigrations(context.Background(), db); err != nil {
		return nil, err
	}

	// Nil hook == HookDrop (JIT archiving makes archive-at-drop unnecessary).
	pm, err := partman.New(
		partman.WithDB(db.GetConn()),
		partman.WithClock(partman.NewRealClock()),
		partman.WithLogger(slog.Default()),
		// Asynq drives Maintain; do not also run the library ticker.
		partman.WithScheduleInterval(24*time.Hour),
	)
	if err != nil {
		return nil, err
	}

	return &PartitionRetentionPolicy{
		retentionPeriod: period,
		manager:         pm,
		logger:          logger,
		db:              db,
	}, nil
}

func applyPartmanMigrations(ctx context.Context, db database.Database) error {
	pool := db.GetConn()
	for _, m := range partman.Migrations() {
		if _, err := pool.Exec(ctx, m.SQL); err != nil {
			return fmt.Errorf("apply partman migration %s: %w", m.Name, err)
		}
	}
	return nil
}

func (r *PartitionRetentionPolicy) parentConfig(table string) partman.ParentConfig {
	// Keep AutomaticMaintenance enabled so Maintain() processes these
	// parents. We never call Manager.Start(), so the library ticker does
	// not run; the asynq nightly job drives Maintain.
	return partman.ParentConfig{
		SchemaName:        retentionSchema,
		TableName:         table,
		TenantColumn:      retentionTenantCol,
		PartitionBy:       retentionPartitionBy,
		PartitionInterval: partman.PartitionDayInterval,
		Premake:           retentionPremake,
		RetentionPeriod:   r.retentionPeriod,
	}
}

func (r *PartitionRetentionPolicy) registerParents(ctx context.Context) {
	for _, table := range RetentionTables {
		err := r.manager.RegisterParent(ctx, r.parentConfig(table))
		if err != nil && !errors.Is(err, partman.ErrParentAlreadyExists) {
			r.logger.Error(fmt.Sprintf("failed to register convoy.%s with gopartman", table), "error", err)
			continue
		}

		ref := partman.ParentRef{SchemaName: retentionSchema, TableName: table}
		if _, err := r.manager.ImportExisting(ctx, ref); err != nil {
			r.logger.Errorf("failed to import existing partitions for convoy.%s: %v", table, err)
		}
	}
}

func (r *PartitionRetentionPolicy) registerTenants(ctx context.Context) {
	projectRepo := projects.New(r.logger, r.db)
	projects, err := projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		r.logger.Error("failed to load projects for retention tenant registration", "error", err)
		return
	}

	for _, project := range projects {
		for _, table := range RetentionTables {
			err := r.manager.RegisterTenant(ctx, partman.TenantConfig{
				ParentSchema: retentionSchema,
				ParentName:   table,
				TenantId:     project.UID,
			})
			if err != nil && !errors.Is(err, partman.ErrTenantAlreadyExists) {
				r.logger.Error(fmt.Sprintf("failed to register tenant for convoy.%s", table), "error", err, "project_id", project.UID)
			}
		}
	}
}

// Start registers parents and tenants, then reconciles both on sampleRate.
// Parent registration is re-attempted on each tick so a transient boot-time
// RegisterParent/ImportExisting failure does not leave Maintain with no
// managed tables for the life of the process (ErrParentAlreadyExists is
// treated as success). It does not start gopartman's internal ticker;
// Perform (the asynq nightly job) calls Maintain.
func (r *PartitionRetentionPolicy) Start(ctx context.Context, sampleRate time.Duration) {
	go func(r *PartitionRetentionPolicy) {
		r.registerParents(ctx)
		r.registerTenants(ctx)

		if sampleRate <= 0 {
			sampleRate = time.Hour
		}
		ticker := time.NewTicker(sampleRate)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				bg := context.Background()
				r.registerParents(bg)
				r.registerTenants(bg)
			}
		}
	}(r)
}

func (r *PartitionRetentionPolicy) Perform(ctx context.Context) error {
	return r.manager.Maintain(ctx)
}
