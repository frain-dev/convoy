package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
)

type startSelfHostedCheckoutRequest struct {
	Email    string `json:"email"`
	PlanID   string `json:"plan_id"`
	Interval string `json:"interval"`
	Host     string `json:"host"`
}

type completeSelfHostedCheckoutRequest struct {
	Token      string `json:"token"`
	AttemptID  string `json:"attempt_id"`
	CheckoutID string `json:"checkout_id"`
}

// Self-hosted checkout attempt statuses, persisted on datastore.SelfHostedCheckoutAttempt.
const (
	checkoutStatusPending    = "pending"
	checkoutStatusSuperseded = "superseded"
	checkoutStatusFailed     = "failed"
	checkoutStatusCompleted  = "completed"
)

func (h *BillingHandler) GetBillingConfig(w http.ResponseWriter, r *http.Request) {
	instanceBilling, err := configuration.New(h.A.Logger, h.A.DB).LoadInstanceBillingConfig(r.Context())
	// Fail closed on a real read error like the checkout handlers, which load the
	// same config: a DB failure must not be reported as an unlicensed instance.
	// ErrConfigNotFound is the legitimate fresh-instance state, so let it fall
	// through to the unlicensed response below.
	if err != nil && !errors.Is(err, datastore.ErrConfigNotFound) {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
	// Floor with the in-memory env/file license so a not-yet-persisted or absent
	// config row does not misreport an env-licensed instance as OSS. A non-empty
	// persisted key (env resolved at boot, or a guest purchase) takes precedence.
	instanceLicenseKey := strings.TrimSpace(h.A.Cfg.LicenseKey)
	if instanceBilling != nil {
		if k := strings.TrimSpace(instanceBilling.LicenseKey); k != "" {
			instanceLicenseKey = k
		}
	}
	mode := h.A.Cfg.BillingMode(instanceLicenseKey)
	selfHosted := map[string]interface{}{
		"enabled":            mode != config.BillingModeCloud,
		"license_configured": instanceLicenseKey != "",
	}
	if instanceBilling != nil {
		selfHosted["license_synced_at"] = instanceBilling.LicenseSyncedAt
		selfHosted["license_source"] = instanceBilling.LicenseKeySource
		// Same resolver the start handler uses, so the UI's Resubscribe label cannot
		// disagree with whether a license_key is actually sent to Overwatch.
		selfHosted["resubscribe"] = config.ResolveCheckoutLicenseKey(instanceBilling.CheckoutLicenseKey, instanceBilling.LicenseKey, instanceBilling.LicenseKeySource) != ""
		if h.isInstanceAdmin(r) {
			activeAttempt, hasActiveAttempt := instanceBilling.CheckoutAttempts[instanceBilling.ActiveCheckoutAttemptID]
			selfHosted["active_checkout_attempt_id"] = instanceBilling.ActiveCheckoutAttemptID
			selfHosted["checkout_id"] = instanceBilling.CheckoutID
			selfHosted["external_id"] = instanceBilling.ExternalID
			if hasActiveAttempt && strings.TrimSpace(activeAttempt.CheckoutNonce) != "" {
				selfHosted["active_checkout"] = map[string]interface{}{
					"attempt_id":   activeAttempt.AttemptID,
					"checkout_id":  activeAttempt.CheckoutID,
					"checkout_url": activeAttempt.CheckoutURL,
					"plan_id":      activeAttempt.PlanID,
					"interval":     activeAttempt.Interval,
					"status":       activeAttempt.Status,
					"created_at":   activeAttempt.CreatedAt,
					"updated_at":   activeAttempt.UpdatedAt,
				}
			}
		}
	}

	response := map[string]interface{}{
		"strategy": string(mode),
		"cloud":    mode == config.BillingModeCloud,
		"payment_provider": map[string]interface{}{
			"type":            h.A.Cfg.Billing.PaymentProvider.Type,
			"publishable_key": h.A.Cfg.Billing.PaymentProvider.PublishableKey,
		},
		"self_hosted": selfHosted,
	}

	_ = render.Render(w, r, util.NewServerResponse("Billing configuration retrieved", response, http.StatusOK))
}

func (h *BillingHandler) StartSelfHostedCheckout(w http.ResponseWriter, r *http.Request) {
	if h.A.Cfg.UsesOrgBilling() {
		_ = render.Render(w, r, util.NewErrorResponse("self-hosted checkout is not available in cloud org billing mode", http.StatusForbidden))
		return
	}
	if !h.requireSelfHostedBillingAdmin(w, r) {
		return
	}
	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return
	}

	var req startSelfHostedCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	email := strings.TrimSpace(req.Email)
	if req.PlanID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("plan_id is required", http.StatusBadRequest))
		return
	}
	cfgSvc := configuration.New(h.A.Logger, h.A.DB)
	cfg, err := cfgSvc.LoadInstanceBillingConfig(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	// A non-empty resubscribe key (empty = first purchase) reuses the existing org;
	// Overwatch returns 409 if it still has a live subscription.
	resubscribeKey := config.ResolveCheckoutLicenseKey(cfg.CheckoutLicenseKey, cfg.LicenseKey, cfg.LicenseKeySource)

	// Email identifies a first-time buyer's new org. On resubscribe the org is already
	// known by the license key, so email is optional and Overwatch reuses the stored one.
	if email == "" && resubscribeKey == "" {
		_ = render.Render(w, r, util.NewErrorResponse("email is required", http.StatusBadRequest))
		return
	}

	host, err := billing.CanonicalOrigin(req.Host)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	attemptID := ulid.Make().String()
	nonce, err := newCheckoutNonce()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to create checkout nonce", http.StatusInternalServerError))
		return
	}
	nonceHash := hashCheckoutNonce(nonce)

	// Call the billing service before mutating local state. If StartGuestCheckout
	// fails, we have not superseded the prior active attempt nor persisted a new
	// one, so a transient failure cannot orphan an in-flight checkout.
	resp, err := h.BillingClient.StartGuestCheckout(r.Context(), billing.StartGuestCheckoutRequest{
		Email:             email,
		PlanID:            req.PlanID,
		Interval:          req.Interval,
		Host:              host,
		OrganisationName:  h.activeOrganisationName(r.Context(), r),
		AttemptID:         attemptID,
		CheckoutNonceHash: nonceHash,
		LicenseKey:        resubscribeKey,
	})
	if err != nil {
		status := http.StatusServiceUnavailable
		var billErr *billing.Error
		if errors.As(err, &billErr) && billErr.StatusCode == http.StatusConflict {
			status = http.StatusConflict
		}
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), status))
		return
	}

	now := time.Now()
	if cfg.CheckoutAttempts == nil {
		cfg.CheckoutAttempts = map[string]datastore.SelfHostedCheckoutAttempt{}
	}
	if cfg.ActiveCheckoutAttemptID != "" {
		if activeAttempt, ok := cfg.CheckoutAttempts[cfg.ActiveCheckoutAttemptID]; ok {
			activeAttempt.Status = checkoutStatusSuperseded
			activeAttempt.CheckoutNonce = ""
			activeAttempt.CheckoutNonceHash = ""
			activeAttempt.UpdatedAt = now
			cfg.CheckoutAttempts[cfg.ActiveCheckoutAttemptID] = activeAttempt
		}
	}
	cfg.CheckoutAttempts[attemptID] = datastore.SelfHostedCheckoutAttempt{
		AttemptID:         attemptID,
		CheckoutNonce:     nonce,
		CheckoutNonceHash: nonceHash,
		Email:             email,
		PlanID:            req.PlanID,
		Interval:          req.Interval,
		Status:            checkoutStatusPending,
		CheckoutID:        resp.Data.CheckoutID,
		CheckoutURL:       resp.Data.CheckoutURL,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	cfg.ActiveCheckoutAttemptID = attemptID
	cfg.CheckoutID = resp.Data.CheckoutID

	// Persist only attempt bookkeeping. Starting a checkout must never write the
	// license columns: this cfg was loaded at request start, so a full-row write
	// could erase a license a concurrent completion just persisted.
	//
	// Failure policy (fail closed, external-first): the Overwatch session is
	// created before this write, so a persist failure leaves it orphaned. We do
	// not compensate because (a) there is no cancel-checkout API, (b) the session
	// is unpaid and self-expires via Overwatch's checkout-expiry job, and (c) the
	// prior active attempt is only superseded in-memory above, so on failure it
	// stays the active attempt locally and in Overwatch. The caller gets an error
	// and simply retries; no license or payment state diverges.
	if err := cfgSvc.UpdateCheckoutAttempts(r.Context(), cfg); err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout started", map[string]interface{}{
		"checkout_url": resp.Data.CheckoutURL,
		"checkout_id":  resp.Data.CheckoutID,
		"attempt_id":   attemptID,
	}, http.StatusOK))
}

