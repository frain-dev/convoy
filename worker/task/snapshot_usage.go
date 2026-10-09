package task

import (
	"context"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/license/usage"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// SnapshotUsage materializes anonymized instance counts into the active cache for the
// license-validate ping. Licensed instances only: no effective license → no-op.
func SnapshotUsage(lo log.Logger, db database.Database, brokerCache cache.Cache, locker JobLocker) func(context.Context, *asynq.Task) error {
	store := usage.NewStore(db, brokerCache)
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

		return locker.WithLock(ctx, "convoy:usage:mutex", 30*time.Minute, func(ctx context.Context) error {
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
		})
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
