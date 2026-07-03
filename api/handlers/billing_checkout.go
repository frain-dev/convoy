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

type startSelfHostedTrialRequest struct {
	Email string `json:"email"`
	Host  string `json:"host"`
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
	if err != nil && !errors.Is(err, datastore.ErrConfigNotFound) {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
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
		selfHosted["resubscribe"] = config.ResolveCheckoutLicenseKey(instanceBilling.CheckoutLicenseKey, instanceBilling.LicenseKey, instanceBilling.LicenseKeySource) != ""
		if h.canManageSelfHostedBilling(r) {
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
	if mode == config.BillingModeOSS && instanceLicenseKey == "" && h.BillingClient != nil {
		if catalog, err := h.BillingClient.GetSelfHostedCatalog(r.Context()); err == nil && catalog.TrialOffer != nil {
			selfHosted["trial_offer"] = catalog.TrialOffer
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
	if !h.requireSelfHostedBillingRoute(w, r, "self-hosted checkout is not available in cloud org billing mode") {
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

	resubscribeKey := config.ResolveCheckoutLicenseKey(cfg.CheckoutLicenseKey, cfg.LicenseKey, cfg.LicenseKeySource)

	if err := validateResubscribeEmail(email, resubscribeKey); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
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

	// Call billing before mutating local checkout state.
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
		renderBillingClientError(w, r, err, http.StatusServiceUnavailable)
		return
	}

	now := time.Now()
	ensureCheckoutAttemptsMap(cfg)
	supersedeActiveCheckoutAttempt(cfg, now)
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

	// Persist attempt bookkeeping only; license columns are written on completion.
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

// StartSelfHostedTrial stores the license key returned by the billing service.
func (h *BillingHandler) StartSelfHostedTrial(w http.ResponseWriter, r *http.Request) {
	if !h.requireSelfHostedBillingRoute(w, r, "self-hosted trial is not available in cloud org billing mode") {
		return
	}

	var req startSelfHostedTrialRequest
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

	resubscribeKey := config.ResolveCheckoutLicenseKey(cfg.CheckoutLicenseKey, cfg.LicenseKey, cfg.LicenseKeySource)
	email := strings.TrimSpace(req.Email)
	if err := validateResubscribeEmail(email, resubscribeKey); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	host, err := optionalCanonicalHost(req.Host)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	attemptID := ulid.Make().String()
	now := time.Now()
	ensureCheckoutAttemptsMap(cfg)
	supersedeActiveCheckoutAttempt(cfg, now)
	cfg.CheckoutAttempts[attemptID] = datastore.SelfHostedCheckoutAttempt{
		AttemptID: attemptID,
		Status:    checkoutStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	cfg.ActiveCheckoutAttemptID = attemptID

	if err := cfgSvc.UpdateCheckoutAttempts(r.Context(), cfg); err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	recordTrialAttemptFailure := func() bool {
		failActiveCheckoutAttempt(cfg, attemptID, time.Now())
		if err := cfgSvc.UpdateCheckoutAttempts(r.Context(), cfg); err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return false
		}
		return true
	}

	resp, err := h.BillingClient.StartSelfHostedTrial(r.Context(), billing.StartSelfHostedTrialRequest{
		Email:            email,
		LicenseKey:       resubscribeKey,
		Host:             host,
		OrganisationName: h.activeOrganisationName(r.Context(), r),
		AttemptID:        attemptID,
	})
	if err != nil {
		if billingClientErrorIsDefinitive(err) {
			if !recordTrialAttemptFailure() {
				return
			}
		}
		renderBillingClientError(w, r, err, http.StatusServiceUnavailable)
		return
	}
	purchased := strings.TrimSpace(resp.Data.LicenseKey)
	if purchased == "" {
		if !recordTrialAttemptFailure() {
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("trial license key was not returned by the billing service", http.StatusBadGateway))
		return
	}

	attempt := cfg.CheckoutAttempts[attemptID]
	attempt.Status = checkoutStatusCompleted
	attempt.LastCompletionStatus = resp.Data.Status
	attempt.ExternalID = resp.Data.ExternalID
	attempt.CompletedAt = null.NewTime(now, true)
	attempt.UpdatedAt = now
	if !h.finalizePurchasedLicense(w, r, cfgSvc, cfg, attemptID, attempt, purchased, func(c *datastore.Configuration, a *datastore.SelfHostedCheckoutAttempt) {
		c.ExternalID = resp.Data.ExternalID
	}) {
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Self-hosted trial started", map[string]interface{}{
		"status":      checkoutStatusCompleted,
		"license_key": purchased,
		"external_id": resp.Data.ExternalID,
		"trial":       true,
	}, http.StatusOK))
}

func (h *BillingHandler) CompleteSelfHostedCheckout(w http.ResponseWriter, r *http.Request) {
	if !h.requireSelfHostedBillingRoute(w, r, "self-hosted checkout is not available in cloud org billing mode") {
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

		attempt.Status = checkoutStatusCompleted
		attempt.CompletedAt = null.NewTime(now, true)
		attempt.CheckoutNonce = ""
		if !h.finalizePurchasedLicense(w, r, cfgSvc, cfg, attemptID, attempt, purchased, func(c *datastore.Configuration, a *datastore.SelfHostedCheckoutAttempt) {
			c.CheckoutID = resp.Data.CheckoutID
			c.ExternalID = resp.Data.ExternalID
		}) {
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
