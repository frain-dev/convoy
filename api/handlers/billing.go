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

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/util"
)

type BillingHandler struct {
	*Handler
}

func billingServiceErrorStatus(err error) int {
	if errors.Is(err, billing.ErrNoLicense) {
		return http.StatusUnprocessableEntity
	}

	if serviceErr, ok := errors.AsType[*billing.ServiceError](err); ok {
		if serviceErr.StatusCode == http.StatusUnauthorized {
			// Map billing 401 to 422 so the dashboard does not treat it as Convoy session auth failure / logout.
			return http.StatusUnprocessableEntity
		}
		if serviceErr.StatusCode >= http.StatusBadRequest && serviceErr.StatusCode < http.StatusInternalServerError {
			return serviceErr.StatusCode
		}
		if serviceErr.StatusCode >= http.StatusInternalServerError && serviceErr.StatusCode < 600 {
			return serviceErr.StatusCode
		}
	}

	if strings.Contains(strings.ToLower(err.Error()), "invalid license") {
		return http.StatusUnprocessableEntity
	}

	return http.StatusInternalServerError
}

func (h *BillingHandler) checkBillingAccess(w http.ResponseWriter, r *http.Request, orgID string) bool {
	org, err := h.retrieveOrganisation(r)
	if err != nil || org == nil {
		_ = render.Render(w, r, util.NewErrorResponse("organisation not found", http.StatusNotFound))
		return false
	}

	if org.UID != orgID {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID mismatch", http.StatusForbidden))
		return false
	}
	if h.A.Authz == nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: billing authorizer is unavailable", http.StatusForbidden))
		return false
	}

	if err := h.A.Authz.Authorize(r.Context(), string(policies.PermissionBillingManage), org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: billing access requires billing admin or organisation admin role", http.StatusForbidden))
		return false
	}

	return true
}

// checkBillingCreateAccess enforces billing manage on the active workspace organisation (X-Organisation-Id)
// before creating a record in the billing service.
func (h *BillingHandler) checkBillingCreateAccess(w http.ResponseWriter, r *http.Request) bool {
	orgID := strings.TrimSpace(r.Header.Get("X-Organisation-Id"))
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("X-Organisation-Id is required", http.StatusBadRequest))
		return false
	}

	var (
		org *datastore.Organisation
		err error
	)
	if h.A.OrgRepo != nil {
		org, err = h.A.OrgRepo.FetchOrganisationByID(r.Context(), orgID)
	} else {
		orgRepo := organisations.New(h.A.Logger, h.A.DB)
		org, err = orgRepo.FetchOrganisationByID(r.Context(), orgID)
	}
	if err != nil || org == nil {
		_ = render.Render(w, r, util.NewErrorResponse("organisation not found", http.StatusNotFound))
		return false
	}

	if org.UID != orgID {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID mismatch", http.StatusForbidden))
		return false
	}
	if h.A.Authz == nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: billing authorizer is unavailable", http.StatusForbidden))
		return false
	}

	if err := h.A.Authz.Authorize(r.Context(), string(policies.PermissionBillingManage), org); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized: billing access requires billing admin or organisation admin role", http.StatusForbidden))
		return false
	}

	return true
}

// bindBillingOrganisationExternalID writes 400 and returns false when the body sets external_id
// to a value other than orgID; otherwise sets orgData.ExternalID to orgID and returns true.
func bindBillingOrganisationExternalID(w http.ResponseWriter, r *http.Request, orgID string, orgData *billing.BillingOrganisation) bool {
	orgID = strings.TrimSpace(orgID)
	if ext := strings.TrimSpace(orgData.ExternalID); ext != "" && ext != orgID {
		_ = render.Render(w, r, util.NewErrorResponse("external_id must match the billing organisation id", http.StatusBadRequest))
		return false
	}
	orgData.ExternalID = orgID
	return true
}

// allowBillingCatalogOrgIDQuery reports whether the handler may continue. If it returns false, it has already
// written an error response. When org_id is empty, any caller may load global catalog data. When set, the user
// must be a member of that organisation.
func (h *BillingHandler) allowBillingCatalogOrgIDQuery(w http.ResponseWriter, r *http.Request, orgID string) bool {
	orgID = strings.TrimSpace(orgID)
	if orgID == "" {
		return true
	}

	user, err := h.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusUnauthorized))
		return false
	}

	orgMemberRepo := h.A.OrgMemberRepo
	if orgMemberRepo == nil {
		orgMemberRepo = organisation_members.New(h.A.Logger, h.A.DB)
	}
	if _, err := orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, orgID); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Forbidden", http.StatusForbidden))
		return false
	}

	return true
}

func (h *BillingHandler) GetBillingEnabled(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"enabled":     true,
		"mode":        h.A.Cfg.Mode(),
		"self_hosted": h.A.Cfg.IsSelfHosted(),
	}

	_ = render.Render(w, r, util.NewServerResponse("Billing status retrieved", response, http.StatusOK))
}

