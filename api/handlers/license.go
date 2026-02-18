package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) GetLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgID")
	if util.IsStringEmpty(orgID) {
		orgID = r.Header.Get("X-Organisation-Id")
	}

	if h.A.Cfg.Billing.Enabled && h.A.BillingClient != nil && !util.IsStringEmpty(orgID) {
		var org *datastore.Organisation
		var err error
		if h.A.OrgRepo != nil {
			org, err = h.A.OrgRepo.FetchOrganisationByID(r.Context(), orgID)
		} else {
			orgRepo := organisations.New(h.A.Logger, h.A.DB)
			org, err = orgRepo.FetchOrganisationByID(r.Context(), orgID)
		}
		if err != nil || org == nil {
			log.FromContext(r.Context()).WithError(err).Warnf("get license features: fetch org failed org_id=%s", orgID)
			v, _ := license.BillingRequiredFeatureListJSON()
			_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
			return
		}
		log.FromContext(r.Context()).WithFields(log.Fields{"org_id": orgID, "has_license_data": len(org.LicenseData) > 0, "license_data_len": len(org.LicenseData)}).Debug("get license features: fetched org")

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

		if org.LicenseData != "" {
			payload, decErr := license.DecryptLicenseData(org.UID, org.LicenseData)
			if decErr == nil && payload != nil && len(payload.Entitlements) > 0 {
				v, encErr := license.FeatureListFromEntitlementsWithOrgProjectCount(payload.Entitlements, projectCount)
				if encErr == nil {
					_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
					return
				}
				billingRequiredReason = fmt.Sprintf("FeatureListFromEntitlements failed: %v", encErr)
				log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Debugf("get license features: %s", billingRequiredReason)
			} else if decErr != nil {
				billingRequiredReason = fmt.Sprintf("DecryptLicenseData failed: %v", decErr)
				log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Debugf("get license features: %s", billingRequiredReason)
			} else {
				billingRequiredReason = "stored license_data decrypt returned no usable entitlements (nil payload or empty)"
				log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Debugf("get license features: %s", billingRequiredReason)
			}
		}

		if resp, err := h.A.BillingClient.GetOrganisationLicense(r.Context(), org.UID); err == nil && resp != nil && resp.Data.Key != "" {
			licClient := licensesvc.NewClient(licensesvc.Config{
				Host:         h.A.Cfg.LicenseService.Host,
				ValidatePath: h.A.Cfg.LicenseService.ValidatePath,
				Timeout:      h.A.Cfg.LicenseService.Timeout,
				RetryCount:   h.A.Cfg.LicenseService.RetryCount,
				Logger:       h.A.Logger,
			})
			data, err := licClient.ValidateLicense(r.Context(), resp.Data.Key)
			if err == nil {
				entitlements, err := data.GetEntitlementsMap()
				if err == nil && len(entitlements) > 0 {
					v, encErr := license.FeatureListFromEntitlementsWithOrgProjectCount(entitlements, projectCount)
					if encErr == nil {
						_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
						return
					}
					billingRequiredReason = fmt.Sprintf("FeatureListFromEntitlements (billing key) failed: %v", encErr)
					log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Debugf("get license features: %s", billingRequiredReason)
				} else if err != nil {
					billingRequiredReason = fmt.Sprintf("GetEntitlementsMap failed: %v", err)
					log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Debugf("get license features: %s", billingRequiredReason)
				}
			} else {
				billingRequiredReason = fmt.Sprintf("ValidateLicense (billing key) failed: %v", err)
				log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Debugf("get license features: %s", billingRequiredReason)
			}
		} else {
			if err != nil {
				if billingRequiredReason == "" {
					billingRequiredReason = fmt.Sprintf("GetOrganisationLicense failed: %v", err)
				}
				log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Debugf("get license features: %s", billingRequiredReason)
			} else if (resp == nil || resp.Data.Key == "") && billingRequiredReason == "" {
				billingRequiredReason = "no billing license key"
				log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Debugf("get license features: %s", billingRequiredReason)
			}
		}

		if billingRequiredReason == "" {
			billingRequiredReason = "no license data"
		}
		// Only trigger refresh when org has no license data; otherwise let login handle refresh.
		if org.LicenseData == "" {
			log.FromContext(r.Context()).WithFields(log.Fields{"org_id": org.UID}).Info("get license features: no license data, returning billing-required and triggering refresh")
			orgRepo := h.A.OrgRepo
			if orgRepo == nil {
				orgRepo = organisations.New(h.A.Logger, h.A.DB)
			}
			deps := services.RefreshLicenseDataDeps{
				OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
				OrgRepo:       orgRepo,
				BillingClient: h.A.BillingClient,
				Logger:        h.A.Logger,
				Cfg:           h.A.Cfg,
			}
			defaultKey := h.A.Cfg.LicenseKey
			billingEnabled := h.A.Cfg.Billing.Enabled && h.A.BillingClient != nil
			licClient := licensesvc.NewClient(licensesvc.Config{
				Host:         h.A.Cfg.LicenseService.Host,
				ValidatePath: h.A.Cfg.LicenseService.ValidatePath,
				Timeout:      h.A.Cfg.LicenseService.Timeout,
				RetryCount:   h.A.Cfg.LicenseService.RetryCount,
				Logger:       h.A.Logger,
			})
			if val := r.Context().Value(convoy.AuthUserCtx); val != nil {
				if authUser, ok := val.(*auth.AuthenticatedUser); ok && authUser != nil {
					if user, ok := authUser.Metadata.(*datastore.User); ok && user != nil && user.UID != "" {
						go services.RefreshLicenseDataForUser(user.UID, deps)
					} else {
						go services.RefreshLicenseDataForOrg(context.Background(), *org, defaultKey, billingEnabled, deps, licClient)
					}
				}
			} else {
				go services.RefreshLicenseDataForOrg(context.Background(), *org, defaultKey, billingEnabled, deps, licClient)
			}
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
		log.FromContext(r.Context()).WithError(err).Error("failed to get license features")
		_ = render.Render(w, r, util.NewErrorResponse("failed to get license features", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
}
