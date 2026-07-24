package retention

import (
	"context"
	"fmt"
	"time"

	partman "github.com/jirevwe/go_partman"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/projects"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// RetentionTables are the tables the partition retention policy manages.
// They must be converted to partitioned parents (`convoy partition`) before
// retention can run.
var RetentionTables = []string{"events", "events_search", "event_deliveries", "delivery_attempts"}

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
	partitioner partman.Partitioner
	logger      log.Logger
	db          database.Database
}

func (t *TestRetentionPolicy) Perform(ctx context.Context) error {
	return t.partitioner.Maintain(ctx)
}

func (t *TestRetentionPolicy) Start(_ context.Context, _ time.Duration) {}

func NewTestRetentionPolicy(db database.Database, manager *partman.Manager) *TestRetentionPolicy {
	return &TestRetentionPolicy{
		partitioner: manager,
		logger:      log.New("convoy", log.LevelInfo),
		db:          db,
	}
}

type PartitionRetentionPolicy struct {
	retentionPeriod time.Duration
	partitioner     partman.Partitioner
	logger          log.Logger
	db              database.Database
}

func NewPartitionRetentionPolicy(db database.Database, logger log.Logger, period time.Duration) (*PartitionRetentionPolicy, error) {
	pm, err := partman.NewManager(
		partman.WithDB(db.GetDB()),
		partman.WithLogger(logger),
		partman.WithConfig(&partman.Config{SampleRate: time.Second}),
		partman.WithClock(partman.NewRealClock()),
	)
	if err != nil {
		return nil, err
	}

	return &PartitionRetentionPolicy{
		retentionPeriod: period,
		partitioner:     pm,
		logger:          logger,
		db:              db,
	}, nil
}

func (r *PartitionRetentionPolicy) Start(ctx context.Context, sampleRate time.Duration) {
	go func(r *PartitionRetentionPolicy) {
		ticker := time.NewTicker(sampleRate)
		defer ticker.Stop()

		// fetch existing partitions on startup,
		// this is useful for one time setups,
		// but I'll leave it in since it'll no-op after the first time
		err := r.partitioner.ImportExistingPartitions(ctx, partman.Table{
			Schema:            "convoy",
			TenantIdColumn:    "project_id",
			PartitionBy:       "created_at",
			PartitionType:     partman.TypeRange,
			RetentionPeriod:   r.retentionPeriod,
			PartitionInterval: time.Hour * 24,
			PartitionCount:    10,
		})
		if err != nil {
			r.logger.Errorf("failed to import existing partitions: %v", err)
		}

		projectRepo := projects.New(r.logger, r.db)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				projects, pErr := projectRepo.LoadProjects(context.Background(), &datastore.ProjectFilter{})
				if pErr != nil {
					r.logger.Error("failed to load projects", "error", pErr)
				}

				for _, project := range projects {
					for _, table := range RetentionTables {
						err = r.partitioner.AddManagedTable(partman.Table{
							Name:              table,
							Schema:            "convoy",
							TenantId:          project.UID,
							TenantIdColumn:    "project_id",
							PartitionBy:       "created_at",
							PartitionType:     partman.TypeRange,
							RetentionPeriod:   r.retentionPeriod,
							PartitionInterval: time.Hour * 24,
							PartitionCount:    10,
						})
						if err != nil {
							r.logger.Error(fmt.Sprintf("failed to add convoy.%s to managed tables", table), "error", err)
						}
					}
				}
			}
		}
	}(r)
}

func (r *PartitionRetentionPolicy) Perform(ctx context.Context) error {
	return r.partitioner.Maintain(ctx)
}
