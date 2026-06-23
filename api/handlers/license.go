package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) GetLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgID")
	if util.IsStringEmpty(orgID) {
		orgID = r.Header.Get(headerOrganisationID)
	}

	if h.A.Cfg.UsesOrgBilling() && h.A.BillingClient != nil && !util.IsStringEmpty(orgID) {
		h.serveOrgLicenseFeatures(w, r, orgID)
		return
	}

	v, err := h.A.Licenser.FeatureListJSON(r.Context())
	if err != nil {
		h.A.Logger.Error("failed to get license features", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("failed to get license features", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
}

// serveOrgLicenseFeatures resolves license features for a specific organisation in cloud
// org-billing mode: it tries the billing service license first, falls back to the
// org's stored license_data only on billing errors, and finally to the billing-required feature list.
func (h *Handler) serveOrgLicenseFeatures(w http.ResponseWriter, r *http.Request, orgID string) {
	org, err := h.orgRepo().FetchOrganisationByID(r.Context(), orgID)
	if err != nil || org == nil {
		h.A.Logger.Warnf("get license features: fetch org failed org_id=%s: %v", orgID, err)
		v, _ := license.BillingRequiredFeatureListJSON()
		_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
		return
	}
	h.A.Logger.Debug("get license features: fetched org", "org_id", orgID, "has_license_data", len(org.LicenseData) > 0, "license_data_len", len(org.LicenseData))

	logReason := func(reason string) {
		h.A.Logger.Debug("get license features", "reason", reason, "org_id", org.UID)
	}

	var billingRequiredReason string
	allowStoredLicenseFallback := false
	billingReturnedNoLicense := false

	projectCount := int64(0)
	if projs, err := h.projectRepo().LoadProjects(r.Context(), &datastore.ProjectFilter{OrgID: org.UID}); err == nil {
		projectCount = int64(len(projs))
	}

	licClient := licensesvc.NewClientFromConfig(h.A.Cfg.LicenseService, h.A.Logger)
	defaultKey := h.A.Cfg.LicenseKey
	useOrgBilling := h.A.Cfg.UsesOrgBilling() && h.A.BillingClient != nil
	deps := services.RefreshLicenseDataDeps{
		OrgMemberRepo: h.orgMemberRepo(),
		OrgRepo:       h.orgRepo(),
		BillingClient: h.A.BillingClient,
		Logger:        h.A.Logger,
		Cfg:           h.A.Cfg,
	}

	// In cloud org billing mode, try billing first for fresh license data.
	if resp, err := h.A.BillingClient.GetOrganisationLicense(r.Context(), org.UID); err == nil && resp != nil && resp.Data.Organisation != nil && resp.Data.Organisation.LicenseKey != "" {
		licenseKey := resp.Data.Organisation.LicenseKey
		data, err := licClient.ValidateLicense(r.Context(), licenseKey)
		if err == nil {
			entitlements, err := data.GetEntitlementsMap()
			if err == nil && len(entitlements) > 0 {
				v, encErr := license.FeatureListFromEntitlementsWithOrgProjectCount(entitlements, projectCount)
				if encErr == nil {
					go services.RefreshLicenseDataForOrg(context.Background(), *org, defaultKey, useOrgBilling, deps, licClient)
					_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
					return
				}
				billingRequiredReason = fmt.Sprintf("FeatureListFromEntitlements (billing key) failed: %v", encErr)
				logReason(billingRequiredReason)
			} else if err != nil {
				billingRequiredReason = fmt.Sprintf("GetEntitlementsMap failed: %v", err)
				logReason(billingRequiredReason)
			}
		} else {
			billingRequiredReason = fmt.Sprintf("ValidateLicense (billing key) failed: %v", err)
			logReason(billingRequiredReason)
		}
	} else {
		if err != nil {
			allowStoredLicenseFallback = true
			if billingRequiredReason == "" {
				billingRequiredReason = fmt.Sprintf("GetOrganisationLicense failed: %v", err)
			}
			logReason(billingRequiredReason)
		} else {
			billingReturnedNoLicense = true
			billingRequiredReason = "no billing license key"
			logReason(billingRequiredReason)
		}
	}

	if billingReturnedNoLicense && org.LicenseData != "" {
		clearCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		org.LicenseData = ""
		if err := h.orgRepo().UpdateOrganisationLicenseData(clearCtx, org.UID, ""); err != nil {
			h.A.Logger.Warn("get license features: clear stale license_data failed", "error", err, "org_id", org.UID)
			billingRequiredReason = fmt.Sprintf("clear stale license_data failed after billing returned no license key: %v", err)
		}
	}

	// Stored license_data is a cache only. When billing answers definitively with
	// no license key, fail closed instead of letting stale entitlements grant access.
	if allowStoredLicenseFallback && org.LicenseData != "" {
		payload, decErr := license.DecryptLicenseData(org.UID, org.LicenseData)
		if decErr == nil && payload != nil && len(payload.Entitlements) > 0 {
			v, encErr := license.FeatureListFromEntitlementsWithOrgProjectCount(payload.Entitlements, projectCount)
			if encErr == nil {
				_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
				return
			}
		}
	}

	if billingRequiredReason == "" {
		billingRequiredReason = "no license data"
	}
	// Trigger refresh on uncertain local/license-service failures so license_data can be repopulated (e.g. after subscription activated).
	h.A.Logger.Info("get license features: returning billing-required, triggering license refresh", "org_id", org.UID)
	if !billingReturnedNoLicense {
		go services.RefreshLicenseDataForOrg(context.Background(), *org, defaultKey, useOrgBilling, deps, licClient)
	}
	v, _ := license.BillingRequiredFeatureListJSON()
	msg := "Retrieved license features successfully"
	if billingRequiredReason != "" {
		msg = msg + "; billing required: " + billingRequiredReason
	}
	_ = render.Render(w, r, util.NewServerResponse(msg, v, http.StatusOK))
}
