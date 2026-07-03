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

	// Org-scoped features require membership; guests get instance-level features.
	if h.A.Cfg.UsesOrgBilling() && h.A.BillingClient != nil && !util.IsStringEmpty(orgID) && h.isOrgMember(r, orgID) {
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

func (h *Handler) isOrgMember(r *http.Request, orgID string) bool {
	user, err := h.retrieveUser(r)
	if err != nil || user == nil || user.UID == "" {
		return false
	}

	member, err := h.orgMemberRepo().FetchOrganisationMemberByUserID(r.Context(), user.UID, orgID)
	return err == nil && member != nil
}

func (h *Handler) GetPortalLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	if h.A.Cfg.UsesOrgBilling() && h.A.BillingClient != nil {
		project, err := h.retrieveProject(r)
		if err != nil {
			h.A.Logger.Error("portal license features: failed to resolve project from token", "error", err)
			_ = render.Render(w, r, util.NewErrorResponse("failed to get license features", http.StatusBadRequest))
			return
		}

		h.serveOrgLicenseFeatures(w, r, project.OrganisationID)
		return
	}

	// Self-hosted / non-org-billing: use the deployment licenser.
	v, err := h.A.Licenser.FeatureListJSON(r.Context())
	if err != nil {
		h.A.Logger.Error("failed to get license features", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("failed to get license features", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
}

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

	memberCount := int64(-1)
	if c, err := h.orgMemberRepo().CountOrganisationMembers(r.Context(), org.UID); err == nil {
		memberCount = c
	}

	orgCount := int64(-1)
	if user, err := h.retrieveUser(r); err == nil && user != nil && user.UID != "" {
		if c, err := h.orgMemberRepo().CountUserOrganisations(r.Context(), user.UID, ""); err == nil {
			orgCount = c
		}
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
		Cache:         h.A.Cache,
	}

	// Try billing first for fresh license data.
	if resp, err := h.A.BillingClient.GetOrganisationLicense(r.Context(), org.UID); err == nil && resp != nil && resp.Data.Organisation != nil && resp.Data.Organisation.LicenseKey != "" {
		licenseKey := resp.Data.Organisation.LicenseKey
		data, err := licClient.ValidateLicense(r.Context(), licenseKey)
		if err == nil {
			entitlements, err := data.GetEntitlementsMap()
			if err == nil && len(entitlements) > 0 {
				v, encErr := license.FeatureListFromEntitlementsWithUsage(entitlements, orgCount, memberCount, projectCount)
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
		if err := services.ClearOrgLicenseData(clearCtx, deps, org.UID); err != nil {
			h.A.Logger.Warn("get license features: clear stale license_data failed", "error", err, "org_id", org.UID)
			billingRequiredReason = fmt.Sprintf("clear stale license_data failed after billing returned no license key: %v", err)
		}

		if license.IsProvisional(org.UID, org.LicenseData) {
			payload, decErr := license.DecryptLicenseData(org.UID, org.LicenseData)
			if decErr == nil && payload != nil && len(payload.Entitlements) > 0 {
				v, encErr := license.FeatureListFromEntitlementsWithUsage(payload.Entitlements, orgCount, memberCount, projectCount)
				if encErr == nil {
					_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
					return
				}
				logReason(fmt.Sprintf("FeatureListFromEntitlements (provisional seed) failed: %v", encErr))
			}
		}
	}

	if allowStoredLicenseFallback && org.LicenseData != "" {
		payload, decErr := license.DecryptLicenseData(org.UID, org.LicenseData)
		if decErr == nil && payload != nil && len(payload.Entitlements) > 0 {
			v, encErr := license.FeatureListFromEntitlementsWithUsage(payload.Entitlements, orgCount, memberCount, projectCount)
			if encErr == nil {
				_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
				return
			}
		}
	}

	if billingRequiredReason == "" {
		billingRequiredReason = "no license data"
	}
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
