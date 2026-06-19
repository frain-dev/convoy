package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licenseservice "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/util"
)

var ErrHostRequiredForBilling = errors.New("organisation host (assigned domain) is required for billing. Please set the assigned domain in the configuration")
var ErrOwnerEmailRequiredForBilling = errors.New("organisation owner email is required for billing")

type BillingHandler struct {
	*Handler
	BillingClient billing.Client
}

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

func isBillingOrgNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "organisation") && strings.Contains(s, "not found")
}

func (h *BillingHandler) ensureOrganisationInBilling(w http.ResponseWriter, r *http.Request, orgID string) bool {
	orgRepo := organisations.New(h.A.Logger, h.A.DB)
	org, err := orgRepo.FetchOrganisationByID(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to fetch organisation data", http.StatusInternalServerError))
		return true
	}

	cfg, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to fetch config", http.StatusInternalServerError))
		return true
	}

	if cfg.Host == "" {
		_ = render.Render(w, r, util.NewErrorResponse(ErrHostRequiredForBilling.Error(), http.StatusBadRequest))
		return true
	}

	ownerEmail := h.getOwnerEmail(r.Context(), orgID)
	if ownerEmail == "" {
		_ = render.Render(w, r, util.NewErrorResponse(ErrOwnerEmailRequiredForBilling.Error(), http.StatusUnprocessableEntity))
		return true
	}

	orgData := billing.BillingOrganisation{
		Name:         org.Name,
		ExternalID:   orgID,
		BillingEmail: ownerEmail,
		Host:         cfg.Host,
	}

	_, createErr := h.BillingClient.CreateOrganisation(r.Context(), orgData)
	if createErr != nil {
		errorMsg := createErr.Error()
		if strings.Contains(errorMsg, "Validation failed") {
			_ = render.Render(w, r, util.NewErrorResponse(errorMsg, http.StatusBadRequest))
		} else {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Failed to create organisation in billing service: %s", errorMsg), http.StatusInternalServerError))
		}
		return true
	}
	return false
}

func (h *BillingHandler) checkBillingAccess(w http.ResponseWriter, r *http.Request, orgID string) bool {
	if !h.A.Cfg.UsesOrgBilling() {
		_ = render.Render(w, r, util.NewErrorResponse("cloud org billing is not configured", http.StatusServiceUnavailable))
		return false
	}

	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return false
	}

	org, err := h.retrieveOrganisation(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("organisation not found", http.StatusNotFound))
		return false
	}

	if org.UID != orgID {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID mismatch", http.StatusForbidden))
		return false
	}

	if err := h.A.Authz.Authorize(r.Context(), string(policies.PermissionBillingManage), org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: billing access requires billing admin or organisation admin role", http.StatusForbidden))
		return false
	}

	return true
}

func (h *BillingHandler) getOwnerEmail(ctx context.Context, orgID string) string {
	orgRepo := organisations.New(h.A.Logger, h.A.DB)
	org, err := orgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil {
		return ""
	}

	userRepo := users.New(h.A.Logger, h.A.DB)
	owner, err := userRepo.FindUserByID(ctx, org.OwnerID)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(owner.Email)
}

func (h *BillingHandler) requireInstanceAdmin(w http.ResponseWriter, r *http.Request) bool {
	user, err := h.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("user not found", http.StatusUnauthorized))
		return false
	}

	memberRepo := organisation_members.New(h.A.Logger, h.A.DB)
	if _, err := memberRepo.FetchInstanceAdminByUserID(r.Context(), user.UID); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
		return false
	}

	return true
}

func (h *BillingHandler) requireSelfHostedBillingAdmin(w http.ResponseWriter, r *http.Request) bool {
	if h.canManageSelfHostedBilling(r) {
		return true
	}

	_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: organisation admin access required", http.StatusForbidden))
	return false
}

