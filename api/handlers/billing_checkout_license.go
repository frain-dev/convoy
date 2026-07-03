package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/util"
)

func recordFailedTrialAttempt(cfg *datastore.Configuration, attemptID string, now time.Time) {
	ensureCheckoutAttemptsMap(cfg)
	cfg.CheckoutAttempts[attemptID] = datastore.SelfHostedCheckoutAttempt{
		AttemptID: attemptID,
		Status:    checkoutStatusFailed,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (h *BillingHandler) persistFailedTrialAttempt(
	w http.ResponseWriter,
	r *http.Request,
	cfgSvc *configuration.Service,
	attemptID string,
	now time.Time,
) bool {
	cfg, err := cfgSvc.LoadInstanceBillingConfig(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return false
	}
	recordFailedTrialAttempt(cfg, attemptID, now)
	if err := cfgSvc.UpdateCheckoutAttempts(r.Context(), cfg); err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return false
	}
	return true
}

func supersedeActiveCheckoutAttempt(cfg *datastore.Configuration, now time.Time) {
	if cfg.ActiveCheckoutAttemptID == "" {
		return
	}
	activeAttempt, ok := cfg.CheckoutAttempts[cfg.ActiveCheckoutAttemptID]
	if !ok {
		return
	}
	activeAttempt.Status = checkoutStatusSuperseded
	activeAttempt.CheckoutNonce = ""
	activeAttempt.CheckoutNonceHash = ""
	activeAttempt.UpdatedAt = now
	cfg.CheckoutAttempts[cfg.ActiveCheckoutAttemptID] = activeAttempt
}

func (h *BillingHandler) finalizePurchasedLicense(
	w http.ResponseWriter,
	r *http.Request,
	cfgSvc *configuration.Service,
	cfg *datastore.Configuration,
	attemptID string,
	attempt datastore.SelfHostedCheckoutAttempt,
	purchased string,
	patch func(*datastore.Configuration, *datastore.SelfHostedCheckoutAttempt),
) bool {
	now := time.Now()
	envActive := cfg.LicenseKeySource == config.LicenseSourceEnv

	patch(cfg, &attempt)
	cfg.CheckoutAttempts[attemptID] = attempt
	cfg.ActiveCheckoutAttemptID = ""
	cfg.CheckoutLicenseKey = purchased
	if !envActive {
		cfg.LicenseKey = purchased
		cfg.LicenseKeySource = config.LicenseSourceGuestCheckout
		cfg.LicenseSyncedAt = null.NewTime(now, true)
	}
	effectiveKey := cfg.LicenseKey

	applied, err := cfgSvc.CompleteCheckoutIfActive(r.Context(), cfg, attemptID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return false
	}
	if !applied {
		if err := cfgSvc.CompleteSupersededCheckout(r.Context(), cfg.UID, attemptID, attempt, purchased, !envActive); err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return false
		}
	}

	if err := h.refreshInstanceLicenser(effectiveKey); err != nil {
		// Failure policy: license columns are already persisted; in-process refresh is
		// best-effort so callers are not told to retry mint after a successful write.
		h.A.Logger.Warn("failed to refresh license entitlements after checkout; license persisted", "error", err)
	}
	return true
}

func ensureCheckoutAttemptsMap(cfg *datastore.Configuration) {
	if cfg.CheckoutAttempts == nil {
		cfg.CheckoutAttempts = map[string]datastore.SelfHostedCheckoutAttempt{}
	}
}
