package dashboard

import (
	"net/http"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createDeviceService(a *DashboardHandler) *services.DeviceService {
	deviceRepo := postgres.NewDeviceRepo(a.A.DB)

	return services.NewDeviceService(deviceRepo)
}

func (a *DashboardHandler) FindDevicesByAppID(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())
	endpoint := m.GetEndpointFromContext(r.Context())

	f := &datastore.ApiKeyFilter{
		EndpointID: endpoint.UID,
	}

	deviceService := createDeviceService(a)
	devices, paginationData, err := deviceService.LoadDevicesPaged(r.Context(), project, f, pageable)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching devices", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Devices fetched successfully", pagedResponse{Content: &devices, Pagination: &paginationData}, http.StatusOK))
}