func (h *BillingHandler) canManageSelfHostedBilling(r *http.Request) bool {
	user, err := h.retrieveUser(r)
	if err != nil {
		return false
	}

	memberRepo := organisation_members.New(h.A.Logger, h.A.DB)
	if _, err = memberRepo.FetchInstanceAdminByUserID(r.Context(), user.UID); err == nil {
		return true
	}
	if h.A.Cfg.UsesOrgBilling() {
		return false
	}

	_, err = memberRepo.FetchAnyOrganisationAdminByUserID(r.Context(), user.UID)
	return err == nil
}

func (h *BillingHandler) isInstanceAdmin(r *http.Request) bool {
	user, err := h.retrieveUser(r)
	if err != nil {
		return false
	}

	memberRepo := organisation_members.New(h.A.Logger, h.A.DB)
	_, err = memberRepo.FetchInstanceAdminByUserID(r.Context(), user.UID)
	return err == nil
}

func newCheckoutNonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashCheckoutNonce(nonce string) string {
	sum := sha256.Sum256([]byte(nonce))
	return hex.EncodeToString(sum[:])
}

func (h *BillingHandler) activeOrganisationName(ctx context.Context, r *http.Request) string {
	orgID := strings.TrimSpace(r.Header.Get("X-Organisation-Id"))
	if orgID == "" {
		return ""
	}

	orgRepo := organisations.New(h.A.Logger, h.A.DB)
	org, err := orgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil || org == nil {
		return ""
	}

	return strings.TrimSpace(org.Name)
}

func (h *BillingHandler) updateBillingEmailIfEmpty(orgID string) {
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := h.BillingClient.GetOrganisation(bgCtx, orgID)
		if err != nil {
			return
		}

		if resp.Data.BillingEmail != "" {
			return
		}

		ownerEmail := h.getOwnerEmail(bgCtx, orgID)
		if ownerEmail == "" {
			return
		}

		updateData := billing.BillingOrganisation{
			BillingEmail: ownerEmail,
		}
		_, updateErr := h.BillingClient.UpdateOrganisation(bgCtx, orgID, updateData)
		if updateErr != nil {
			h.A.Logger.Warnf("Failed to update billing_email for organisation %s: %v", orgID, updateErr)
		} else {
			h.A.Logger.Infof("Updated billing_email for organisation %s", orgID)
		}
	}()
}

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
			activeAttempt.Status = "superseded"
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
		Status:            "pending",
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
		attempt.Status = "failed"
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
		attempt.Status = "superseded"
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
	if resp.Data.Status == "completed" && resp.Data.LicenseKey != "" {
		if !isActiveAttempt {
			attempt.Status = "stale_paid"
			attempt.CompletedAt = null.NewTime(now, true)
			attempt.CheckoutNonce = ""
			attempt.LastCompletionStatus = "stale_paid"
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
		attempt.Status = "completed"
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

	if resp.Data.Status == "completed" && resp.Data.LicenseKey != "" {
		if err := h.refreshInstanceLicenser(resp.Data.LicenseKey); err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to refresh license entitlements", http.StatusInternalServerError))
			return
		}
	}

	status := http.StatusAccepted
	if resp.Data.Status == "completed" {
		status = http.StatusOK
	}
	_ = render.Render(w, r, util.NewServerResponse("Self-hosted checkout completion checked", map[string]interface{}{
		"status":      resp.Data.Status,
		"license_key": resp.Data.LicenseKey,
		"checkout_id": resp.Data.CheckoutID,
		"external_id": resp.Data.ExternalID,
	}, status))
}

func (h *BillingHandler) selfHostedLicenseKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	if !h.requireSelfHostedBillingAdmin(w, r) {
		return "", false
	}
	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return "", false
	}

	instanceBilling, err := configuration.New(h.A.Logger, h.A.DB).LoadInstanceBillingConfig(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return "", false
	}

	licenseKey := strings.TrimSpace(instanceBilling.LicenseKey)
	if licenseKey == "" {
		_ = render.Render(w, r, util.NewErrorResponse("self-hosted license is not configured", http.StatusForbidden))
		return "", false
	}

	mode := h.A.Cfg.BillingMode(licenseKey)
	if mode != config.BillingModeLicensedSelfHosted {
		_ = render.Render(w, r, util.NewErrorResponse("licensed self-hosted billing is not configured", http.StatusForbidden))
		return "", false
	}

	return licenseKey, true
}

