package handlers

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

func (h *Handler) GetLicenseFeatures(w http.ResponseWriter, r *http.Request) {
	if h.A.Cfg.Billing.Enabled {
		orgID := r.URL.Query().Get("orgID")
		if util.IsStringEmpty(orgID) {
			orgID = r.Header.Get("X-Organisation-Id")
		}
		if !util.IsStringEmpty(orgID) {
			var org *datastore.Organisation
			var err error
			if h.A.OrgRepo != nil {
				org, err = h.A.OrgRepo.FetchOrganisationByID(r.Context(), orgID)
			} else {
				orgRepo := organisations.New(h.A.Logger, h.A.DB)
				org, err = orgRepo.FetchOrganisationByID(r.Context(), orgID)
			}
			if err == nil && org != nil && org.LicenseData != "" {
				payload, decErr := license.DecryptLicenseData(org.UID, org.LicenseData)
				if decErr == nil && payload != nil && len(payload.Entitlements) > 0 {
					v, encErr := license.FeatureListFromEntitlements(payload.Entitlements)
					if encErr == nil {
						_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
						return
					}
				}
			}
		}
	}

	v, err := h.A.Licenser.FeatureListJSON(r.Context())
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to get license features")
		_ = render.Render(w, r, util.NewErrorResponse("failed to get license features", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Retrieved license features successfully", v, http.StatusOK))
}
