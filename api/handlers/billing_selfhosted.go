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

	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licenseservice "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/util"
)

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

// serveSelfHosted handles the uniform self-hosted GET pass-throughs: resolve the license
// key, call the billing client, and render the result (503 on error). When successMsg is
// empty the billing response message is used.
func serveSelfHosted[T any](h *BillingHandler, w http.ResponseWriter, r *http.Request, successMsg string, call func(context.Context, string) (*billing.Response[T], error)) {
	licenseKey, ok := h.selfHostedLicenseKey(w, r)
	if !ok {
		return
	}

	resp, err := call(r.Context(), licenseKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusServiceUnavailable))
		return
	}

	msg := successMsg
	if msg == "" {
		msg = resp.Message
	}
	_ = render.Render(w, r, util.NewServerResponse(msg, resp.Data, http.StatusOK))
}

func (h *BillingHandler) GetSelfHostedSubscription(w http.ResponseWriter, r *http.Request) {
	serveSelfHosted(h, w, r, "", h.BillingClient.GetSelfHostedSubscription)
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
	serveSelfHosted(h, w, r, "", h.BillingClient.GetSelfHostedOrganisation)
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
	serveSelfHosted(h, w, r, "", h.BillingClient.GetSelfHostedInvoices)
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

	orgRepo := h.orgRepo()
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
	serveSelfHosted(h, w, r, "", h.BillingClient.GetSelfHostedPaymentMethods)
}

func (h *BillingHandler) GetSelfHostedSetupIntent(w http.ResponseWriter, r *http.Request) {
	serveSelfHosted(h, w, r, "Setup intent retrieved successfully", h.BillingClient.GetSelfHostedSetupIntent)
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
	return h.rebuildLicenser(licenseKey)
}

func (h *BillingHandler) useBillingRequiredLicenser() error {
	return h.rebuildLicenser("")
}

// rebuildLicenser re-initialises h.A.Licenser. A non-empty licenseKey rebuilds the
// instance licenser around that key (cloud org-billing mode is derived from config) and
// persists it via config.Override; an empty licenseKey installs the billing-required
// licenser (org billing on, no key/client) without persisting config.
func (h *BillingHandler) rebuildLicenser(licenseKey string) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	lc := licenseservice.LicenserConfig{
		OrgRepo:     h.orgRepo(),
		UserRepo:    users.New(h.A.Logger, h.A.DB),
		ProjectRepo: h.projectRepo(),
		Logger:      h.A.Logger,
	}

	persist := false
	if licenseKey != "" {
		cfg.LicenseKey = licenseKey
		lc.LicenseKey = cfg.LicenseKey
		lc.UseOrgBilling = cfg.UsesOrgBilling()
		lc.Client = licenseservice.NewClientFromConfig(cfg.LicenseService, h.A.Logger)
		persist = true
	} else {
		lc.UseOrgBilling = true
	}

	licenser, err := license.NewLicenser(&license.Config{LicenseService: lc})
	if err != nil {
		return err
	}

	h.A.Licenser = licenser
	h.A.Cfg = cfg
	if persist {
		return config.Override(&cfg)
	}
	return nil
}
