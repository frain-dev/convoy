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

// LicensedRetentionPolicy is installed when the license includes retention.
// It re-reads partition state on Start, on a reconcile ticker, and on every
// Perform so `convoy partition` can activate retention without a worker
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
		l.logger.Error(fmt.Sprintf("retention is licensed but skipped: tables are not partitioned: %v. Run `convoy partition`", missing))
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
	// Register before Maintain so the nightly job is not racing Start's
	// first reconcile tick (or stuck after a transient RegisterParent miss).
	r.registerParents(ctx)
	r.registerTenants(ctx)
	return r.manager.Maintain(ctx)
}
