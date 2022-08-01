package server

import (
	"net/http"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

// LoadDevicesPaged
// @Summary Fetch multiple devices
// @Description This endpoint fetches multiple devices
// @Tags Source
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.Device}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /devices [get]
func (a *ApplicationHandler) LoadDevicesPaged(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	group := m.GetGroupFromContext(r.Context())

	devices, paginationData, err := a.S.DeviceService.LoadDevicesPaged(r.Context(), group, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching devices", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Devices fetched successfully", pagedResponse{Content: &devices, Pagination: &paginationData}, http.StatusOK))
}
