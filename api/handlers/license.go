package handlers

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
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
			v, _ := license.BillingRequiredFeatureListJSON()
			_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
			return
		}

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
				}
			}
		}

		v, _ := license.BillingRequiredFeatureListJSON()
		_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
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