func (h *BillingHandler) GetBillingConfig(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"enabled":     true,
		"mode":        h.A.Cfg.Mode(),
		"self_hosted": h.A.Cfg.IsSelfHosted(),
	}
	if h.A.Cfg.IsCloud() {
		response["payment_provider"] = map[string]interface{}{
			"type":            h.A.Cfg.Billing.PaymentProvider.Type,
			"publishable_key": h.A.Cfg.Billing.PaymentProvider.PublishableKey,
		}
	}
	if h.A.Cfg.IsSelfHosted() {
		orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
		if orgID != "" {
			if !h.allowBillingCatalogOrgIDQuery(w, r, orgID) {
				return
			}
			if h.A.Billing != nil {
				response["license"] = h.A.Billing.LicenseSummary(r.Context(), orgID)
			} else {
				response["license"] = billing.LicenseSummary{}
			}
		} else if strings.TrimSpace(r.Header.Get("X-Organisation-Id")) == "" &&
			r.Context().Value(convoy.OrganisationCtx) == nil &&
			r.Context().Value(convoy.ProjectCtx) == nil &&
			chi.URLParam(r, "projectID") == "" {
			response["license"] = billing.LicenseSummary{}
		} else {
			org, err := h.retrieveOrganisationForActiveWorkspace(r)
			if err != nil {
				// Empty summary keeps the dashboard usable, but operators need a breadcrumb so we
				// notice misconfigured org context (missing membership, deleted org) instead of
				// silently returning an empty license payload.
				h.A.Logger.Warnf("GetBillingConfig: unable to resolve organisation for active workspace: %v", err)
				response["license"] = billing.LicenseSummary{}
			} else {
				if h.A.Billing != nil {
					response["license"] = h.A.Billing.LicenseSummary(r.Context(), org.UID)
				} else {
					response["license"] = billing.LicenseSummary{}
				}
			}
		}
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
	if h.A.Billing == nil {
		_ = render.Render(w, r, util.NewErrorResponse("billing service unavailable", http.StatusServiceUnavailable))
		return
	}

	resp, err := h.A.Billing.GetUsage(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.GetInvoices(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.GetSubscription(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}

	h.updateOrganisationStatus(r.Context(), orgID, resp.Data)

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

	resp, err := h.A.Billing.GetPaymentMethods(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.SetDefaultPaymentMethod(r.Context(), orgID, pmID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.DeletePaymentMethod(r.Context(), orgID, pmID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Payment method deleted successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetPlans(w http.ResponseWriter, r *http.Request) {
	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if !h.allowBillingCatalogOrgIDQuery(w, r, orgID) {
		return
	}

	resp, err := h.A.Billing.GetPlans(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Plans retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetTaxIDTypes(w http.ResponseWriter, r *http.Request) {
	orgID := strings.TrimSpace(r.URL.Query().Get("org_id"))
	if !h.allowBillingCatalogOrgIDQuery(w, r, orgID) {
		return
	}
	resp, err := h.A.Billing.GetTaxIDTypes(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Tax ID types retrieved successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
	if !h.A.Cfg.IsCloud() {
		_ = render.Render(w, r, util.NewErrorResponse("billing organisation creation is only available on managed cloud", http.StatusBadRequest))
		return
	}

	var orgData billing.BillingOrganisation
	if err := json.NewDecoder(r.Body).Decode(&orgData); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request body", http.StatusBadRequest))
		return
	}

	if !h.checkBillingCreateAccess(w, r) {
		return
	}

	headerOrgID := strings.TrimSpace(r.Header.Get("X-Organisation-Id"))
	if !bindBillingOrganisationExternalID(w, r, headerOrgID, &orgData) {
		return
	}

	resp, err := h.A.Billing.CreateOrganisation(r.Context(), orgData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.GetOrganisation(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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
	if !bindBillingOrganisationExternalID(w, r, orgID, &orgData) {
		return
	}

	resp, err := h.A.Billing.UpdateOrganisation(r.Context(), orgID, orgData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.UpdateOrganisationTaxID(r.Context(), orgID, taxData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.UpdateOrganisationAddress(r.Context(), orgID, addressData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Address updated successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}
	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.A.Billing.GetSubscriptions(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.OnboardSubscription(r.Context(), orgID, requestData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.UpgradeSubscription(r.Context(), orgID, subscriptionID, requestData)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	resp, err := h.A.Billing.DeleteSubscription(r.Context(), orgID, subscriptionID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}

	h.updateOrganisationStatus(r.Context(), orgID, resp.Data)

	_ = render.Render(w, r, util.NewServerResponse("Subscription cancelled successfully", resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSetupIntent(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}
	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	resp, err := h.A.Billing.GetSetupIntent(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Setup intent retrieved successfully", resp.Data, http.StatusOK))
}

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

	resp, err := h.A.Billing.GetInvoice(r.Context(), orgID, invoiceID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
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

	pdfResp, _, err := h.A.Billing.DownloadInvoice(r.Context(), orgID, invoiceID)
	if err != nil {
		status := billingServiceErrorStatus(err)
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			status = http.StatusNotFound
		}
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), status))
		return
	}
	defer pdfResp.Body.Close()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%s.pdf"`, invoiceID))

	if _, err := io.Copy(w, pdfResp.Body); err != nil {
		h.A.Logger.Error("Failed to stream PDF to client", "error", err)
		return
	}
}

func (h *BillingHandler) GetInternalOrganisationID(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "orgID")
	if orgID == "" {
		_ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
		return
	}
	if !h.checkBillingAccess(w, r, orgID) {
		return
	}

	id, err := h.A.Billing.GetInternalOrganisationID(r.Context(), orgID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), billingServiceErrorStatus(err)))
		return
	}
	_ = render.Render(w, r, util.NewServerResponse("Internal organisation ID retrieved successfully", map[string]interface{}{"id": id}, http.StatusOK))
}

func (h *BillingHandler) updateOrganisationStatus(ctx context.Context, orgID string, subscriptionData interface{}) {
	// disabled_at is currently a cloud-only organisation control.
	if !h.A.Cfg.IsCloud() {
		return
	}

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
		return
	}

	if !org.DisabledAt.Valid {
		org.DisabledAt = null.NewTime(time.Now(), true)
		if err := orgRepo.UpdateOrganisation(ctx, org); err != nil {
			h.A.Logger.Errorf("Failed to set organisation %s disabled_at: %v", orgID, err)
			return
		}
		h.A.Logger.Infof("Set organisation %s disabled_at - subscription not active", orgID)
	}
}
