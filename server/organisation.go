package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"

	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createOrganisationService(a *ApplicationHandler) *services.OrganisationService {
	orgRepo := mongo.NewOrgRepo(a.A.Store)
	orgMemberRepo := mongo.NewOrgMemberRepo(a.A.Store)

	return services.NewOrganisationService(orgRepo, orgMemberRepo)
}

func (a *ApplicationHandler) GetOrganisation(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("Organisation fetched successfully",
		m.GetOrganisationFromContext(r.Context()), http.StatusOK))
}

// GetOrganisationsPaged - this is a duplicate annotation for the api/v1 route of this handler
// @Summary Get organisations
// @Description This endpoint fetches the organisations a user is part of, this route can only be accessed with a personal api key
// @Tags Organisation
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.Organisation}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/organisations [get]
func _() {}

func (a *ApplicationHandler) GetOrganisationsPaged(w http.ResponseWriter, r *http.Request) { // TODO: change to GetUserOrganisationsPaged
	pageable := m.GetPageableFromContext(r.Context())
	user := m.GetUserFromContext(r.Context())
	orgService := createOrganisationService(a)

	organisations, paginationData, err := orgService.LoadUserOrganisationsPaged(r.Context(), user, pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load organisations")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
		pagedResponse{Content: &organisations, Pagination: &paginationData}, http.StatusOK))
}

func (a *ApplicationHandler) GetUserOrganisations(w http.ResponseWriter, r *http.Request) { // TODO: change to GetUserOrganisationsPaged
	user := m.GetUserFromContext(r.Context())

	orgService := createOrganisationService(a)
	organisations, _, err := orgService.LoadUserOrganisationsPaged(r.Context(), user, datastore.Pageable{Sort: -1})
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load organisations")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
		organisations, http.StatusOK))
}

func (a *ApplicationHandler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
	var newOrg models.Organisation
	err := util.ReadJSON(r, &newOrg)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	user := m.GetUserFromContext(r.Context())
	orgService := createOrganisationService(a)

	organisation, err := orgService.CreateOrganisation(r.Context(), &newOrg, user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation created successfully", organisation, http.StatusCreated))
}

func (a *ApplicationHandler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	var orgUpdate models.Organisation
	err := util.ReadJSON(r, &orgUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
	orgService := createOrganisationService(a)

	org, err := orgService.UpdateOrganisation(r.Context(), m.GetOrganisationFromContext(r.Context()), &orgUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation updated successfully", org, http.StatusAccepted))
}

func (a *ApplicationHandler) DeleteOrganisation(w http.ResponseWriter, r *http.Request) {
	org := m.GetOrganisationFromContext(r.Context())
	orgService := createOrganisationService(a)
	err := orgService.DeleteOrganisation(r.Context(), org.UID)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to delete organisation")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation deleted successfully", nil, http.StatusOK))
}
