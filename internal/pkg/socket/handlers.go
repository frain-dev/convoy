package socket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/queue"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type ListenRequest struct {
	HostName   string `json:"host_name" valid:"required~please provide a hostname"`
	ProjectID  string `json:"project_id" valid:"required~please provide a project id"`
	DeviceID   string `json:"device_id" valid:"required~please provide a device id"`
	SourceID   string `json:"-"`
	SourceName string `json:"source_name"`
	// EventTypes []string `json:"event_types"`
}

type LoginRequest struct {
	HostName string `json:"host_name" valid:"required~please provide a hostname"`
}

type LoginResponse struct {
	Projects []ProjectDevice `json:"projects"`
	UserName string          `json:"user_name"`
}

type ProjectDevice struct {
	Project *datastore.Project `json:"project"`
	Device  *datastore.Device  `json:"device"`
}

type Repo struct {
	OrgMemberRepository datastore.OrganisationMemberRepository
	ProjectRepo         datastore.ProjectRepository
	DeviceRepo          datastore.DeviceRepository
	SourceRepo          datastore.SourceRepository
	EndpointRepo        datastore.EndpointRepository
	SubscriptionRepo    datastore.SubscriptionRepository
	EventDeliveryRepo   datastore.EventDeliveryRepository
	Queue               queue.Queuer
}

var ug = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func BuildRoutes(r *Repo) http.Handler {
	router := chi.NewRouter()
	router.Use(chiMiddleware.Recoverer)

	router.Route("/stream", func(streamRouter chi.Router) {
		streamRouter.Use(
			middleware.RequireAuth(),
			middleware.InstrumentRequests(),
			middleware.RequirePersonalAccessToken(),
		)

		// TODO(subomi): Add authz
		streamRouter.Get("/listen", ListenHandler(r))
		streamRouter.Post("/login", LoginHandler(r))
	})

	return router
}

func ListenHandler(repo *Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listenRequest := &ListenRequest{}
		err := json.Unmarshal([]byte(r.Header.Get("Body")), &listenRequest)
		if err != nil {
			log.WithError(err).Error("failed to marshal data")
			respond(w, http.StatusBadRequest, fmt.Sprintf("failed to marshal response: %v", err.Error()))
			return
		}

		err = util.Validate(listenRequest)
		if err != nil {
			respond(w, http.StatusBadRequest, err.Error())
			return
		}

		device, err := listen(r.Context(), listenRequest, repo)
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

		fmt.Printf("Listener connected for device %s with hostname %s\n", device.UID, device.HostName)
		NewClient(context.Background(), conn, device, listenRequest.SourceID, repo.DeviceRepo, repo.EventDeliveryRepo)
	}
}

func LoginHandler(repo *Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loginRequest := &LoginRequest{}
		err := util.ReadJSON(r, loginRequest)
		if err != nil {
			respond(w, http.StatusBadRequest, err.Error())
			return
		}

		err = util.Validate(loginRequest)
		if err != nil {
			respond(w, http.StatusBadRequest, err.Error())
			return
		}

		authUser := middleware.GetAuthUserFromContext(r.Context())

		lr, err := login(r.Context(), loginRequest, repo, authUser.User.(*datastore.User))
		if err != nil {
			respond(w, err.(*util.ServiceError).ErrCode(), err.Error())
			return
		}

		respondWithData(w, lr)
	}
}

func login(ctx context.Context, loginRequest *LoginRequest, repo *Repo, user *datastore.User) (*LoginResponse, error) {
	projects, err := repo.OrgMemberRepository.FindUserProjects(ctx, user.UID)
	if err != nil {
		log.WithError(err).Error("failed to find user projects")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find user projects"))
	}

	loginResponse := &LoginResponse{
		UserName: fmt.Sprintf("%s %s", user.FirstName, user.LastName),
	}
	loginResponse.Projects = make([]ProjectDevice, 0, len(projects))

	for i := range projects {
		project := &projects[i]
		if project.Type != datastore.IncomingProject {
			continue
		}

		var device *datastore.Device
		device, err = repo.DeviceRepo.FetchDeviceByHostName(ctx, loginRequest.HostName, "", project.UID)
		if err != nil {
			log.WithError(err).Error("failed to find device for this hostname, will create new device")

			device = &datastore.Device{
				UID:        ulid.Make().String(),
				ProjectID:  project.UID,
				HostName:   loginRequest.HostName,
				Status:     datastore.DeviceStatusOffline,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
				LastSeenAt: time.Now(),
			}

			err = repo.DeviceRepo.CreateDevice(ctx, device)
			if err != nil {
				log.WithError(err).Error("failed to create new device")
				return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create new device"))
			}
		}

		loginResponse.Projects = append(loginResponse.Projects, ProjectDevice{
			Project: project,
			Device:  device,
		})
	}

	return loginResponse, nil
}

func listen(ctx context.Context, listenRequest *ListenRequest, r *Repo) (*datastore.Device, error) {
	project, err := r.ProjectRepo.FetchProjectByID(ctx, listenRequest.ProjectID)
	if err != nil {
		log.WithError(err).Error("failed to find project")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find project"))
	}

	if project.Type == datastore.OutgoingProject {
		return nil, util.NewServiceError(http.StatusUnauthorized, errors.New("cli streaming is not available for outgoing projects"))
	}

	device, err := r.DeviceRepo.FetchDeviceByID(ctx, listenRequest.DeviceID, "", project.UID)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if device.Status != datastore.DeviceStatusOnline {
		device.Status = datastore.DeviceStatusOnline
		if err != nil {
			log.WithError(err).Error("failed to update device to online")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update device to online"))
		}
	}

	if !util.IsStringEmpty(listenRequest.SourceName) {
		trimmedName := strings.TrimSpace(listenRequest.SourceName)
		source, err := r.SourceRepo.FindSourceByName(ctx, device.ProjectID, trimmedName)
		if err != nil {
			log.WithError(err).Error("failed to find source by name")
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find source by name"))
		}

		listenRequest.SourceID = source.UID
	}

	_, err = r.SubscriptionRepo.FindSubscriptionByDeviceID(ctx, project.UID, device.UID, datastore.SubscriptionTypeCLI)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			s := &datastore.Subscription{
				UID:          ulid.Make().String(),
				Name:         fmt.Sprintf("%s-subscription", device.HostName),
				Type:         datastore.SubscriptionTypeCLI,
				ProjectID:    project.UID,
				DeviceID:     device.UID,
				SourceID:     listenRequest.SourceID,
				FilterConfig: &datastore.FilterConfiguration{EventTypes: []string{"*"}, Filter: datastore.FilterSchema{}},
			}

			err = r.SubscriptionRepo.CreateSubscription(ctx, project.UID, s)
			if err != nil {
				return nil, util.NewServiceError(http.StatusBadRequest, err)
			}

			return device, nil
		}

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

func respondWithData(w http.ResponseWriter, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		log.WithError(err).Error("failed to marshal data")
		respond(w, http.StatusInternalServerError, "failed to marshal response: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		log.WithError(err).Error("failed to write response data")
	}
}
