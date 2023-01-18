package socket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ListenRequest struct {
	HostName   string   `json:"host_name"`
	DeviceID   string   `json:"device_id"`
	SourceID   string   `json:"source_id"`
	EventTypes []string `json:"event_types"`
}

type LoginRequest struct {
	HostName string `json:"host_name"`
	DeviceID string `json:"device_id"`
}

type LoginResponse struct {
	Device   *datastore.Device   `json:"device"`
	Project  *datastore.Project  `json:"project"`
	Endpoint *datastore.Endpoint `json:"endpoint"`
}

type Repo struct {
	DeviceRepo        datastore.DeviceRepository
	SourceRepo        datastore.SourceRepository
	EndpointRepo      datastore.EndpointRepository
	SubscriptionRepo  datastore.SubscriptionRepository
	EventDeliveryRepo datastore.EventDeliveryRepository
}

var ug = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func BuildRoutes(h *Hub, r *Repo, m *m.Middleware) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)

	router.Route("/stream", func(streamRouter chi.Router) {
		streamRouter.Use(
			m.RequireAuth(),
			m.RequireProject(),
			m.RequireEndpointID(),
			// m.RequireAppPortalApplication(),
		)

		streamRouter.Get("/listen", ListenHandler(h, r))
		streamRouter.Post("/login", LoginHandler(h, r))
	})

	return router
}

func ListenHandler(hub *Hub, repo *Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listenRequest := &ListenRequest{}
		err := json.Unmarshal([]byte(r.Header.Get("Body")), &listenRequest)
		if err != nil {
			log.WithError(err).Error("failed to marshal data")
			respond(w, http.StatusBadRequest, "failed to marshal response: "+err.Error())
			return
		}

		project := m.GetProjectFromContext(r.Context())
		app := m.GetEndpointFromContext(r.Context())

		device, err := listen(r.Context(), project, app, listenRequest, hub, repo)
		if err != nil {
			respond(w, err.(*util.ServiceError).ErrCode(), err.Error())
			return
		}

		conn, err := ug.Upgrade(w, r, nil)
		if err != nil {
			log.WithError(err).Error("failed to upgrade connection to websocket connection")
			respond(w, http.StatusBadRequest, "failed to upgrade connection to websocket connection: "+err.Error())
			return
		}

		NewClient(conn, device, listenRequest.EventTypes, repo.DeviceRepo, repo.EventDeliveryRepo)
	}
}

func LoginHandler(hub *Hub, repo *Repo) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loginRequest := &LoginRequest{}
		err := util.ReadJSON(r, &loginRequest)
		if err != nil {
			respond(w, http.StatusBadRequest, err.Error())
			return
		}

		project := m.GetProjectFromContext(r.Context())
		endpoint := m.GetEndpointFromContext(r.Context())

		device, err := login(r.Context(), project, endpoint, loginRequest, hub, repo)
		if err != nil {
			respond(w, err.(*util.ServiceError).ErrCode(), err.Error())
			return
		}

		lr := &LoginResponse{Device: device, Project: project, Endpoint: endpoint}

		respondWithData(w, http.StatusOK, lr)
	})
}

