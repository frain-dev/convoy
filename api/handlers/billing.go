package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type BillingHandler struct {
	*Handler
	BillingClient *billing.Client
}

func (h *BillingHandler) GetBillingEnabled(w http.ResponseWriter, r *http.Request) {
	response := map[string]bool{
		"enabled": h.A.Cfg.Billing.Enabled,
	}

	_ = render.Render(w, r, util.NewServerResponse("Billing status retrieved", response, http.StatusOK))
}

func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.GetUsage(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Usage retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetInvoices(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
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

	resp, err := h.BillingClient.GetPaymentMethods(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment methods retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	resp, err := h.BillingClient.GetPlans(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Plans retrieved successfully", resp.Data, http.StatusOK))
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

func (h *BillingHandler) UpdateSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	var subData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&subData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateSubscription(r.Context(), orgID, subData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.DeleteSubscription(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Subscription deleted successfully", resp.Data, http.StatusOK))
}

// Payment method handlers
func (h *BillingHandler) GetSetupIntent(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.GetSetupIntent(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Setup intent retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) CreatePaymentMethod(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}

	var pmData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&pmData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.CreatePaymentMethod(r.Context(), orgID, pmData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	// Check if billing service returned an error response
	if !resp.Status {
		_ = render.Render(w, r, util.NewErrorResponse(resp.Message, http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment method created successfully", resp.Data, http.StatusCreated))
}

func (h *BillingHandler) DeletePaymentMethod(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	pmID := chi.URLParam(r, "pmID")
	if orgID == "" || pmID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and payment method ID are required", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.DeletePaymentMethod(r.Context(), orgID, pmID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment method deleted successfully", resp.Data, http.StatusOK))
}

// Invoice handlers
func (h *BillingHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	invoiceID := chi.URLParam(r, "invoiceID")
	if orgID == "" || invoiceID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID and invoice ID are required", http.StatusBadRequest))
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

	resp, err := h.BillingClient.DownloadInvoice(r.Context(), orgID, invoiceID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	// Set response headers for PDF download
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=invoice-%s.pdf", invoiceID))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(resp)))

	// Write PDF content
	w.Write(resp)
}

// Public billing handlers
func (h *BillingHandler) CreateBillingPaymentMethod(w http.ResponseWriter, r *http.Request) {
	var pmData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&pmData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.CreateBillingPaymentMethod(r.Context(), pmData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment method created successfully", resp.Data, http.StatusCreated))
}

func (h *BillingHandler) UpdateBillingAddress(w http.ResponseWriter, r *http.Request) {
	var addressData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&addressData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateBillingAddress(r.Context(), addressData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Address updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) UpdateBillingTaxID(w http.ResponseWriter, r *http.Request) {
	var taxData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&taxData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	resp, err := h.BillingClient.UpdateBillingTaxID(r.Context(), taxData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Tax ID updated successfully", resp.Data, http.StatusOK))
}
