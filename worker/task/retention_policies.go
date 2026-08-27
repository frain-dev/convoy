package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/retention"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func RetentionPolicies(locker JobLocker, configRepo datastore.ConfigurationRepository, ret retention.Retentioner, logger log.Logger) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		return locker.WithLock(ctx, "convoy:retention:mutex", 30*time.Minute, func(ctx context.Context) error {
			cfg, err := configRepo.LoadConfiguration(ctx)
			if err != nil {
				return fmt.Errorf("load configuration for retention: %w", err)
			}
			rc := cfg.GetRetentionPolicyConfig()
			if !rc.Enabled {
				logger.InfoContext(ctx, "retention disabled in configuration; skipping partition drop")
				return nil
			}

			periodStr := strings.TrimSpace(rc.Period)
			if periodStr == "" {
				periodStr = datastore.DefaultRetentionPolicy.Period
			}
			period, err := time.ParseDuration(periodStr)
			if err != nil {
				return fmt.Errorf("parse retention period %q: %w", periodStr, err)
			}
			if lr, ok := ret.(*retention.LicensedRetentionPolicy); ok {
				lr.SetPeriod(period)
			}

			c := time.Now()
			if err := ret.Perform(ctx); err != nil {
				return err
			}
			logger.InfoContext(ctx, fmt.Sprintf("Retention job took %f minutes to run", time.Since(c).Minutes()))
			return nil
		})
	}
}