func (h *BillingHandler) GetSelfHostedSubscription(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSelfHostedSubscription(r.Context(), licenseKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(resp.Message, resp.Data, http.StatusOK))
}

func (h *BillingHandler) DeleteSelfHostedSubscription(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.DeleteSelfHostedSubscription(r.Context(), licenseKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	if err := h.refreshInstanceLicenser(licenseKey); err != nil {
		if fallbackErr := h.useBillingRequiredLicenser(); fallbackErr != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to refresh license entitlements", http.StatusInternalServerError))
			return
		}
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription cancelled successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSelfHostedOrganisation(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSelfHostedOrganisation(r.Context(), licenseKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(resp.Message, resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateSelfHostedOrganisationTaxID(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	var taxData billing.UpdateOrganisationTaxIDRequest
	if err := json.NewDecoder(r.Body).Decode(&taxData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateSelfHostedOrganisationTaxID(r.Context(), licenseKey, taxData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Tax ID updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateSelfHostedOrganisationAddress(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	var addressData billing.UpdateOrganisationAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&addressData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateSelfHostedOrganisationAddress(r.Context(), licenseKey, addressData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Address updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSelfHostedInvoices(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSelfHostedInvoices(r.Context(), licenseKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(resp.Message, resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSelfHostedInvoice(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	invoiceID := strings.TrimSpace(chi.URLParam(r, "invoiceID"))
	if invoiceID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("invoice ID is required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.GetSelfHostedInvoice(r.Context(), licenseKey, invoiceID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(resp.Message, resp.Data, http.StatusOK))
}

func (h *BillingHandler) DownloadSelfHostedInvoice(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	invoiceID := strings.TrimSpace(chi.URLParam(r, "invoiceID"))
	if invoiceID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("invoice ID is required", http.StatusBadRequest))
		return
	}

	pdfResp, err := h.BillingClient.DownloadSelfHostedInvoice(r.Context(), licenseKey, invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			_ = render.Render(w, r, util.NewErrorResponse("Invoice PDF link not available", http.StatusNotFound))
		} else {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Failed to download invoice: %s", err.Error()), http.StatusServiceUnavailable))
		}
		return
	}
	defer pdfResp.Body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%s.pdf"`, invoiceID))

	if _, err = io.Copy(w, pdfResp.Body); err != nil {
		h.A.Logger.Error("Failed to stream PDF to client", "error", err)
		return
	}
}

// GetSelfHostedUsage returns usage for the current organisation computed from this
// instance's own event data. Self-hosted usage is local data, so it never calls the
// billing provider; the cloud GetUsage path keeps using the provider.
func (h *BillingHandler) GetSelfHostedUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.selfHostedLicenseKey(w, r); !ok {
		return
	}

	org, err := h.retrieveOrganisation(r)
	if err != nil || org == nil {
		_ = render.Render(w, r, util.NewErrorResponse("organisation not found", http.StatusNotFound))
		return
	}

	if err := h.A.Authz.Authorize(r.Context(), string(policies.PermissionBillingManage), org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: billing access requires billing admin or organisation admin role", http.StatusForbidden))
		return
	}

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

	orgRepo := organisations.New(h.A.Logger, h.A.DB)
	usage, err := orgRepo.CalculateUsage(r.Context(), org.UID, startOfMonth, endOfMonth)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("failed to calculate usage: %s", err.Error()), http.StatusInternalServerError))
		return
	}

	usageResponse := map[string]interface{}{
		"organisation_id": usage.OrganisationID,
		"period":          usage.Period,
		"received": map[string]interface{}{
			"volume": usage.Received.Volume,
			"bytes":  usage.Received.Bytes,
		},
		"sent": map[string]interface{}{
			"volume": usage.Sent.Volume,
			"bytes":  usage.Sent.Bytes,
		},
		"created_at": usage.CreatedAt,
	}

	_ = render.Render(w, r, util.NewServerResponse("Usage retrieved successfully", usageResponse, http.StatusOK))
}

func (h *BillingHandler) GetSelfHostedPaymentMethods(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSelfHostedPaymentMethods(r.Context(), licenseKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(resp.Message, resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSelfHostedSetupIntent(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSelfHostedSetupIntent(r.Context(), licenseKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Setup intent retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) SetDefaultSelfHostedPaymentMethod(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	pmID := strings.TrimSpace(chi.URLParam(r, "pmID"))
	if pmID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("payment method ID is required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.SetDefaultSelfHostedPaymentMethod(r.Context(), licenseKey, pmID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Default payment method set successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) DeleteSelfHostedPaymentMethod(w http.ResponseWriter, r *http.Request) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	pmID := strings.TrimSpace(chi.URLParam(r, "pmID"))
	if pmID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("payment method ID is required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.DeleteSelfHostedPaymentMethod(r.Context(), licenseKey, pmID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment method deleted successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) refreshInstanceLicenser(licenseKey string) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}
	cfg.LicenseKey = licenseKey

	licenseClient := licenseservice.NewClient(licenseservice.Config{
		Host:         cfg.LicenseService.Host,
		ValidatePath: cfg.LicenseService.ValidatePath,
		Timeout:      cfg.LicenseService.Timeout,
		RetryCount:   cfg.LicenseService.RetryCount,
		Logger:       h.A.Logger,
	})

	licenser, err := license.NewLicenser(&license.Config{
		LicenseService: licenseservice.LicenserConfig{
			LicenseKey:    cfg.LicenseKey,
			UseOrgBilling: cfg.UsesOrgBilling(),
			Client:        licenseClient,
			OrgRepo:       organisations.New(h.A.Logger, h.A.DB),
			UserRepo:      users.New(h.A.Logger, h.A.DB),
			ProjectRepo:   projects.New(h.A.Logger, h.A.DB),
			Logger:        h.A.Logger,
		},
	})
	if err != nil {
		return err
	}

	h.A.Licenser = licenser
	h.A.Cfg = cfg
	return config.Override(&cfg)
}

func (h *BillingHandler) useBillingRequiredLicenser() error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	licenser, err := license.NewLicenser(&license.Config{
		LicenseService: licenseservice.LicenserConfig{
			UseOrgBilling: true,
			OrgRepo:       organisations.New(h.A.Logger, h.A.DB),
			UserRepo:      users.New(h.A.Logger, h.A.DB),
			ProjectRepo:   projects.New(h.A.Logger, h.A.DB),
			Logger:        h.A.Logger,
		},
	})
	if err != nil {
		return err
	}

	h.A.Licenser = licenser
	h.A.Cfg = cfg
	return nil
}

func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetUsage(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(resp.Message, resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetInvoices(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetInvoices(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invoices retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	_, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && isBillingOrgNotFound(err) {
		if h.ensureOrganisationInBilling(w, r, orgID) {
			return
		}
	}

	resp, err := h.BillingClient.GetSubscription(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	h.updateOrganisationStatus(r.Context(), orgID, resp.Data)
	h.updateBillingEmailIfEmpty(orgID)

	_ = render.Render(w, r, util.NewServerResponse("Subscription retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetPaymentMethods(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetPaymentMethods(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment methods retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) SetDefaultPaymentMethod(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	pmID := chi.URLParam(r, "pmID")
	if orgID == "" || pmID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and payment method ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.SetDefaultPaymentMethod(r.Context(), orgID, pmID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Default payment method set successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) DeletePaymentMethod(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	pmID := chi.URLParam(r, "pmID")
	if orgID == "" || pmID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and payment method ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.DeletePaymentMethod(r.Context(), orgID, pmID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment method deleted successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return
	}

	resp, err := h.BillingClient.GetPlans(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	configPlans := make([]billing.Plan, 0, len(h.A.Cfg.Billing.Plans))
	for _, p := range h.A.Cfg.Billing.Plans {
		configPlans = append(configPlans, billing.Plan{ID: p.ID, Name: p.Name, ProductType: p.ProductType})
	}

	mergedPlans := h.mergePlansWithFeatures(resp.Data, configPlans)

	_ = render.Render(w, r, util.NewServerResponse("Plans retrieved successfully", mergedPlans, http.StatusOK))
}

func (h *BillingHandler) mergePlansWithFeatures(plans, configPlans []billing.Plan) []interface{} {
	configPlansMap := make(map[string]map[string]interface{})
	for _, plan := range configPlans {
		if plan.Name == "" {
			continue
		}
		planJSON, err := json.Marshal(plan)
		if err != nil {
			continue
		}
		var planMap map[string]interface{}
		if err := json.Unmarshal(planJSON, &planMap); err != nil {
			continue
		}
		configPlansMap[strings.ToLower(plan.Name)] = planMap
	}

	mergedPlans := make([]interface{}, 0, len(plans))
	for _, plan := range plans {
		planJSON, err := json.Marshal(plan)
		if err != nil {
			continue
		}
		var planMap map[string]interface{}
		if err := json.Unmarshal(planJSON, &planMap); err != nil {
			continue
		}

		configPlanMap, found := configPlansMap[strings.ToLower(plan.Name)]
		if found {
			mergedPlan := make(map[string]interface{})
			for k, v := range configPlanMap {
				mergedPlan[k] = v
			}
			for k, v := range planMap {
				mergedPlan[k] = v
			}
			mergedPlans = append(mergedPlans, mergedPlan)
		} else {
			mergedPlans = append(mergedPlans, planMap)
		}
	}

	return mergedPlans
}

func (h *BillingHandler) GetTaxIDTypes(w http.ResponseWriter, r *http.Request) {
	if h.BillingClient == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return
	}

	resp, err := h.BillingClient.GetTaxIDTypes(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Tax ID types retrieved successfully", resp.Data, http.StatusOK))
}

// Organisation handlers
func (h *BillingHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
	var orgData billing.BillingOrganisation
	if err := json.NewDecoder(r.Body).Decode(&orgData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.CreateOrganisation(r.Context(), orgData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation created successfully", resp.Data, http.StatusCreated))
}

func (h *BillingHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && isBillingOrgNotFound(err) {
		if h.ensureOrganisationInBilling(w, r, orgID) {
			return
		}
		resp, err = h.BillingClient.GetOrganisation(r.Context(), orgID)
	}
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	h.updateBillingEmailIfEmpty(orgID)
	_ = render.Render(w, r, util.NewServerResponse("Organisation retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	var orgData billing.BillingOrganisation
	if err := json.NewDecoder(r.Body).Decode(&orgData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisation(r.Context(), orgID, orgData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisationTaxID(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	var taxData billing.UpdateOrganisationTaxIDRequest
	if err := json.NewDecoder(r.Body).Decode(&taxData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisationTaxID(r.Context(), orgID, taxData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Tax ID updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisationAddress(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	var addressData billing.UpdateOrganisationAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&addressData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisationAddress(r.Context(), orgID, addressData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Address updated successfully", resp.Data, http.StatusOK))
}

// Subscription handlers
func (h *BillingHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetSubscriptions(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) OnboardSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	// Only require billing access, not enabled organisation - allows onboarding even if org is disabled
	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	var requestData billing.OnboardSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if requestData.PlanID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("plan_id is required and must be a valid UUID", http.StatusBadRequest))
		return
	}

	if requestData.Host == "" {
		_ = render.Render(w, r, util.NewErrorResponse("host is required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.OnboardSubscription(r.Context(), orgID, requestData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Checkout session created successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpgradeSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	subscriptionID := chi.URLParam(r, "subscriptionID")
	if orgID == "" || subscriptionID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and subscription ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	var requestData billing.UpgradeSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if requestData.PlanID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("plan_id is required and must be a valid UUID", http.StatusBadRequest))
		return
	}

	if requestData.Host == "" {
		_ = render.Render(w, r, util.NewErrorResponse("host is required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpgradeSubscription(r.Context(), orgID, subscriptionID, requestData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Checkout session created successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	subscriptionID := chi.URLParam(r, "subscriptionID")
	if orgID == "" || subscriptionID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and subscription ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.DeleteSubscription(r.Context(), orgID, subscriptionID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	h.updateOrganisationStatus(r.Context(), orgID, resp.Data)

	_ = render.Render(w, r, util.NewServerResponse("Subscription cancelled successfully", resp.Data, http.StatusOK))
}

// Payment method handlers
func (h *BillingHandler) GetSetupIntent(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetSetupIntent(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Setup intent retrieved successfully", resp.Data, http.StatusOK))
}

// Invoice handlers
func (h *BillingHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	invoiceID := chi.URLParam(r, "invoiceID")
	if orgID == "" || invoiceID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and invoice ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetInvoice(r.Context(), orgID, invoiceID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invoice retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) DownloadInvoice(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	invoiceID := chi.URLParam(r, "invoiceID")
	if orgID == "" || invoiceID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and invoice ID are required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	pdfResp, err := h.BillingClient.DownloadInvoice(r.Context(), orgID, invoiceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			_ = render.Render(w, r, util.NewErrorResponse("Invoice not found", http.StatusNotFound))
		} else if strings.Contains(err.Error(), "PDF link not found") {
			_ = render.Render(w, r, util.NewErrorResponse("Invoice PDF link not available", http.StatusNotFound))
		} else {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Failed to download invoice: %s", err.Error()), http.StatusInternalServerError))
		}
		return
	}
	defer pdfResp.Body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%s.pdf"`, invoiceID))

	_, err = io.Copy(w, pdfResp.Body)
	if err != nil {
		h.A.Logger.Error("Failed to stream PDF to client", "error", err)
		return
	}
}

// GetInternalOrganisationID returns the internal organisation ID from billing service
func (h *BillingHandler) GetInternalOrganisationID(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && isBillingOrgNotFound(err) {
		if h.ensureOrganisationInBilling(w, r, orgID) {
			return
		}
		resp, err = h.BillingClient.GetOrganisation(r.Context(), orgID)
	}
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	h.updateBillingEmailIfEmpty(orgID)

	responseData := map[string]interface{}{
		"id": resp.Data.ID,
	}

	_ = render.Render(w, r, util.NewServerResponse("Internal organisation ID retrieved successfully", responseData, http.StatusOK))
}

func (h *BillingHandler) updateOrganisationStatus(ctx context.Context, orgID string, subscriptionData interface{}) {
	orgRepo := organisations.New(h.A.Logger, h.A.DB)
	org, err := orgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil {
		h.A.Logger.Errorf("Failed to fetch organisation %s for disabled status update: %v", orgID, err)
		return
	}

	isActive := billing.HasActiveSubscription(subscriptionData)

	if isActive {
		if org.DisabledAt.Valid {
			org.DisabledAt = null.Time{}
			if err := orgRepo.UpdateOrganisation(ctx, org); err != nil {
				h.A.Logger.Errorf("Failed to clear organisation %s disabled_at: %v", orgID, err)
				return
			}
			h.A.Logger.Infof("Cleared organisation %s disabled_at - subscription active", orgID)
		}
	} else {
		if !org.DisabledAt.Valid {
			org.DisabledAt = null.NewTime(time.Now(), true)
			if err := orgRepo.UpdateOrganisation(ctx, org); err != nil {
				h.A.Logger.Errorf("Failed to set organisation %s disabled_at: %v", orgID, err)
				return
			}
			h.A.Logger.Infof("Set organisation %s disabled_at - subscription not active", orgID)
		}
	}
}
