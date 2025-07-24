package noop

import (
	"context"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	partman "github.com/jirevwe/go_partman"
	"os"
	"time"
)

type NoopRetentionPolicy struct {
	part   partman.Partitioner
	logger log.StdLogger
	db     database.Database
}

func (t *NoopRetentionPolicy) Maintain(ctx context.Context) error {
	return t.part.Maintain(ctx)
}

func (t *NoopRetentionPolicy) Start(_ context.Context, _ time.Duration) {}

func NewTestRetentionPolicy(db database.Database, manager *partman.Manager) *NoopRetentionPolicy {
	return &NoopRetentionPolicy{
		logger: log.NewLogger(os.Stdout),
		part:   manager,
		db:     db,
	}
}
