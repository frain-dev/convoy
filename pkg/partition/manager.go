package partition

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
	"time"
)

// Manager Core partition manager
type Manager struct {
	db     *sqlx.DB
	logger *log.Logger
	config Config
}

func (m *Manager) Initialize(ctx context.Context, config Config) error {
	// create the library's management table

	// upsert: add the new tables that are to be managed by the library

	return nil
}

func (m *Manager) CreateFuturePartitions(ctx context.Context, tableConfig TableConfig, ahead uint) error {
	// fetch active managed tables

	// for each parent table: create a future partition table

	// create the indexes and attach it to the parent table

	return nil
}

func (m *Manager) DropOldPartitions(ctx context.Context) error {
	// fetch active managed tables

	// for each parent table: fetch the tables to be dropped

	// run any hooks (gzip and export to object storage)

	// drop the table

	return nil
}

// createPartition creates a partition for a table
func (m *Manager) createPartition(ctx context.Context, tableConfig TableConfig, bounds Bounds) error {
	// Generate partition name based on bounds
	partitionName := m.generatePartitionName(tableConfig, bounds)

	// Create SQL for partition
	sql, err := m.generatePartitionSQL(partitionName, tableConfig, bounds)

	// Execute partition creation
	_, err = m.db.ExecContext(ctx, sql)
	if err != nil {
		return err
	}

	return nil
}

// Maintain defines a regularly run maintenance routine
func (m *Manager) Maintain(ctx context.Context) error {
	// loop all tables and run maintenance

	for i := 0; i < len(m.config.Tables); i++ {
		table := m.config.Tables[i]

		// Check for necessary future partitions
		if err := m.CreateFuturePartitions(ctx, table, 1); err != nil {
			return fmt.Errorf("failed to create future partitions: %w", err)
		}

		// Drop old partitions if needed
		if err := m.DropOldPartitions(ctx); err != nil {
			return fmt.Errorf("failed to drop old partitions: %w", err)
		}
	}

	return nil
}

// generatePartitionSQL generates the name of the partition table
func (m *Manager) generatePartitionSQL(name string, tableConfig TableConfig, bounds Bounds) (string, error) {
	switch tableConfig.PartitionType {
	case "range":
		return m.generateRangePartitionSQL(name, bounds), nil
	case "list", "hash":
		return "", fmt.Errorf("list and hash partitions are not implemented yet %q", tableConfig.PartitionType)
	default:
		return "", fmt.Errorf("unsupported partition type %q", tableConfig.PartitionType)
	}
}

func (m *Manager) generateRangePartitionSQL(name string, bounds Bounds) string {
	return fmt.Sprintf(`
        CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM (%s) TO (%s)
    `, name, m.config.Tables[0], bounds.From.Format(time.DateOnly), bounds.To.Format(time.DateOnly))
}

func (m *Manager) generatePartitionName(tableConfig TableConfig, bounds Bounds) string {
	return fmt.Sprintf("%s_%s", tableConfig.Name, bounds.From.Format(time.DateOnly))
}

func run() {
	config := Config{
		Tables: []TableConfig{
			{
				Name:              "samples",
				TenantId:          "project_id_asd",
				PartitionType:     TypeRange,
				PartitionBy:       []string{"project_id", "created_at"},
				PartitionInterval: OneDay,
				RetentionPeriod:   OneMonth,
			},
			{
				Name:              "samples",
				TenantId:          "project_id_124",
				PartitionType:     TypeRange,
				PartitionBy:       []string{"project_id", "created_at"},
				PartitionInterval: OneDay,
				RetentionPeriod:   OneMonth * 2,
			},
		},
		SchemaName: "public",
	}

	manager := NewManager(nil, config)

	// Initialize partition structure
	if err := manager.Initialize(context.Background(), config); err != nil {
		log.Fatal(err)
	}

	// Set up maintenance routine
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		for range ticker.C {
			if err := manager.Maintain(context.Background()); err != nil {
				log.Printf("maintenance error: %v", err)
			}
		}
	}()
}

func NewManager(db *sqlx.DB, config Config) *Manager {
	return &Manager{
		db:     db,
		config: config,
	}
}