func login(ctx context.Context, project *datastore.Project, endpoint *datastore.Endpoint, loginRequest *LoginRequest, h *Hub, repo *Repo) (*datastore.Device, error) {
	endpointID := ""
	if endpoint != nil {
		endpointID = endpoint.UID
	}

	var device *datastore.Device
	var err error
	if !util.IsStringEmpty(loginRequest.DeviceID) {
		device, err = repo.DeviceRepo.FetchDeviceByID(ctx, loginRequest.DeviceID, endpointID, project.UID)
		if err != nil {
			log.WithError(err).Error("failed to find device by id")
			return nil, util.NewServiceError(http.StatusBadRequest, err)
		}

		if device.ProjectID != project.UID {
			return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("this device cannot access this project"))
		}

		if device.EndpointID != endpointID {
			return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("this device cannot access this application"))
		}

		// we set the device to offline because it was in an inconsistent state on the server.
		// the device should only be set to online when we start listening for events
		if device.Status == datastore.DeviceStatusOnline {
			device.Status = datastore.DeviceStatusOffline
			err = repo.DeviceRepo.UpdateDevice(ctx, device, device.EndpointID, device.ProjectID)
			if err != nil {
				return nil, util.NewServiceError(http.StatusBadRequest, err)
			}
		}
	} else {
		device, err = repo.DeviceRepo.FetchDeviceByHostName(ctx, loginRequest.HostName, endpointID, project.UID)
		if err != nil {
			log.WithError(err).Error("failed to find device for this hostname, will create new device")
		}

		if device != nil {
			d := &datastore.Device{
				EndpointID: endpointID,
				ProjectID:  project.UID,
				HostName:   loginRequest.HostName,
			}

			err = repo.DeviceRepo.UpdateDevice(ctx, d, endpointID, project.UID)
			if err != nil {
				log.WithError(err).Error("failed to update device")
				return nil, util.NewServiceError(http.StatusBadRequest, err)
			}

			device.HostName = d.HostName
			device.ProjectID = d.ProjectID
			device.EndpointID = d.EndpointID

		} else {
			device = &datastore.Device{
				EndpointID: endpointID,
				ProjectID:  project.UID,
				UID:        uuid.NewString(),
				HostName:   loginRequest.HostName,
				Status:     datastore.DeviceStatusOffline,
				LastSeenAt: primitive.NewDateTimeFromTime(time.Now()),
				CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:  primitive.NewDateTimeFromTime(time.Now()),
			}

			err = repo.DeviceRepo.CreateDevice(ctx, device)
			if err != nil {
				log.WithError(err).Error("failed to create new device")
				return nil, util.NewServiceError(http.StatusBadRequest, err)
			}
		}
	}

	return device, nil
}

func listen(ctx context.Context, project *datastore.Project, endpoint *datastore.Endpoint, listenRequest *ListenRequest, h *Hub, r *Repo) (*datastore.Device, error) {
	endpointID := ""
	if endpoint != nil {
		endpointID = endpoint.UID
	}

	device, err := r.DeviceRepo.FetchDeviceByID(ctx, listenRequest.DeviceID, endpointID, project.UID)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if device.ProjectID != project.UID {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("this device cannot access this project"))
	}

	if device.EndpointID != endpointID {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("this device cannot access this application"))
	}

	if project.Type == datastore.IncomingProject && util.IsStringEmpty(listenRequest.SourceID) {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("the source is required for incoming projects"))
	}

	if project.Type == datastore.OutgoingProject && !util.IsStringEmpty(listenRequest.SourceID) {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("the source should not be passed for outgoing projects"))
	}

	if project.Type == datastore.IncomingProject && !util.IsStringEmpty(listenRequest.SourceID) {
		source, err := r.SourceRepo.FindSourceByID(ctx, device.ProjectID, listenRequest.SourceID)
		if err != nil {
			log.WithError(err).Error("error retrieving source")
			return nil, util.NewServiceError(http.StatusBadRequest, err)
		}

		if source.ProjectID != project.UID {
			return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("this device cannot access this source"))
		}
	}

	sub, err := r.SubscriptionRepo.FindSubscriptionByDeviceID(ctx, project.UID, device.UID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			s := &datastore.Subscription{
				UID:          uuid.NewString(),
				Name:         fmt.Sprintf("%s-subscription", device.HostName),
				Type:         datastore.SubscriptionTypeCLI,
				EndpointID:   endpointID,
				ProjectID:    project.UID,
				SourceID:     listenRequest.SourceID,
				DeviceID:     device.UID,
				FilterConfig: &datastore.FilterConfiguration{EventTypes: []string{"*"}},
				CreatedAt:    primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:    primitive.NewDateTimeFromTime(time.Now()),
			}

			err = r.SubscriptionRepo.CreateSubscription(ctx, project.UID, s)
			if err != nil {
				return nil, util.NewServiceError(http.StatusBadRequest, err)
			}

			return device, nil
		}

		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	sub.SourceID = listenRequest.SourceID
	sub.FilterConfig = &datastore.FilterConfiguration{EventTypes: listenRequest.EventTypes}
	sub.AlertConfig = &datastore.DefaultAlertConfig
	sub.RetryConfig = &datastore.DefaultRetryConfig
	err = r.SubscriptionRepo.UpdateSubscription(ctx, project.UID, sub)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	return device, nil
}

func respond(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err := w.Write([]byte(msg))
	if err != nil {
		log.WithError(err).Error("failed to write response message")
	}
}

func respondWithData(w http.ResponseWriter, code int, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.WithError(err).Error("failed to marshal data")
		respond(w, http.StatusInternalServerError, "failed to marshal response: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, err = w.Write(data)
	if err != nil {
		log.WithError(err).Error("failed to write response data")
	}
}
