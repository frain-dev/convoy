package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/database/postgres"

	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type BillingHandler struct {
	*Handler
	BillingClient billing.Client
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

	// Calculate usage from actual Convoy data instead of external billing service
	usage, err := h.calculateUsageFromConvoy(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Usage retrieved successfully", usage, http.StatusOK))
}

func (h *BillingHandler) calculateUsageFromConvoy(ctx context.Context, orgID string) (map[string]interface{}, error) {
	var totalEvents int64
	var totalDeliveries int64
	var totalIngressBytes int64
	var totalEgressBytes int64
	var err error

	// Calculate current month period
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

	// Ingress (received): events bytes
	var orgRawBytes, orgDataBytes sql.NullInt64
	ingressBytesQuery := `
		SELECT COALESCE(SUM(LENGTH(e.raw)), 0) AS raw_bytes,
		       COALESCE(SUM(OCTET_LENGTH(e.data::text)), 0) AS data_bytes
		FROM convoy.events e
		JOIN convoy.projects p ON p.id = e.project_id
		WHERE p.organisation_id = $1
		  AND e.created_at >= $2 AND e.created_at <= $3
		  AND e.deleted_at IS NULL AND p.deleted_at IS NULL`
	err = h.A.DB.GetReadDB().QueryRowxContext(ctx, ingressBytesQuery, orgID, startOfMonth, endOfMonth).Scan(&orgRawBytes, &orgDataBytes)
	if err == nil {
		totalIngressBytes = orgRawBytes.Int64 + orgDataBytes.Int64
	}

	// Egress (sent): deliveries bytes (count payload per delivery)
	var orgEgressBytes sql.NullInt64
	egressBytesQuery := `
		SELECT COALESCE(SUM(LENGTH(e.raw)), 0) + COALESCE(SUM(OCTET_LENGTH(e.data::text)), 0) AS bytes
		FROM convoy.event_deliveries d
		JOIN convoy.events e ON e.id = d.event_id
		JOIN convoy.projects p ON p.id = e.project_id
		WHERE p.organisation_id = $1
		  AND d.status = 'Success'
		  AND d.created_at >= $2 AND d.created_at <= $3
		  AND p.deleted_at IS NULL`
	err = h.A.DB.GetReadDB().QueryRowxContext(ctx, egressBytesQuery, orgID, startOfMonth, endOfMonth).Scan(&orgEgressBytes)
	if err == nil {
		totalEgressBytes = orgEgressBytes.Int64
	}

	// Org-level total events (received volume)
	eventsQuery := `
		SELECT COUNT(*)
		FROM convoy.events e
		JOIN convoy.projects p ON p.id = e.project_id
		WHERE p.organisation_id = $1
		  AND e.created_at >= $2 AND e.created_at <= $3
		  AND e.deleted_at IS NULL AND p.deleted_at IS NULL`
	_ = h.A.DB.GetReadDB().QueryRowxContext(ctx, eventsQuery, orgID, startOfMonth, endOfMonth).Scan(&totalEvents)

	// Org-level successful deliveries (sent volume)
	deliveriesQuery := `
		SELECT COUNT(*)
		FROM convoy.event_deliveries d
		JOIN convoy.events e ON e.id = d.event_id
		JOIN convoy.projects p ON p.id = e.project_id
		WHERE p.organisation_id = $1
		  AND d.status = 'Success'
		  AND d.created_at >= $2 AND d.created_at <= $3
		  AND p.deleted_at IS NULL`
	_ = h.A.DB.GetReadDB().QueryRowxContext(ctx, deliveriesQuery, orgID, startOfMonth, endOfMonth).Scan(&totalDeliveries)

	// Format period as YYYY-MM
	period := now.Format("2006-01")

	usage := map[string]interface{}{
		"organisation_id": orgID,
		"period":          period,
		"received": map[string]interface{}{
			"volume": totalEvents,
			"bytes":  totalIngressBytes,
		},
		"sent": map[string]interface{}{
			"volume": totalDeliveries,
			"bytes":  totalEgressBytes,
		},
		"created_at": now,
	}

	return usage, nil
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

	_, err := h.BillingClient.GetOrganisation(r.Context(), orgID)
	if err != nil && strings.Contains(err.Error(), "Organisation not found") {
		orgRepo := postgres.NewOrgRepo(h.A.DB)
		org, err := orgRepo.FetchOrganisationByID(r.Context(), orgID)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to fetch organisation data", http.StatusInternalServerError))
			return
		}

		orgData := map[string]interface{}{
			"name":          org.Name,
			"external_id":   orgID,
			"billing_email": "",
			"host":          org.AssignedDomain.String,
		}

		_, createErr := h.BillingClient.CreateOrganisation(r.Context(), orgData)
		if createErr != nil {
			_ = render.Render(w, r, util.NewErrorResponse("failed to create organisation in billing service", http.StatusInternalServerError))
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

	resp, err := h.BillingClient.GetPaymentMethods(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Payment methods retrieved successfully", resp.Data, http.StatusOK))
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
	if _, err := w.Write(resp); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to write response", http.StatusInternalServerError))
		return
	}
}

// GetInternalOrganisationID returns the internal organisation ID from Overwatch
func (h *BillingHandler) GetInternalOrganisationID(w http.ResponseWriter, r *http.Request) {
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
