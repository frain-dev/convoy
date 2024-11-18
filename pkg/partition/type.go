package partition

import (
	"context"
	"time"
)

type PartitionerType string

// tableNamingPatten
const tableNamingPatten = "%s_%s"

const (
	TypeRange PartitionerType = "range"
	TypeHash  PartitionerType = "hash"
	TypeList  PartitionerType = "list"
)

const (
	OneDay   = 24 * time.Hour
	OneMonth = 30 * OneDay
)

type Partitioner interface {
	// Initialize Create initial partition structure
	Initialize(ctx context.Context, config Config) error

	// CreateFuturePartitions Create new partitions ahead of time
	CreateFuturePartitions(ctx context.Context, ahead uint) error

	// DropOldPartitions Drop old partitions based on retention policy
	DropOldPartitions(ctx context.Context) error

	// Maintain  Manage partition maintenance
	Maintain(ctx context.Context) error
}

type Bounds struct {
	From, To time.Time
}

type TableConfig struct {
	// Name is the table
	Name string

	// TenantId
	TenantId string

	// Partition type and settings
	PartitionType PartitionerType // "range", "list", or "hash"

	// PartitionBy Columns to partition by, they are applied in order
	PartitionBy []string

	// PartitionInterval For range partitions (e.g., "1 month", "1 day")
	PartitionInterval time.Duration

	// PreCreateCount is the number of partitions to create ahead when the partition is first created
	PreCreateCount uint

	// Retention settings
	RetentionPeriod time.Duration
}

type Config struct {
	// SchemaName is the schema of the tables
	SchemaName string

	Tables []TableConfig
}
