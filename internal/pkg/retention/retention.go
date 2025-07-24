package retention

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/retention/deleter"
	"github.com/frain-dev/convoy/internal/pkg/retention/partman"
	"github.com/frain-dev/convoy/pkg/log"
	"time"
)

type Retentioner interface {
	Maintain(context.Context) error
	Start(context.Context, time.Duration)
}

func NewRetentioner(ctx context.Context, cfg config.RetentionPolicyConfiguration, database database.Database, licenser license.Licenser, flags *fflag.FFlag, lo log.StdLogger) (Retentioner, error) {
	policy, err := time.ParseDuration(cfg.Policy)
	if err != nil {
		lo.WithError(err).Fatal("Failed to parse retention policy")
		return nil, fmt.Errorf("failed to parse retention policy: %w", err)
	}

	if !flags.CanAccessFeature(fflag.RetentionPolicy) || !licenser.RetentionPolicy() {
		lo.WithError(fflag.ErrRetentionPolicyNotEnabled).Info("Defaulting to delete retention policy.")
		return deleter.NewDeleteRetentionPolicy(database, lo, policy), nil
	}

	ret, err := partman.NewPartitionRetentionPolicy(database, lo, policy)
	if err != nil {
		lo.WithError(err).Info("Failed to create retention policy")
		return nil, fmt.Errorf("failed to create retention policy: %w", err)
	}

	ret.Start(ctx, policy)

	return ret, nil
}
