package public

import (
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/database/postgres"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (a *PublicHandler) GetOrganisationsPaged(w http.ResponseWriter, r *http.Request) { // TODO: change to GetUserOrganisationsPaged
	pageable := m.GetPageableFromContext(r.Context())
	user, err := a.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	organisations, paginationData, err := postgres.NewOrgMemberRepo(a.A.DB, a.A.Cache).LoadUserOrganisationsPaged(r.Context(), user.UID, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch user organisations")
		_ = render.Render(w, r, util.NewErrorResponse("failed to fetch user organisations", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
		pagedResponse{Content: &organisations, Pagination: &paginationData}, http.StatusOK))
}
