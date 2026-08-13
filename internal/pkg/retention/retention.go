package retention

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	partman "github.com/jirevwe/gopartman"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/projects"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// RetentionTables are the tables the partition retention policy manages.
// They must be converted to partitioned parents (`convoy utils partition`) before
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

// LicensedRetentionPolicy is installed when the license includes retention.
// It re-reads partition state on Start, on a reconcile ticker, and on every
// Perform so `convoy utils partition` can activate retention without a worker
// restart. Until all RetentionTables are partitioned parents it never
// deletes; each skip logs the actionable error so the asynq job stays healthy.
type LicensedRetentionPolicy struct {
	db     database.Database
	logger log.Logger
	period time.Duration

	mu       sync.Mutex
	interval time.Duration
	// lifeCtx is the worker lifetime context from Start. Inner reconcile
	// must use this, not an asynq task ctx from Perform, or the goroutine
	// exits when the nightly job finishes.
	lifeCtx context.Context
	inner   *PartitionRetentionPolicy
	missing []string
}

func NewLicensedRetentionPolicy(db database.Database, logger log.Logger, period time.Duration) *LicensedRetentionPolicy {
	return &LicensedRetentionPolicy{db: db, logger: logger, period: period}
}

func (l *LicensedRetentionPolicy) Start(ctx context.Context, sampleRate time.Duration) {
	l.mu.Lock()
	l.interval = sampleRate
	l.lifeCtx = ctx
	l.mu.Unlock()

	if err := l.ensureActive(ctx); err != nil {
		l.logger.Error("failed to activate partition retention", "error", err)
	}

	go func() {
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
				l.mu.Lock()
				active := l.inner != nil
				l.mu.Unlock()
				if active {
					// inner.Start owns parent/tenant reconciliation.
					return
				}
				if err := l.ensureActive(ctx); err != nil {
					l.logger.Error("failed to activate partition retention", "error", err)
				}
			}
		}
	}()
}

func (l *LicensedRetentionPolicy) Perform(ctx context.Context) error {
	if err := l.ensureActive(ctx); err != nil {
		// Fail closed: a partition-state lookup or manager create failure is
		// not a definitive "unpartitioned" verdict; do not Maintain.
		return err
	}

	l.mu.Lock()
	inner := l.inner
	missing := append([]string(nil), l.missing...)
	l.mu.Unlock()

	if inner == nil {
		l.logger.Error(fmt.Sprintf("retention is licensed but skipped: tables are not partitioned: %v. Run `convoy utils partition`", missing))
		return nil
	}
	return inner.Perform(ctx)
}

// ensureActive upgrades to PartitionRetentionPolicy once tables are
// partitioned. Safe to call concurrently; activation is one-shot.
func (l *LicensedRetentionPolicy) ensureActive(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inner != nil {
		return nil
	}

	missing, err := UnpartitionedTables(ctx, l.db)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		l.missing = missing
		return nil
	}

	inner, err := NewPartitionRetentionPolicy(l.db, l.logger, l.period)
	if err != nil {
		return err
	}
	interval := l.interval
	if interval <= 0 {
		interval = time.Minute
	}
	lifeCtx := l.lifeCtx
	if lifeCtx == nil {
		// Start has not run yet; do not bind reconcile to a short-lived
		// caller ctx (e.g. asynq task). Fail closed: refuse activation.
		return fmt.Errorf("retention Start has not run; cannot bind reconcile goroutine")
	}
	inner.Start(lifeCtx, interval)
	l.inner = inner
	l.missing = nil
	l.logger.Info("retention activated: tables are partitioned")
	return nil
}

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
	// Logger omitted: gopartman defaults to slog.Default(); importing log/slog
	// here is blocked by depguard (use pkg/logger for Convoy-owned logs).
	pm, err := partman.New(
		partman.WithDB(db.GetConn()),
		partman.WithClock(partman.NewRealClock()),
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
		report, err := r.manager.ImportExisting(ctx, ref)
		if err != nil {
			r.logger.Errorf("failed to import existing partitions for convoy.%s: %v", table, err)
			continue
		}
		r.logImportReport(table, report)
	}
}

