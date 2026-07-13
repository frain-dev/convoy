package task

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/license/usage"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// SnapshotUsage materializes anonymized instance counts into Redis for the
// license-validate ping. Licensed instances only: no effective license → no-op.
func SnapshotUsage(lo log.Logger, db database.Database, rd *rdb.Redis) func(context.Context, *asynq.Task) error {
	pool := goredis.NewPool(rd.Client())
	rs := redsync.New(pool)
	store := usage.NewStore(db, rd)
	configRepo := configuration.New(lo, db)

	return func(ctx context.Context, t *asynq.Task) error {
		cfg, err := config.Get()
		if err != nil {
			return err
		}
		// Resolve env + persisted guest-checkout key (worker may not have seen
		// an in-process checkout that only updated the API singleton).
		if !hasEffectiveLicense(ctx, cfg.LicenseKey, configRepo) {
			return nil
		}

		const mutexName = "convoy:usage:mutex"
		// 30m matches other nightly schedule locks (retention, org status). COUNT(*)
		// on large events tables can exceed 1m; expired lock allows overlapping refreshes.
		mutex := rs.NewMutex(mutexName, redsync.WithExpiry(30*time.Minute), redsync.WithTries(1))
		tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := mutex.LockContext(tctx); err != nil {
			return fmt.Errorf("failed to obtain lock: %v", err)
		}
		defer func() {
			uctx, ucancel := context.WithTimeout(ctx, 2*time.Second)
			defer ucancel()
			if ok, err := mutex.UnlockContext(uctx); !ok || err != nil {
				lo.Error("failed to release usage snapshot lock", "error", err)
			}
		}()

		rctx, rcancel := context.WithTimeout(ctx, 25*time.Minute)
		defer rcancel()
		snap, err := store.Refresh(rctx)
		if err != nil {
			return err
		}
		lo.Info("refreshed usage snapshot",
			"endpoint_count", snap.EndpointCount,
			"event_count", snap.EventCount,
			"project_count", snap.ProjectCount,
			"org_count", snap.OrgCount,
			"user_count", snap.UserCount,
		)
		return nil
	}
}

// hasEffectiveLicense mirrors boot precedence: env/file key wins, else checkout.
// DB load failure fails open → treat as unlicensed (skip snapshot).
func hasEffectiveLicense(ctx context.Context, envKey string, configRepo *configuration.Service) bool {
	if !util.IsStringEmpty(envKey) {
		return true
	}
	instCfg, err := configRepo.LoadInstanceBillingConfig(ctx)
	if err != nil || instCfg == nil {
		return false
	}
	checkoutKey := instCfg.CheckoutLicenseKey
	if checkoutKey == "" && instCfg.LicenseKey != "" && instCfg.LicenseKeySource != config.LicenseSourceEnv {
		checkoutKey = instCfg.LicenseKey
	}
	effective, _ := config.ResolveEffectiveLicense(envKey, checkoutKey)
	return !util.IsStringEmpty(effective)
}
