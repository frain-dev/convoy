package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) GetLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	orgID := strings.TrimSpace(r.Header.Get("X-Organisation-Id"))
	if util.IsStringEmpty(orgID) {
		orgID = strings.TrimSpace(r.URL.Query().Get("orgID"))
	}
	if util.IsStringEmpty(orgID) {
		orgID = strings.TrimSpace(r.URL.Query().Get("organisation_id"))
	}

	if h.shouldUseOrgScopedLicenseFeatures(orgID) {
		var org *datastore.Organisation
		var err error
		if h.A.OrgRepo != nil {
			org, err = h.A.OrgRepo.FetchOrganisationByID(r.Context(), orgID)
		} else {
			orgRepo := organisations.New(h.A.Logger, h.A.DB)
			org, err = orgRepo.FetchOrganisationByID(r.Context(), orgID)
		}
		if err != nil || org == nil {
			h.A.Logger.Warnf("get license features: fetch org failed org_id=%s: %v", orgID, err)
			v, _ := license.BillingRequiredFeatureListJSON()
			_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
			return
		}

		user, err := h.retrieveUser(r)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusUnauthorized))
			return
		}
		orgMemberRepo := h.A.OrgMemberRepo
		if orgMemberRepo == nil {
			orgMemberRepo = organisation_members.New(h.A.Logger, h.A.DB)
		}
		if _, err := orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, org.UID); err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("Forbidden", http.StatusForbidden))
			return
		}

		h.A.Logger.Debug("get license features: fetched org", "org_id", orgID, "has_license_data", len(org.LicenseData) > 0, "license_data_len", len(org.LicenseData))

		var billingRequiredReason string

		projectCount := int64(0)
		if h.A.ProjectRepo != nil {
			projs, err := h.A.ProjectRepo.LoadProjects(r.Context(), &datastore.ProjectFilter{OrgID: org.UID})
			if err == nil {
				projectCount = int64(len(projs))
			}
		} else {
			projectRepo := projects.New(h.A.Logger, h.A.DB)
			if projs, err := projectRepo.LoadProjects(r.Context(), &datastore.ProjectFilter{OrgID: org.UID}); err == nil {
				projectCount = int64(len(projs))
			}
		}

		licClient := licensesvc.NewClient(licensesvc.Config{
			Host:         h.A.Cfg.LicenseService.Host,
			ValidatePath: h.A.Cfg.LicenseService.ValidatePath,
			Timeout:      h.A.Cfg.LicenseService.Timeout,
			RetryCount:   h.A.Cfg.LicenseService.RetryCount,
			Logger:       h.A.Logger,
		})
		orgRepo := h.A.OrgRepo
		if orgRepo == nil {
			orgRepo = organisations.New(h.A.Logger, h.A.DB)
		}
		defaultKey := h.A.Cfg.LicenseKey
		deps := services.RefreshLicenseDataDeps{
			OrgMemberRepo: orgMemberRepo,
			OrgRepo:       orgRepo,
			BillingClient: h.A.BillingClient,
			OrgBilling:    h.A.Billing,
			Logger:        h.A.Logger,
			Cfg:           h.A.Cfg,
		}

		if h.A.Cfg.IsCloud() {
			if resp, err := h.A.BillingClient.GetOrganisationLicense(r.Context(), org.UID); err == nil && resp != nil && resp.Data.Organisation != nil && resp.Data.Organisation.LicenseKey != "" {
				licenseKey := resp.Data.Organisation.LicenseKey
				data, err := licClient.ValidateLicense(r.Context(), licenseKey)
				if err == nil {
					entitlements, err := data.GetEntitlementsMap()
					if err == nil && len(entitlements) > 0 {
						v, encErr := license.FeatureListFromEntitlementsWithOrgProjectCount(entitlements, projectCount)
						if encErr == nil {
							go services.RefreshLicenseDataForOrg(context.Background(), *org, defaultKey, deps, licClient)
							_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
							return
						}
						billingRequiredReason = fmt.Sprintf("FeatureListFromEntitlements (billing key) failed: %v", encErr)
						h.A.Logger.Debug(fmt.Sprintf("get license features: %s", billingRequiredReason), "org_id", org.UID)
					} else if err != nil {
						billingRequiredReason = fmt.Sprintf("GetEntitlementsMap failed: %v", err)
						h.A.Logger.Debug(fmt.Sprintf("get license features: %s", billingRequiredReason), "org_id", org.UID)
					}
				} else {
					billingRequiredReason = fmt.Sprintf("ValidateLicense (billing key) failed: %v", err)
					h.A.Logger.Debug(fmt.Sprintf("get license features: %s", billingRequiredReason), "org_id", org.UID)
				}
			} else {
				if err != nil {
					if billingRequiredReason == "" {
						billingRequiredReason = fmt.Sprintf("GetOrganisationLicense failed: %v", err)
					}
					h.A.Logger.Debug(fmt.Sprintf("get license features: %s", billingRequiredReason), "org_id", org.UID)
				} else {
					billingRequiredReason = "no billing license key"
					h.A.Logger.Debug(fmt.Sprintf("get license features: %s", billingRequiredReason), "org_id", org.UID)
				}
			}
		}

		skipCachedLicenseData := false
		if !h.A.Cfg.IsCloud() && h.A.Billing != nil {
			if subResp, subErr := h.A.Billing.GetSubscription(r.Context(), org.UID); subErr == nil && subResp != nil && subResp.Status {
				if !billing.HasActiveSubscription(subResp.Data) {
					if billingRequiredReason == "" {
						billingRequiredReason = "no active subscription"
					}
					skipCachedLicenseData = true
					h.A.Logger.Debug("get license features: self-hosted subscription inactive; skipping cached license data", "org_id", org.UID)
				}
			}
		}

		if !skipCachedLicenseData && org.LicenseData != "" {
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
		if skipCachedLicenseData {
			if org.LicenseData != "" {
				if err := orgRepo.UpdateOrganisationLicenseData(r.Context(), org.UID, ""); err != nil {
					h.A.Logger.Warn("get license features: clear license_data failed", "error", err, "org_id", org.UID)
				}
			}
			h.A.Logger.Info("get license features: returning billing-required (inactive subscription); license_data cleared", "org_id", org.UID)
		} else {
			h.A.Logger.Info("get license features: returning billing-required, triggering license refresh", "org_id", org.UID)
			go services.RefreshLicenseDataForOrg(context.Background(), *org, defaultKey, deps, licClient)
		}
		v, _ := license.BillingRequiredFeatureListJSON()
		msg := "Retrieved license features successfully"
		if billingRequiredReason != "" {
			msg = msg + "; billing required: " + billingRequiredReason
		}
		_ = render.Render(w, r, util.NewServerResponse(msg, v, http.StatusOK))
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

func (h *Handler) shouldUseOrgScopedLicenseFeatures(orgID string) bool {
	if util.IsStringEmpty(orgID) {
		return false
	}
	if h.A.Cfg.IsCloud() {
		return h.A.BillingClient != nil
	}

	return !util.IsStringEmpty(h.A.Cfg.LicenseKey)
}