// logImportReport surfaces the outcome of ImportExisting. gopartman reports
// drifted and skipped children without returning an error, and only adopted
// partitions reach partman.partitions, so an unread report means retention can
// silently manage nothing while the table keeps growing. Logged at error level
// because every unadopted partition is one the nightly job will never drop.
// Names are capped: a mismatch is usually systematic and affects every child.
func (r *PartitionRetentionPolicy) logImportReport(table string, report partman.ReconcileReport) {
	const maxNamed = 3

	if len(report.Drifted) == 0 && len(report.Skipped) == 0 {
		r.logger.Info(fmt.Sprintf("imported %d existing partitions for convoy.%s", len(report.Imported), table))
		return
	}

	r.logger.Error(fmt.Sprintf(
		"convoy.%s has partitions retention cannot drop: %d adopted, %d drifted, %d skipped",
		table, len(report.Imported), len(report.Drifted), len(report.Skipped)))

	for i, d := range report.Drifted {
		if i == maxNamed {
			r.logger.Errorf("... and %d more drifted partitions on convoy.%s", len(report.Drifted)-maxNamed, table)
			break
		}
		r.logger.Errorf("drifted partition %s: %s", d.Name, d.Reason)
	}
	for i, s := range report.Skipped {
		if i == maxNamed {
			r.logger.Errorf("... and %d more skipped partitions on convoy.%s", len(report.Skipped)-maxNamed, table)
			break
		}
		r.logger.Errorf("skipped partition %s: %s", s.Name, s.Reason)
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
	// Register before Maintain so the nightly job is not racing Start's
	// first reconcile tick (or stuck after a transient RegisterParent miss).
	r.registerParents(ctx)
	r.registerTenants(ctx)

	if err := r.manager.Maintain(ctx); err != nil {
		return err
	}

	r.dropExpiredAdoptedPartitions(ctx)
	return nil
}

// dropExpiredAdoptedPartitions reclaims history that Maintain structurally
// cannot.
//
// Converting event_deliveries adopts the pre-conversion table as the parent's
// DEFAULT partition rather than copying it into daily children. gopartman
// selects expired partitions with is_default = false, deliberately, because a
// default is normally a catch-all that must never be dropped on a schedule. The
// consequence is that every row written before the conversion becomes exempt
// from retention forever, which on a large instance is most of the table.
//
// Failures are logged rather than returned. Maintain has already done the work
// the nightly job exists for, and this runs again tomorrow.
func (r *PartitionRetentionPolicy) dropExpiredAdoptedPartitions(ctx context.Context) {
	for _, table := range RetentionTables {
		dropped, err := r.dropAdoptedPartition(ctx, table)
		if err != nil {
			r.logger.Error(fmt.Sprintf("failed to drop expired history partition for convoy.%s", table), "error", err)
			continue
		}
		if dropped {
			r.logger.Info(fmt.Sprintf("dropped expired history partition convoy.%s_default", table))
		}
	}
}

// dropAdoptedPartition drops <table>_default when every row in it is older than
// the retention period.
//
// Two gates, both necessary. The partition must carry the bounds constraint the
// attach conversion writes, which is what distinguishes an adopted table from
// the empty catch-all gopartman provisions under the same name; dropping that
// one would delete live rows that arrived while a day partition was missing. And
// the newest row in it must already be expired, read from the data rather than
// inferred from the constraint, so this cannot be wrong about what it is
// deleting. The read is cheap despite the table's size: created_at is indexed,
// so max() is a backwards index scan rather than a scan of the partition.
func (r *PartitionRetentionPolicy) dropAdoptedPartition(ctx context.Context, table string) (bool, error) {
	partition := table + "_default"

	var adopted bool
	err := r.db.GetConn().QueryRow(ctx, `
        SELECT EXISTS (
            SELECT 1
            FROM pg_constraint con
            JOIN pg_class c ON c.oid = con.conrelid
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE n.nspname = $1 AND c.relname = $2 AND con.conname = $3
        )`, retentionSchema, partition, partition+"_bounds").Scan(&adopted)
	if err != nil {
		return false, fmt.Errorf("checking for an adopted history partition: %w", err)
	}
	if !adopted {
		return false, nil
	}

	var newest *time.Time
	err = r.db.GetConn().QueryRow(ctx,
		fmt.Sprintf(`SELECT max(created_at) FROM %s.%s`, retentionSchema, partition)).Scan(&newest)
	if err != nil {
		return false, fmt.Errorf("reading the newest row in %s: %w", partition, err)
	}

	// An empty adopted partition is left alone. It still routes rows below the
	// conversion's cutoff, and reclaiming nothing is not worth a destructive
	// statement.
	if newest == nil || newest.After(time.Now().Add(-r.retentionPeriod)) {
		return false, nil
	}

	// Dropping the table leaves gopartman's row behind if the import adopted it,
	// and that row would then block a later default from being registered. Both
	// statements are in one transaction so the catalog and the metadata cannot
	// disagree.
	tx, err := r.db.GetConn().Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `SET LOCAL lock_timeout = '3s'`); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DROP TABLE %s.%s`, retentionSchema, partition)); err != nil {
		return false, fmt.Errorf("dropping %s: %w", partition, err)
	}
	// gopartman stores the child's name schema qualified, so matching on the bare
	// name deletes nothing and leaves the row this transaction exists to remove.
	if _, err = tx.Exec(ctx, `DELETE FROM partman.partitions WHERE name = $1`,
		retentionSchema+"."+partition); err != nil {
		return false, fmt.Errorf("clearing partition metadata for %s: %w", partition, err)
	}

	return true, tx.Commit(ctx)
}
