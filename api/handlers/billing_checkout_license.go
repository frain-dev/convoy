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
	if err := h.finalizePurchasedLicenseErr(r, cfgSvc, cfg, attemptID, attempt, purchased, patch); err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return false
	}
	return true
}

func (h *BillingHandler) finalizePurchasedLicenseErr(
	r *http.Request,
	cfgSvc *configuration.Service,
	cfg *datastore.Configuration,
	attemptID string,
	attempt datastore.SelfHostedCheckoutAttempt,
	purchased string,
	patch func(*datastore.Configuration, *datastore.SelfHostedCheckoutAttempt),
) error {
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
		return err
	}
	if !applied {
		if err := cfgSvc.CompleteSupersededCheckout(r.Context(), cfg.UID, attemptID, attempt, purchased, !envActive); err != nil {
			return err
		}
	}

	if err := h.refreshInstanceLicenser(effectiveKey); err != nil {
		// Failure policy: license columns are already persisted; in-process refresh is
		// best-effort so callers are not told to retry mint after a successful write.
		h.A.Logger.Warn("failed to refresh license entitlements after checkout; license persisted", "error", err)
	}
	return nil
}

type selfHostedTrialMint struct {
	attemptID  string
	licenseKey string
	externalID string
	status     string
}

func (h *BillingHandler) settleOrRecoverSelfHostedTrial(
	w http.ResponseWriter,
	r *http.Request,
	cfgSvc *configuration.Service,
	mint selfHostedTrialMint,
) bool {
	if err := h.settleSelfHostedTrialMint(r, cfgSvc, mint); err == nil {
		return true
	} else {
		h.A.Logger.Warn(
			"trial mint settlement failed; attempting orphan recovery",
			"error", err,
			"attempt_id", mint.attemptID,
		)
	}

	if err := h.recoverOrphanedSelfHostedTrialMint(r, cfgSvc, mint); err == nil {
		return true
	} else {
		h.A.Logger.Error(
			"trial mint orphan recovery failed",
			"error", err,
			"attempt_id", mint.attemptID,
		)
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return false
	}
}

func (h *BillingHandler) settleSelfHostedTrialMint(
	r *http.Request,
	cfgSvc *configuration.Service,
	mint selfHostedTrialMint,
) error {
	cfg, err := cfgSvc.LoadInstanceBillingConfig(r.Context())
	if err != nil {
		return err
	}

	now := time.Now()
	ensureCheckoutAttemptsMap(cfg)
	supersedeActiveCheckoutAttempt(cfg, now)
	cfg.CheckoutID = ""
	cfg.CheckoutAttempts[mint.attemptID] = datastore.SelfHostedCheckoutAttempt{
		AttemptID: mint.attemptID,
		Status:    checkoutStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	cfg.ActiveCheckoutAttemptID = mint.attemptID
	if err := cfgSvc.UpdateCheckoutAttempts(r.Context(), cfg); err != nil {
		return err
	}

	attempt := cfg.CheckoutAttempts[mint.attemptID]
	attempt.Status = checkoutStatusCompleted
	attempt.LastCompletionStatus = mint.status
	attempt.ExternalID = mint.externalID
	attempt.CompletedAt = null.NewTime(now, true)
	attempt.UpdatedAt = now

	return h.finalizePurchasedLicenseErr(r, cfgSvc, cfg, mint.attemptID, attempt, mint.licenseKey, func(c *datastore.Configuration, a *datastore.SelfHostedCheckoutAttempt) {
		c.ExternalID = mint.externalID
	})
}

func (h *BillingHandler) recoverOrphanedSelfHostedTrialMint(
	r *http.Request,
	cfgSvc *configuration.Service,
	mint selfHostedTrialMint,
) error {
	cfg, err := cfgSvc.LoadInstanceBillingConfig(r.Context())
	if err != nil {
		return err
	}

	now := time.Now()
	envActive := cfg.LicenseKeySource == config.LicenseSourceEnv
	ensureCheckoutAttemptsMap(cfg)
	supersedeActiveCheckoutAttempt(cfg, now)
	cfg.CheckoutID = ""
	cfg.CheckoutAttempts[mint.attemptID] = datastore.SelfHostedCheckoutAttempt{
		AttemptID:            mint.attemptID,
		Status:               checkoutStatusCompleted,
		LastCompletionStatus: mint.status,
		ExternalID:           mint.externalID,
		CompletedAt:          null.NewTime(now, true),
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	cfg.ActiveCheckoutAttemptID = ""
	cfg.CheckoutLicenseKey = mint.licenseKey
	cfg.ExternalID = mint.externalID
	if !envActive {
		cfg.LicenseKey = mint.licenseKey
		cfg.LicenseKeySource = config.LicenseSourceGuestCheckout
		cfg.LicenseSyncedAt = null.NewTime(now, true)
	}

	if err := cfgSvc.UpdateInstanceBillingConfig(r.Context(), cfg); err != nil {
		return err
	}

	effectiveKey := cfg.LicenseKey
	if err := h.refreshInstanceLicenser(effectiveKey); err != nil {
		h.A.Logger.Warn("failed to refresh license entitlements after orphan trial recovery; license persisted", "error", err)
	}
	return nil
}

func ensureCheckoutAttemptsMap(cfg *datastore.Configuration) {
	if cfg.CheckoutAttempts == nil {
		cfg.CheckoutAttempts = map[string]datastore.SelfHostedCheckoutAttempt{}
	}
}
