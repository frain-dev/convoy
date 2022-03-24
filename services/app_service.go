package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AppService struct {
	appRepo           datastore.ApplicationRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	eventQueue        queue.Queuer
}

func NewAppService(appRepo datastore.ApplicationRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, eventQueue queue.Queuer) *AppService {
	return &AppService{appRepo: appRepo, eventRepo: eventRepo, eventDeliveryRepo: eventDeliveryRepo, eventQueue: eventQueue}
}

func (a *AppService) CreateApp(ctx context.Context, newApp *models.Application, appName string, g *datastore.Group) (*datastore.Application, error) {
	if err := util.Validate(newApp); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	uid := uuid.New().String()
	app := &datastore.Application{
		UID:            uid,
		GroupID:        g.UID,
		Title:          appName,
		SupportEmail:   newApp.SupportEmail,
		IsDisabled:     newApp.IsDisabled,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		Endpoints:      []datastore.Endpoint{},
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err := a.appRepo.CreateApplication(ctx, app)

	return app, err
}

func (a *AppService) LoadApplicationsPaged(ctx context.Context, uid string, q string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	apps, paginationData, err := a.appRepo.LoadApplicationsPaged(ctx, uid, q, pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch apps")
		return nil, datastore.PaginationData{}, NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching apps"))
	}

	return apps, paginationData, nil
}

func (a *AppService) UpdateApplication(ctx context.Context, appUpdate *models.UpdateApplication, app *datastore.Application) error {

	if appUpdate.SupportEmail != nil {
		app.SupportEmail = *appUpdate.SupportEmail
	}

	if appUpdate.IsDisabled != nil {
		app.IsDisabled = *appUpdate.IsDisabled
	}

	err := a.appRepo.UpdateApplication(ctx, app)
	if err != nil {
		return NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating app"))
	}
	return nil
}

func (a *AppService) DeleteApplication(ctx context.Context, app *datastore.Application) error {
	err := a.appRepo.DeleteApplication(ctx, app)
	if err != nil {
		log.Errorln("failed to delete app - ", err)
		return NewServiceError(http.StatusBadRequest, errors.New("an error occurred while deleting app"))
	}
	return nil
}

func (a *AppService) CreateAppEndpoint(ctx context.Context, e models.Endpoint, app *datastore.Application) (*datastore.Endpoint, error) {
	// Events being nil means it wasn't passed at all, which automatically
	// translates into a accept all scenario. This is quite different from
	// an empty array which signifies a blacklist all events -- no events
	// will be sent to such endpoints.
	if e.Events == nil {
		e.Events = []string{"*"}
	}

	if e.RateLimit == 0 {
		e.RateLimit = convoy.RATE_LIMIT
	}

	if util.IsStringEmpty(e.RateLimitDuration) {
		e.RateLimitDuration = convoy.RATE_LIMIT_DURATION
	}

	duration, err := time.ParseDuration(e.RateLimitDuration)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, fmt.Errorf((fmt.Sprintf("an error occured parsing the rate limit duration...%v", err.Error()))))
	}

	endpoint := &datastore.Endpoint{
		UID:               uuid.New().String(),
		TargetURL:         e.URL,
		Description:       e.Description,
		Events:            e.Events,
		Secret:            e.Secret,
		Status:            datastore.ActiveEndpointStatus,
		RateLimit:         e.RateLimit,
		RateLimitDuration: duration.String(),
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus:    datastore.ActiveDocumentStatus,
	}

	if util.IsStringEmpty(e.Secret) {
		endpoint.Secret, err = util.GenerateSecret()
		if err != nil {
			return nil, NewServiceError(http.StatusBadRequest, fmt.Errorf(fmt.Sprintf("could not generate secret...%v", err.Error())))
		}
	}

	app.Endpoints = append(app.Endpoints, *endpoint)

	err = a.appRepo.UpdateApplication(ctx, app)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, fmt.Errorf("an error occurred while adding app endpoint"))
	}
	return endpoint, nil
}

func (a *AppService) UpdateAppEndpoint(ctx context.Context, e models.Endpoint, endPointId string, app *datastore.Application) (*datastore.Endpoint, error) {

	endpoints, endpoint, err := updateEndpointIfFound(&app.Endpoints, endPointId, e)
	if err != nil {
		return endpoint, NewServiceError(http.StatusBadRequest, err)
	}

	app.Endpoints = *endpoints
	err = a.appRepo.UpdateApplication(ctx, app)
	if err != nil {
		return endpoint, NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating app endpoints"))
	}
	return endpoint, nil
}

func (a *AppService) DeleteAppEndpoint(ctx context.Context, e *datastore.Endpoint, app *datastore.Application) error {

	for i, endpoint := range app.Endpoints {
		if endpoint.UID == e.UID && endpoint.DeletedAt == 0 {
			app.Endpoints = append(app.Endpoints[:i], app.Endpoints[i+1:]...)
			break
		}
	}

	err := a.appRepo.UpdateApplication(ctx, app)
	if err != nil {
		return NewServiceError(http.StatusBadRequest, errors.New("an error occurred while deleting app endpoint"))
	}
	return nil
}

func updateEndpointIfFound(endpoints *[]datastore.Endpoint, id string, e models.Endpoint) (*[]datastore.Endpoint, *datastore.Endpoint, error) {
	for i, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			endpoint.TargetURL = e.URL
			endpoint.Description = e.Description

			// Events being empty means it wasn't passed at all, which automatically
			// translates into a accept all scenario. This is quite different from
			// an empty array which signifies a blacklist all events -- no events
			// will be sent to such endpoints.
			if len(e.Events) == 0 {
				endpoint.Events = []string{"*"}
			} else {
				endpoint.Events = e.Events
			}

			if e.RateLimit == 0 {
				e.RateLimit = convoy.RATE_LIMIT
			}

			if util.IsStringEmpty(e.RateLimitDuration) {
				e.RateLimitDuration = convoy.RATE_LIMIT_DURATION
			}

			duration, err := time.ParseDuration(e.RateLimitDuration)
			if err != nil {
				return nil, nil, err
			}

			endpoint.RateLimit = e.RateLimit
			endpoint.RateLimitDuration = duration.String()

			endpoint.Status = datastore.ActiveEndpointStatus
			endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
			(*endpoints)[i] = endpoint
			return endpoints, &endpoint, nil
		}
	}
	return endpoints, nil, datastore.ErrEndpointNotFound
}
