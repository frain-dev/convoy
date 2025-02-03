package retention

import (
	"context"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	partman "github.com/jirevwe/go_partman"
	"os"
	"time"
)

type Retentioner interface {
	Perform(context.Context) error
	Start(context.Context, time.Duration)
}

type TestRetentionPolicy struct {
	partitioner partman.Partitioner
	logger      log.StdLogger
	db          database.Database
}

func (t TestRetentionPolicy) Perform(ctx context.Context) error {
	return t.partitioner.Maintain(ctx)
}

func (t TestRetentionPolicy) Start(_ context.Context, _ time.Duration) {}

func NewTestRetentionPolicy(db database.Database, manager *partman.Manager) *TestRetentionPolicy {
	return &TestRetentionPolicy{
		partitioner: manager,
		logger:      log.NewLogger(os.Stdout),
		db:          db,
	}
}

type PartitionRetentionPolicy struct {
	retentionPeriod time.Duration
	partitioner     partman.Partitioner
	logger          log.StdLogger
	db              database.Database
}

func NewPartitionRetentionPolicy(db database.Database, logger log.StdLogger, period time.Duration) (*PartitionRetentionPolicy, error) {
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

		projectRepo := postgres.NewProjectRepo(r.db)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				projects, pErr := projectRepo.LoadProjects(context.Background(), &datastore.ProjectFilter{})
				if pErr != nil {
					r.logger.WithError(pErr).Error("failed to load projects")
				}

				for _, project := range projects {
					err = r.partitioner.AddManagedTable(partman.Table{
						Name:              "events",
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
						r.logger.WithError(err).Error("failed to add convoy.events to managed tables")
					}

					err = r.partitioner.AddManagedTable(partman.Table{
						Name:              "events_search",
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
						r.logger.WithError(err).Error("failed to add convoy.events to managed tables")
					}

					err = r.partitioner.AddManagedTable(partman.Table{
						Name:              "event_deliveries",
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
						r.logger.WithError(err).Error("failed to add convoy.event_deliveries to managed tables")
					}

					err = r.partitioner.AddManagedTable(partman.Table{
						Name:              "delivery_attempts",
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
						r.logger.WithError(err).Error("failed to add convoy.delivery_attempts to managed tables")
					}
				}
			}
		}
	}(r)
}

func (r *PartitionRetentionPolicy) Perform(ctx context.Context) error {
	return r.partitioner.Maintain(ctx)
}
