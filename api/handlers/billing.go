package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
)

var ErrHostRequiredForBilling = errors.New("organisation host (assigned domain) is required for billing. Please set the assigned domain in the configuration")

type BillingHandler struct {
	*Handler
	BillingClient billing.Client
}

func (h *BillingHandler) checkBillingAccess(w http.ResponseWriter, r *http.Request, orgID string) bool {
	if !h.A.Licenser.BillingModule() {
		_ = render.Render(w, r, util.NewErrorResponse("Billing module is not available in your license plan", http.StatusForbidden))
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

func (h *BillingHandler) GetBillingEnabled(w http.ResponseWriter, r *http.Request) {
	response := map[string]bool{
		"enabled": h.A.Cfg.Billing.Enabled && h.A.Licenser.BillingModule(),
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
	orgRepo := postgres.NewOrgRepo(h.A.DB)
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
		orgRepo := postgres.NewOrgRepo(h.A.DB)
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

		orgData := map[string]interface{}{
			"name":          org.Name,
			"external_id":   orgID,
			"billing_email": "",
			"host":          cfg.Host,
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
	// Serve plans from configuration if available, otherwise return empty array
	var plans []interface{}
	if len(h.A.Cfg.Billing.Plans) > 0 {
		plans = h.A.Cfg.Billing.Plans
	} else {
		plans = []interface{}{}
	}

	_ = render.Render(w, r, util.NewServerResponse("Plans retrieved successfully", plans, http.StatusOK))
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
	var orgData map[string]interface{}
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

	var orgData map[string]interface{}
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

	var taxData map[string]interface{}
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

	var addressData map[string]interface{}
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

func (h *BillingHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	var subData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&subData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.CreateSubscription(r.Context(), orgID, subData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription created successfully", resp.Data, http.StatusCreated))
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
		orgRepo := postgres.NewOrgRepo(h.A.DB)
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

		orgData := map[string]interface{}{
			"name":          org.Name,
			"external_id":   orgID,
			"billing_email": "",
			"host":          cfg.Host,
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

	// Extract just the internal ID from the response
	var internalID string
	if resp.Data != nil {
		if data, ok := resp.Data.(map[string]interface{}); ok {
			if id, exists := data["id"]; exists {
				if idStr, ok := id.(string); ok {
					internalID = idStr
				}
			}
		}
	}

	responseData := map[string]interface{}{
		"id": internalID,
	}

	_ = render.Render(w, r, util.NewServerResponse("Internal organisation ID retrieved successfully", responseData, http.StatusOK))
}
