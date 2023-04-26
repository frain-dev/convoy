package public

import (
	"net/http"

	"github.com/frain-dev/convoy/database/postgres"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createOrganisationService(a *PublicHandler) *services.OrganisationService {
	orgRepo := postgres.NewOrgRepo(a.A.DB)
	orgMemberRepo := postgres.NewOrgMemberRepo(a.A.DB)

	return services.NewOrganisationService(orgRepo, orgMemberRepo)
}

func (a *PublicHandler) GetOrganisationsPaged(w http.ResponseWriter, r *http.Request) { // TODO: change to GetUserOrganisationsPaged
	pageable := m.GetPageableFromContext(r.Context())
	user, err := a.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
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