func (h *BillingHandler) CompleteSelfHostedCheckout(w http.ResponseWriter, r *http.Request) {
	if h.A.Cfg.UsesOrgBilling() {
		_ = render.Render(w, r, util.NewErrorResponse("self-hosted checkout is not available in cloud org billing mode", http.StatusForbidden))
		return
	}
	if !h.requireSelfHostedBillingAdmin(w, r) {
		return
	}
	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return
	}

	var req completeSelfHostedCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	cfgSvc := configuration.New(h.A.Logger, h.A.DB)
	cfg, err := cfgSvc.LoadInstanceBillingConfig(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	attemptID := strings.TrimSpace(req.AttemptID)
	if attemptID == "" {
		attemptID = cfg.ActiveCheckoutAttemptID
	}
	attempt, ok := cfg.CheckoutAttempts[attemptID]
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("active checkout attempt not found", http.StatusNotFound))
		return
	}

	// Recovery (idempotent): the attempt already completed and its license is
	// persisted, but a prior in-memory licenser refresh may have failed. The
	// active attempt is consumed on completion, so a plain retry would 404 and
	// leave entitlements stale until restart. Rebuild the licenser from the
	// stored key and return success. Fail closed if the rebuild errors.
	if attempt.Status == checkoutStatusCompleted && cfg.LicenseKey != "" {
		if err := h.refreshInstanceLicenser(cfg.LicenseKey); err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to refresh license entitlements", http.StatusInternalServerError))
			return
		}
		// Prefer this attempt's own checkout/external id: when it was settled on
		// the superseded path the instance-level columns belong to a newer active
		// attempt, so fall back to them only when the attempt has none.
		checkoutID := attempt.CheckoutID
		if checkoutID == "" {
			checkoutID = cfg.CheckoutID
		}
		externalID := attempt.ExternalID
		if externalID == "" {
			externalID = cfg.ExternalID
		}
		_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout completion checked", map[string]interface{}{
			"status":      checkoutStatusCompleted,
			"license_key": cfg.LicenseKey,
			"checkout_id": checkoutID,
			"external_id": externalID,
		}, http.StatusOK))
		return
	}

	if attempt.CheckoutNonce == "" {
		_ = render.Render(w, r, util.NewErrorResponse("active checkout attempt not found", http.StatusNotFound))
		return
	}
	if attemptID != cfg.ActiveCheckoutAttemptID {
		attempt.Status = checkoutStatusSuperseded
		attempt.CheckoutNonce = ""
		attempt.CheckoutNonceHash = ""
		attempt.UpdatedAt = time.Now()
		cfg.CheckoutAttempts[attemptID] = attempt
		// Marking an attempt superseded touches only attempt bookkeeping; never
		// write the license columns from this stale snapshot.
		if err := cfgSvc.UpdateCheckoutAttempts(r.Context(), cfg); err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}
		_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout attempt is no longer active", map[string]interface{}{
			"status": "stale",
		}, http.StatusConflict))
		return
	}

	checkoutID := strings.TrimSpace(req.CheckoutID)
	if checkoutID == "" {
		checkoutID = attempt.CheckoutID
	}

	resp, err := h.BillingClient.CompleteGuestCheckout(r.Context(), billing.CompleteGuestCheckoutRequest{
		Token:         req.Token,
		AttemptID:     attempt.AttemptID,
		CheckoutID:    checkoutID,
		CheckoutNonce: attempt.CheckoutNonce,
	})
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	now := time.Now()
	attempt.UpdatedAt = now
	attempt.LastCompletionStatus = resp.Data.Status
	attempt.CheckoutID = resp.Data.CheckoutID
	attempt.ExternalID = resp.Data.ExternalID

	if resp.Data.Status == checkoutStatusCompleted && resp.Data.LicenseKey != "" {
		purchased := resp.Data.LicenseKey
		// envActive: an env/file CONVOY_LICENSE_KEY is the active effective source
		// (resolved and persisted at boot). When it is, it wins and the purchased
		// key is only stashed for reversibility; otherwise the purchase becomes the
		// effective license.
		envActive := cfg.LicenseKeySource == config.LicenseSourceEnv

		attempt.Status = checkoutStatusCompleted
		attempt.CompletedAt = null.NewTime(now, true)
		attempt.CheckoutNonce = ""
		cfg.CheckoutAttempts[attemptID] = attempt
		cfg.CheckoutID = resp.Data.CheckoutID
		cfg.ExternalID = resp.Data.ExternalID
		cfg.ActiveCheckoutAttemptID = ""

		// The purchased key always lands in checkout_license_key (owned by checkout,
		// preserved so an env override stays reversible).
		cfg.CheckoutLicenseKey = purchased
		if !envActive {
			cfg.LicenseKey = purchased
			cfg.LicenseKeySource = config.LicenseSourceGuestCheckout
			cfg.LicenseSyncedAt = null.NewTime(now, true)
		}
		effectiveKey := cfg.LicenseKey

		// Apply the license only while this attempt is still the active one, so a
		// concurrent start/complete that superseded it cannot be clobbered.
		applied, err := cfgSvc.CompleteCheckoutIfActive(r.Context(), cfg, attemptID)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}
		if !applied {
			// The active attempt changed under us: a concurrent start superseded
			// it, or a concurrent complete already settled it. Overwatch still
			// minted the org license (one per org, idempotent). Mark only this
			// attempt's entry completed (carrying its checkout_id/external_id) and
			// record the paid key, without touching the newer active attempt's
			// bookkeeping or the instance-level checkout_id/external_id it now owns.
			// Persisting this attempt as completed lets a retry recover via the
			// idempotent branch above if the refresh below fails. Fail closed on
			// persist/refresh errors so a paid customer is never left unlicensed.
			if err := cfgSvc.CompleteSupersededCheckout(r.Context(), cfg.UID, attemptID, attempt, purchased, !envActive); err != nil {
				_ = render.Render(w, r, util.NewServiceErrResponse(err))
				return
			}
			if err := h.refreshInstanceLicenser(effectiveKey); err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("failed to refresh license entitlements", http.StatusInternalServerError))
				return
			}
			_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout completed", map[string]interface{}{
				"status":      checkoutStatusCompleted,
				"license_key": purchased,
				"checkout_id": resp.Data.CheckoutID,
				"external_id": resp.Data.ExternalID,
			}, http.StatusOK))
			return
		}

		if err := h.refreshInstanceLicenser(effectiveKey); err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to refresh license entitlements", http.StatusInternalServerError))
			return
		}

		_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout completion checked", map[string]interface{}{
			"status":      resp.Data.Status,
			"license_key": purchased,
			"checkout_id": resp.Data.CheckoutID,
			"external_id": resp.Data.ExternalID,
		}, http.StatusOK))
		return
	}

	attempt.Status = resp.Data.Status
	cfg.CheckoutAttempts[attemptID] = attempt
	// Not completed/licensed: record the latest attempt status only. This path
	// must not write the license columns, so a concurrent completion that
	// persisted a license is not clobbered by this stale snapshot.
	if err := cfgSvc.UpdateCheckoutAttempts(r.Context(), cfg); err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout completion checked", map[string]interface{}{
		"status":      resp.Data.Status,
		"license_key": resp.Data.LicenseKey,
		"checkout_id": resp.Data.CheckoutID,
		"external_id": resp.Data.ExternalID,
	}, http.StatusAccepted))
}
