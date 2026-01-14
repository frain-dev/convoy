package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
)

var ErrHostRequiredForBilling = errors.New("organisation host (assigned domain) is required for billing. Please set the assigned domain in the configuration")

type BillingHandler struct {
	*Handler
	BillingClient billing.Client
}

func (h *BillingHandler) checkBillingAccess(w http.ResponseWriter, r *http.Request, orgID string) bool {
	if !h.A.Cfg.Billing.Enabled {
		_ = render.Render(w, r, util.NewErrorResponse("Billing module is not enabled", http.StatusForbidden))
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

	userRepo := postgres.NewUserRepo(h.A.DB)
	owner, err := userRepo.FindUserByID(ctx, org.OwnerID)
	if err != nil {
		return ""
	}

	return owner.Email
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
			h.A.Logger.WithError(updateErr).Warnf("Failed to update billing_email for organisation %s", orgID)
		} else {
			h.A.Logger.Infof("Updated billing_email for organisation %s", orgID)
		}
	}()
}

func (h *BillingHandler) GetBillingEnabled(w http.ResponseWriter, r *http.Request) {
	response := map[string]bool{
		"enabled": h.A.Cfg.Billing.Enabled,
	}

	_ = render.Render(w, r, util.NewServerResponse("Billing status retrieved", response, http.StatusOK))
}

func (h *BillingHandler) GetBillingConfig(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"enabled": h.A.Cfg.Billing.Enabled,
		"payment_provider": map[string]interface{}{
			"type":            h.A.Cfg.Billing.PaymentProvider.Type,
			"publishable_key": h.A.Cfg.Billing.PaymentProvider.PublishableKey,
		},
	}

	_ = render.Render(w, r, util.NewServerResponse("Billing configuration retrieved", response, http.StatusOK))
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

	// Calculate current month period
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

	// Calculate usage from actual Convoy data using repository
	orgRepo := organisations.New(h.A.Logger, h.A.DB)
	usage, err := orgRepo.CalculateUsage(r.Context(), orgID, startOfMonth, endOfMonth)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("failed to calculate usage: %s", err.Error()), http.StatusInternalServerError))
		return
	}

	// Format response
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
	if err != nil && strings.Contains(err.Error(), "Organisation not found") {
		orgRepo := organisations.New(h.A.Logger, h.A.DB)
		org, err := orgRepo.FetchOrganisationByID(r.Context(), orgID)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to fetch organisation data", http.StatusInternalServerError))
			return
		}

		cfg, err := config.Get()
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to fetch config", http.StatusInternalServerError))
			return
		}

		// Check if host is set - required by billing service
		if cfg.Host == "" {
			_ = render.Render(w, r, util.NewErrorResponse(ErrHostRequiredForBilling.Error(), http.StatusBadRequest))
			return
		}

		ownerEmail := h.getOwnerEmail(r.Context(), orgID)
		if ownerEmail == "" {
			h.A.Logger.Warnf("Failed to fetch owner email for organisation %s, using empty billing_email", orgID)
		}

		orgData := billing.BillingOrganisation{
			Name:         org.Name,
			ExternalID:   orgID,
			BillingEmail: ownerEmail,
			Host:         cfg.Host,
		}

		_, createErr := h.BillingClient.CreateOrganisation(r.Context(), orgData)
		if createErr != nil {
			// Return the actual error message from billing service so UI can display it
			errorMsg := createErr.Error()
			if strings.Contains(errorMsg, "Validation failed") {
				// Extract the validation message for better UX
				_ = render.Render(w, r, util.NewErrorResponse(errorMsg, http.StatusBadRequest))
			} else {
				_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Failed to create organisation in billing service: %s", errorMsg), http.StatusInternalServerError))
			}
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
	if !h.A.Cfg.Billing.Enabled {
		_ = render.Render(w, r, util.NewErrorResponse("Billing module is not enabled", http.StatusForbidden))
		return
	}

	resp, err := h.BillingClient.GetPlans(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	configPlans := make([]billing.Plan, 0, len(h.A.Cfg.Billing.Plans))
	for _, configPlan := range h.A.Cfg.Billing.Plans {
		planJSON, err := json.Marshal(configPlan)
		if err != nil {
			continue
		}
		var plan billing.Plan
		if err := json.Unmarshal(planJSON, &plan); err != nil {
			continue
		}
		configPlans = append(configPlans, plan)
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
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

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
		h.A.Logger.WithError(err).Error("Failed to stream PDF to client")
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

	// Ensure organisation exists in billing service (bootstrap if needed)
	// Use the same bootstrap logic as GetSubscription
	_, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && strings.Contains(err.Error(), "Organisation not found") {
		orgRepo := organisations.New(h.A.Logger, h.A.DB)
		org, err := orgRepo.FetchOrganisationByID(r.Context(), orgID)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to fetch organisation data", http.StatusInternalServerError))
			return
		}

		cfg, err := config.Get()
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to fetch config", http.StatusInternalServerError))
			return
		}

		// Check if host is set - required by billing service
		if cfg.Host == "" {
			_ = render.Render(w, r, util.NewErrorResponse(ErrHostRequiredForBilling.Error(), http.StatusBadRequest))
			return
		}

		ownerEmail := h.getOwnerEmail(r.Context(), orgID)
		if ownerEmail == "" {
			h.A.Logger.Warnf("Failed to fetch owner email for organisation %s, using empty billing_email", orgID)
		}

		orgData := billing.BillingOrganisation{
			Name:         org.Name,
			ExternalID:   orgID,
			BillingEmail: ownerEmail,
			Host:         cfg.Host,
		}

		_, createErr := h.BillingClient.CreateOrganisation(r.Context(), orgData)
		if createErr != nil {
			// Return the actual error message from billing service so UI can display it
			errorMsg := createErr.Error()
			if strings.Contains(errorMsg, "Validation failed") {
				_ = render.Render(w, r, util.NewErrorResponse(errorMsg, http.StatusBadRequest))
			} else {
				_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Failed to create organisation in billing service: %s", errorMsg), http.StatusInternalServerError))
			}
			return
		}
	} else if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	// Now get the organisation to extract the internal ID
	resp, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
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
		h.A.Logger.WithError(err).Errorf("Failed to fetch organisation %s for disabled status update", orgID)
		return
	}

	isActive := billing.HasActiveSubscription(subscriptionData)

	if isActive {
		if org.DisabledAt.Valid {
			org.DisabledAt = null.Time{}
			if err := orgRepo.UpdateOrganisation(ctx, org); err != nil {
				h.A.Logger.WithError(err).Errorf("Failed to clear organisation %s disabled_at", orgID)
				return
			}
			h.A.Logger.Infof("Cleared organisation %s disabled_at - subscription active", orgID)
		}
	} else {
		if !org.DisabledAt.Valid {
			org.DisabledAt = null.NewTime(time.Now(), true)
			if err := orgRepo.UpdateOrganisation(ctx, org); err != nil {
				h.A.Logger.WithError(err).Errorf("Failed to set organisation %s disabled_at", orgID)
				return
			}
			h.A.Logger.Infof("Set organisation %s disabled_at - subscription not active", orgID)
		}
	}
}
