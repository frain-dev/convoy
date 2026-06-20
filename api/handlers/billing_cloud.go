package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
)

func (h *BillingHandler) ensureOrganisationInBilling(w http.ResponseWriter, r *http.Request, orgID string) bool {
	orgRepo := h.orgRepo()
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

// getOrCreateBillingOrg fetches the billing organisation, creating it on first use when
// the billing service reports it missing and then refetching. On any rendered error it
// returns ok=false and the caller must return.
func (h *BillingHandler) getOrCreateBillingOrg(w http.ResponseWriter, r *http.Request, orgID string) (*billing.Response[billing.BillingOrganisation], bool) {
	resp, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && isBillingOrgNotFound(err) {
		if h.ensureOrganisationInBilling(w, r, orgID) {
			return nil, false
		}
		resp, err = h.BillingClient.GetOrganisation(r.Context(), orgID)
	}
	if err != nil {
		renderBillingError(w, r, err)
		return nil, false
	}
	return resp, true
}

// renderBillingError renders a billing service failure as a 500. Endpoints that
// intentionally map the same failure to a different status (e.g. GetUsage returns 503)
// keep their own rendering instead of calling this.
func renderBillingError(w http.ResponseWriter, r *http.Request, err error) {
	_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
}

// updateBillingEmailIfEmpty backfills the organisation's billing email when the billing
// service has none. It is best-effort and fire-and-forget: it runs in its own goroutine
// with a background context, and the failure policy is fail-open (errors are only logged,
// never surfaced to the caller).
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

func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetUsage(r.Context(), orgID)
	if err != nil {
		// GetUsage intentionally maps billing failures to 503, unlike the other
		// cloud reads which use renderBillingError (500). Preserved per A11.
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(resp.Message, resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetInvoices(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetInvoices(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Invoices retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	// Unlike GetOrganisation/GetInternalOrganisationID, this path only triggers a billing
	// org create on "not found" and otherwise tolerates GetOrganisation errors, so it is
	// not folded into getOrCreateBillingOrg (which would render on any error).
	_, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && isBillingOrgNotFound(err) {
		if h.ensureOrganisationInBilling(w, r, orgID) {
			return
		}
	}

	resp, err := h.BillingClient.GetSubscription(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	h.updateOrganisationStatus(r.Context(), orgID, billing.HasActiveSubscription(resp.Data))
	h.updateBillingEmailIfEmpty(orgID)

	_ = render.Render(w, r, util.NewServerResponse("Subscription retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetPaymentMethods(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetPaymentMethods(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
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
		renderBillingError(w, r, err)
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
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment method deleted successfully", resp.Data, http.StatusOK))
}

// Organisation handlers
func (h *BillingHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, ok := h.getOrCreateBillingOrg(w, r, orgID)
	if !ok {
		return
	}

	h.updateBillingEmailIfEmpty(orgID)
	_ = render.Render(w, r, util.NewServerResponse("Organisation retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	var orgData billing.BillingOrganisation
	if err := json.NewDecoder(r.Body).Decode(&orgData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisation(r.Context(), orgID, orgData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisationTaxID(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	var taxData billing.UpdateOrganisationTaxIDRequest
	if err := json.NewDecoder(r.Body).Decode(&taxData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisationTaxID(r.Context(), orgID, taxData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Tax ID updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateOrganisationAddress(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	var addressData billing.UpdateOrganisationAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&addressData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateOrganisationAddress(r.Context(), orgID, addressData)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Address updated successfully", resp.Data, http.StatusOK))
}

// Subscription handlers
func (h *BillingHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSubscriptions(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscriptions retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) OnboardSubscription(w http.ResponseWriter, r *http.Request) {
	// Only require billing access, not enabled organisation - allows onboarding even if org is disabled
	orgID, ok := h.orgGuard(w, r)
	if !ok {
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
		renderBillingError(w, r, err)
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
		renderBillingError(w, r, err)
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
		renderBillingError(w, r, err)
		return
	}

	// A cancelled subscription is always inactive; resp.Data here is an opaque payload
	// (Response[interface{}]) that never carried active state, so pass active=false.
	h.updateOrganisationStatus(r.Context(), orgID, false)

	_ = render.Render(w, r, util.NewServerResponse("Subscription cancelled successfully", resp.Data, http.StatusOK))
}

// Payment method handlers
func (h *BillingHandler) GetSetupIntent(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, err := h.BillingClient.GetSetupIntent(r.Context(), orgID)
	if err != nil {
		renderBillingError(w, r, err)
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
		renderBillingError(w, r, err)
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
	orgID, ok := h.orgGuard(w, r)
	if !ok {
		return
	}

	resp, ok := h.getOrCreateBillingOrg(w, r, orgID)
	if !ok {
		return
	}

	h.updateBillingEmailIfEmpty(orgID)

	responseData := map[string]interface{}{
		"id": resp.Data.ID,
	}

	_ = render.Render(w, r, util.NewServerResponse("Internal organisation ID retrieved successfully", responseData, http.StatusOK))
}

func (h *BillingHandler) updateOrganisationStatus(ctx context.Context, orgID string, active bool) {
	orgRepo := h.orgRepo()
	org, err := orgRepo.FetchOrganisationByID(ctx, orgID)
	if err != nil {
		h.A.Logger.Errorf("Failed to fetch organisation %s for disabled status update: %v", orgID, err)
		return
	}

	if !billing.ApplySubscriptionStatus(org, active) {
		return
	}

	if err := orgRepo.UpdateOrganisation(ctx, org); err != nil {
		if active {
			h.A.Logger.Errorf("Failed to clear organisation %s disabled_at: %v", orgID, err)
		} else {
			h.A.Logger.Errorf("Failed to set organisation %s disabled_at: %v", orgID, err)
		}
		return
	}
	if active {
		h.A.Logger.Infof("Cleared organisation %s disabled_at - subscription active", orgID)
	} else {
		h.A.Logger.Infof("Set organisation %s disabled_at - subscription not active", orgID)
	}
}
