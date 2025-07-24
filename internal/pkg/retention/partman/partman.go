package partman

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	partman "github.com/jirevwe/go_partman"
	"sync"
	"time"
)

var tenantTables = []string{"events", "event_deliveries", "events_search", "delivery_attempts"}

type PartitionRetentionPolicy struct {
	retPeriod time.Duration
	logger    log.StdLogger
	part      partman.Partitioner
	db        database.Database
}

func getParentTable(tableName string, retPeriod time.Duration) partman.Table {
	return partman.Table{
		Id:                "",
		Name:              tableName,
		Schema:            "convoy",
		TenantIdColumn:    "project_id",
		PartitionBy:       "created_at",
		PartitionType:     partman.TypeRange,
		PartitionInterval: time.Hour * 24,
		PartitionCount:    10,
		RetentionPeriod:   retPeriod,
	}
}

func NewPartitionRetentionPolicy(db database.Database, logger log.StdLogger, retPeriod time.Duration) (*PartitionRetentionPolicy, error) {
	tables := make([]partman.Table, 4)
	for i := range tenantTables {
		tables[i] = getParentTable(tenantTables[i], retPeriod)
	}

	pm, err := partman.NewManager(
		partman.WithDB(db.GetDB()),
		partman.WithLogger(logger),
		partman.WithConfig(&partman.Config{
			SampleRate: time.Minute,
			Tables:     tables,
		}),
		partman.WithClock(partman.NewRealClock()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create partition manager: %w", err)
	}

	return &PartitionRetentionPolicy{
		retPeriod: retPeriod,
		part:      pm,
		logger:    logger,
		db:        db,
	}, nil
}

func (r *PartitionRetentionPolicy) Start(ctx context.Context, sampleRate time.Duration) {
	err := r.part.Start(ctx)
	if err != nil {
		r.logger.WithError(err).Error("failed to start partition manager")
		return
	}

	go func(r *PartitionRetentionPolicy) {
		ticker := time.NewTicker(sampleRate)
		defer ticker.Stop()

		projectRepo := postgres.NewProjectRepo(r.db)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				projects, pErr := projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
				if pErr != nil {
					r.logger.WithError(pErr).Error("failed to load projects")
				} else {
					r.processProjects(ctx, projects)
				}
			}
		}
	}(r)
}

func (r *PartitionRetentionPolicy) Maintain(ctx context.Context) error {
	return r.part.Maintain(ctx)
}

func (r *PartitionRetentionPolicy) registerTenant(ctx context.Context, projectUID, tableName string) {
	result, registerErr := r.part.RegisterTenant(ctx, partman.Tenant{
		TableName:   tableName,
		TableSchema: "convoy",
		TenantId:    projectUID,
	})

	if registerErr != nil {
		r.logger.WithError(registerErr).Errorf("failed to register tenant for %q in table %q", projectUID, tableName)
	}

	if len(result.Errors) > 0 {
		r.logger.Errorf("errors occurred while registering tenant %q for table %q: %v", projectUID, tableName, result.Errors)
	}
}

func (r *PartitionRetentionPolicy) processProjects(ctx context.Context, projects []*datastore.Project) {
	var wg sync.WaitGroup

	for _, project := range projects {
		wg.Add(1) // Increment the counter
		go func(projectID string) {
			defer wg.Done()
			for _, tableName := range tenantTables {
				r.registerTenant(ctx, projectID, tableName)
			}
		}(project.UID)
	}

	wg.Wait() // Wait for all operations to complete
}
