package dashboard

import (
	"errors"
	"net/http"

	base "github.com/frain-dev/convoy/api/dashboard"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
)

type DashboardHandler struct {
	*base.DashboardHandler
	Opts *types.APIOptions
}

func (dh *DashboardHandler) retrieveHost() (string, error) {
	cfg, err := config.Get()
	if err != nil {
		return "", err
	}

	return cfg.Host, nil
}

func (dh *DashboardHandler) retrieveUser(r *http.Request) (*datastore.User, error) {
	authUser := middleware.GetAuthUserFromContext(r.Context())
	user, ok := authUser.Metadata.(*datastore.User)
	if !ok {
		return &datastore.User{}, errors.New("User not found")
	}

	return user, nil
}

func (a *DashboardHandler) retrieveOrganisation(r *http.Request) (*datastore.Organisation, error) {
	orgID := chi.URLParam(r, "orgID")

	if util.IsStringEmpty(orgID) {
		orgID = r.URL.Query().Get("orgID")
	}

	orgRepo := postgres.NewOrgRepo(a.A.DB)
	return orgRepo.FetchOrganisationByID(r.Context(), orgID)
}
