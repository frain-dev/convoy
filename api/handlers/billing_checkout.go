package handlers

import (
	"encoding/json"
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
	checkoutStatusStalePaid  = "stale_paid"
)

func (h *BillingHandler) GetBillingConfig(w http.ResponseWriter, r *http.Request) {
	instanceBilling, _ := configuration.New(h.A.Logger, h.A.DB).LoadInstanceBillingConfig(r.Context())
	instanceLicenseKey := ""
	if instanceBilling != nil {
		instanceLicenseKey = instanceBilling.LicenseKey
	}
	mode := h.A.Cfg.BillingMode(instanceLicenseKey)
	selfHosted := map[string]interface{}{
		"enabled":            mode != config.BillingModeCloud,
		"license_configured": false,
	}
	if instanceBilling != nil {
		selfHosted["license_configured"] = instanceBilling.LicenseKey != ""
		selfHosted["license_synced_at"] = instanceBilling.LicenseSyncedAt
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
	if email == "" {
		_ = render.Render(w, r, util.NewErrorResponse("email is required", http.StatusBadRequest))
		return
	}
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
	attempt := datastore.SelfHostedCheckoutAttempt{
		AttemptID:         attemptID,
		CheckoutNonce:     nonce,
		CheckoutNonceHash: nonceHash,
		Email:             email,
		PlanID:            req.PlanID,
		Interval:          req.Interval,
		Status:            checkoutStatusPending,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	cfg.CheckoutAttempts[attemptID] = attempt
	cfg.ActiveCheckoutAttemptID = attemptID

	if err := cfgSvc.UpdateInstanceBillingConfig(r.Context(), cfg); err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp, err := h.BillingClient.StartGuestCheckout(r.Context(), billing.StartGuestCheckoutRequest{
		Email:             email,
		PlanID:            req.PlanID,
		Interval:          req.Interval,
		Host:              host,
		OrganisationName:  h.activeOrganisationName(r.Context(), r),
		AttemptID:         attemptID,
		CheckoutNonceHash: nonceHash,
	})
	if err != nil {
		attempt.Status = checkoutStatusFailed
		attempt.UpdatedAt = time.Now()
		cfg.CheckoutAttempts[attemptID] = attempt
		cfg.ActiveCheckoutAttemptID = ""
		_ = cfgSvc.UpdateInstanceBillingConfig(r.Context(), cfg)
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	attempt.CheckoutID = resp.Data.CheckoutID
	attempt.CheckoutURL = resp.Data.CheckoutURL
	attempt.UpdatedAt = time.Now()
	cfg.CheckoutAttempts[attemptID] = attempt
	cfg.CheckoutID = resp.Data.CheckoutID

	if err := cfgSvc.UpdateInstanceBillingConfig(r.Context(), cfg); err != nil {
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
	if !ok || attempt.CheckoutNonce == "" {
		_ = render.Render(w, r, util.NewErrorResponse("active checkout attempt not found", http.StatusNotFound))
		return
	}
	isActiveAttempt := attemptID == cfg.ActiveCheckoutAttemptID
	if !isActiveAttempt {
		attempt.Status = checkoutStatusSuperseded
		attempt.CheckoutNonce = ""
		attempt.CheckoutNonceHash = ""
		attempt.UpdatedAt = time.Now()
		cfg.CheckoutAttempts[attemptID] = attempt
		if err := cfgSvc.UpdateInstanceBillingConfig(r.Context(), cfg); err != nil {
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
		if !isActiveAttempt {
			attempt.Status = checkoutStatusStalePaid
			attempt.CompletedAt = null.NewTime(now, true)
			attempt.CheckoutNonce = ""
			attempt.LastCompletionStatus = checkoutStatusStalePaid
			cfg.CheckoutAttempts[attemptID] = attempt
			if err := cfgSvc.UpdateInstanceBillingConfig(r.Context(), cfg); err != nil {
				_ = render.Render(w, r, util.NewServiceErrResponse(err))
				return
			}
			_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout attempt is no longer active", map[string]interface{}{
				"status":      "stale",
				"checkout_id": resp.Data.CheckoutID,
				"external_id": resp.Data.ExternalID,
			}, http.StatusConflict))
			return
		}
		attempt.Status = checkoutStatusCompleted
		attempt.CompletedAt = null.NewTime(now, true)
		attempt.CheckoutNonce = ""
		cfg.LicenseKey = resp.Data.LicenseKey
		cfg.CheckoutID = resp.Data.CheckoutID
		cfg.ExternalID = resp.Data.ExternalID
		cfg.LicenseSyncedAt = null.NewTime(now, true)
		cfg.ActiveCheckoutAttemptID = ""
	} else {
		attempt.Status = resp.Data.Status
	}
	cfg.CheckoutAttempts[attemptID] = attempt

	if err := cfgSvc.UpdateInstanceBillingConfig(r.Context(), cfg); err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if resp.Data.Status == checkoutStatusCompleted && resp.Data.LicenseKey != "" {
		if err := h.refreshInstanceLicenser(resp.Data.LicenseKey); err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to refresh license entitlements", http.StatusInternalServerError))
			return
		}
	}

	status := http.StatusAccepted
	if resp.Data.Status == checkoutStatusCompleted {
		status = http.StatusOK
	}
	_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout completion checked", map[string]interface{}{
		"status":      resp.Data.Status,
		"license_key": resp.Data.LicenseKey,
		"checkout_id": resp.Data.CheckoutID,
		"external_id": resp.Data.ExternalID,
	}, status))
}
