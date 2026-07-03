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

func failActiveCheckoutAttempt(cfg *datastore.Configuration, attemptID string, now time.Time) {
	attempt, ok := cfg.CheckoutAttempts[attemptID]
	if !ok {
		return
	}
	attempt.Status = checkoutStatusFailed
	attempt.UpdatedAt = now
	cfg.CheckoutAttempts[attemptID] = attempt
	if cfg.ActiveCheckoutAttemptID == attemptID {
		cfg.ActiveCheckoutAttemptID = ""
	}
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
		_ = render.Render(w, r, util.NewErrorResponse("failed to refresh license entitlements", http.StatusInternalServerError))
		return false
	}
	return true
}

func ensureCheckoutAttemptsMap(cfg *datastore.Configuration) {
	if cfg.CheckoutAttempts == nil {
		cfg.CheckoutAttempts = map[string]datastore.SelfHostedCheckoutAttempt{}
	}
}
