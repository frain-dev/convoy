package server

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createDeviceService(a *ApplicationHandler) *services.DeviceService {
	deviceRepo := mongo.NewDeviceRepository(a.A.Store)

	return services.NewDeviceService(deviceRepo)
}

// FindDevicesByAppID
// @Summary Fetch devices for an app
// @Description This endpoint fetches devices for an app
// @Tags Source
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Param appID path string true "app id"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.Device}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /devices/{appID} [get]
func (a *ApplicationHandler) FindDevicesByAppID(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	group := m.GetGroupFromContext(r.Context())
	endpoint := m.GetEndpointFromContext(r.Context())

	f := &datastore.ApiKeyFilter{
		EndpointID: endpoint.UID,
	}

	deviceService := createDeviceService(a)
	devices, paginationData, err := deviceService.LoadDevicesPaged(r.Context(), group, f, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching devices", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Devices fetched successfully", pagedResponse{Content: &devices, Pagination: &paginationData}, http.StatusOK))
}
