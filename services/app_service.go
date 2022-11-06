package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
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
	cache             cache.Cache
	queue             queue.Queuer
}

func NewAppService(appRepo datastore.ApplicationRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, cache cache.Cache, queue queue.Queuer) *AppService {
	return &AppService{appRepo: appRepo, eventRepo: eventRepo, eventDeliveryRepo: eventDeliveryRepo, cache: cache, queue: queue}
}

func (a *AppService) CreateApp(ctx context.Context, newApp *models.Application, g *datastore.Group) (*datastore.Application, error) {
	if err := util.Validate(newApp); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	app := &datastore.Application{
		UID:             uuid.New().String(),
		GroupID:         g.UID,
		Title:           newApp.AppName,
		SupportEmail:    newApp.SupportEmail,
		SlackWebhookURL: newApp.SlackWebhookURL,
		IsDisabled:      newApp.IsDisabled,
		CreatedAt:       primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:       primitive.NewDateTimeFromTime(time.Now()),
		Endpoints:       []datastore.Endpoint{},
	}

	err := a.appRepo.CreateApplication(ctx, app, app.GroupID)
	if err != nil {
		msg := "failed to create application"
		if err == datastore.ErrDuplicateAppName {
			msg = fmt.Sprintf("%v: %s", datastore.ErrDuplicateAppName, app.Title)
		}
		log.WithError(err).Error(msg)
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New(msg))
	}

	appCacheKey := convoy.ApplicationsCacheKey.Get(app.UID).String()
	err = a.cache.Set(ctx, appCacheKey, &app, time.Minute*5)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create application cache"))
	}

	return app, nil
}

func (a *AppService) LoadApplicationsPaged(ctx context.Context, uid string, q string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	apps, paginationData, err := a.appRepo.LoadApplicationsPaged(ctx, uid, strings.TrimSpace(q), pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch apps")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching apps"))
	}

	return apps, paginationData, nil
}

func (a *AppService) UpdateApplication(ctx context.Context, appUpdate *models.UpdateApplication, app *datastore.Application) error {
	appName := appUpdate.AppName
	if err := util.Validate(appUpdate); err != nil {
		return util.NewServiceError(http.StatusBadRequest, err)
	}

	app.Title = *appName
	if appUpdate.SupportEmail != nil {
		app.SupportEmail = *appUpdate.SupportEmail
	}

	if appUpdate.IsDisabled != nil {
		app.IsDisabled = *appUpdate.IsDisabled
	}

	if appUpdate.SlackWebhookURL != nil {
		app.SlackWebhookURL = *appUpdate.SlackWebhookURL
	}

	if appUpdate.SupportEmail != nil {
		app.SupportEmail = *appUpdate.SupportEmail
	}

	err := a.appRepo.UpdateApplication(ctx, app, app.GroupID)
	if err != nil {
		msg := "an error occurred while updating app"
		if err == datastore.ErrDuplicateAppName {
			msg = fmt.Sprintf("%v: %s", datastore.ErrDuplicateAppName, app.Title)
		}
		log.WithError(err).Error(msg)
		return util.NewServiceError(http.StatusBadRequest, errors.New(msg))
	}

	appCacheKey := convoy.ApplicationsCacheKey.Get(app.UID).String()
	err = a.cache.Set(ctx, appCacheKey, &app, time.Minute*5)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to update application cache"))
	}

	return nil
}

func (a *AppService) DeleteApplication(ctx context.Context, app *datastore.Application) error {
	err := a.appRepo.DeleteApplication(ctx, app)
	if err != nil {
		log.Errorln("failed to delete app - ", err)
		return util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while deleting app"))
	}

	appCacheKey := convoy.ApplicationsCacheKey.Get(app.UID).String()
	err = a.cache.Delete(ctx, appCacheKey)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to delete application cache"))
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
		return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf("an error occurred parsing the rate limit duration: %v", err))
	}

	endpoint := &datastore.Endpoint{
		UID:               uuid.New().String(),
		TargetURL:         e.URL,
		Description:       e.Description,
		RateLimit:         e.RateLimit,
		HttpTimeout:       e.HttpTimeout,
		RateLimitDuration: duration.String(),
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
	}

	if util.IsStringEmpty(e.Secret) {
		sc, err := util.GenerateSecret()
		if err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf(fmt.Sprintf("could not generate secret...%v", err.Error())))
		}

		endpoint.Secrets = []datastore.Secret{
			{
				UID:       uuid.NewString(),
				Value:     sc,
				CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
			},
		}
	} else {
		endpoint.Secrets = append(endpoint.Secrets, datastore.Secret{
			UID:       uuid.NewString(),
			Value:     e.Secret,
			CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
		})
	}

	auth, err := validateEndpointAuthentication(e, endpoint)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	endpoint.Authentication = auth

	err = a.appRepo.CreateApplicationEndpoint(ctx, app.GroupID, app.UID, endpoint)
	if err != nil {
		log.WithError(err).Error("failed to create application endpoint")
		return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf("an error occurred while adding app endpoint"))
	}

	app.Endpoints = append(app.Endpoints, *endpoint)
	app, err = a.appRepo.FindApplicationByID(ctx, app.UID)
	if err != nil {
		log.WithError(err).Error("failed to find application")
		return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf("failed to fetch application to update cache"))
	}

	appCacheKey := convoy.ApplicationsCacheKey.Get(app.UID).String()
	err = a.cache.Set(ctx, appCacheKey, &app, time.Minute*5)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update application cache"))
	}

	return endpoint, nil
}

func (a *AppService) UpdateAppEndpoint(ctx context.Context, e models.Endpoint, endPointId string, app *datastore.Application) (*datastore.Endpoint, error) {
	if err := util.Validate(e); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	endpoints, endpoint, err := updateEndpointIfFound(&app.Endpoints, endPointId, e)
	if err != nil {
		return endpoint, util.NewServiceError(http.StatusBadRequest, err)
	}

	app.Endpoints = *endpoints
	err = a.appRepo.UpdateApplication(ctx, app, app.GroupID)
	if err != nil {
		return endpoint, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating app endpoints"))
	}

	appCacheKey := convoy.ApplicationsCacheKey.Get(app.UID).String()
	err = a.cache.Set(ctx, appCacheKey, &app, time.Minute*5)
	if err != nil {
		return endpoint, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update application cache"))
	}

	return endpoint, nil
}

func (a *AppService) ExpireSecret(ctx context.Context, s *models.ExpireSecret, endPointId string, app *datastore.Application) (*datastore.Application, error) {
	// Expire current secret.
	endpoint, err := app.FindEndpoint(endPointId)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	idx, err := endpoint.GetActiveSecretIndex()
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	expiresAt := time.Now().Add(time.Hour * time.Duration(s.Expiration))
	endpoint.Secrets[idx].ExpiresAt = primitive.NewDateTimeFromTime(expiresAt)

	secret := endpoint.Secrets[idx]

	// Enqueue for final deletion.
	body := struct {
		AppID      string `json:"app_id"`
		EndpointID string `json:"endpoint_id"`
		SecretID   string `json:"secret_id"`
	}{
		AppID:      app.UID,
		EndpointID: endpoint.UID,
		SecretID:   secret.UID,
	}

	jobByte, err := json.Marshal(body)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	payload := json.RawMessage(jobByte)

	job := &queue.Job{
		ID:      secret.UID,
		Payload: payload,
		Delay:   time.Hour * time.Duration(s.Expiration),
	}

	taskName := convoy.ExpireSecretsProcessor
	err = a.queue.Write(taskName, convoy.DefaultQueue, job)
	if err != nil {
		log.Errorf("Error occurred sending new event to the queue %s", err)
	}

	// Generate new secret.
	newSecret := s.Secret
	if len(newSecret) == 0 {
		newSecret, err = util.GenerateSecret()
		if err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf(fmt.Sprintf("could not generate secret...%v", err.Error())))
		}
	}

	sc := datastore.Secret{
		UID:       uuid.NewString(),
		Value:     newSecret,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	secrets := append(endpoint.Secrets, sc)
	endpoint.Secrets = secrets

	err = a.appRepo.ExpireSecret(ctx, app.UID, endpoint.UID, secrets)
	if err != nil {
		log.Errorf("Error occurred expiring secret %s", err)
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to expire endpoint secret"))
	}

	appCacheKey := convoy.ApplicationsCacheKey.Get(app.UID).String()
	err = a.cache.Set(ctx, appCacheKey, &app, time.Minute*5)
	if err != nil {
		log.WithError(err).Error("failed to update app cache")
	}

	return app, nil
}

func (a *AppService) DeleteAppEndpoint(ctx context.Context, e *datastore.Endpoint, app *datastore.Application) error {
	for i, endpoint := range app.Endpoints {
		if endpoint.UID == e.UID && endpoint.DeletedAt == 0 {
			app.Endpoints = append(app.Endpoints[:i], app.Endpoints[i+1:]...)
			break
		}
	}

	err := a.appRepo.UpdateApplication(ctx, app, app.GroupID)
	if err != nil {
		log.WithError(err).Error("failed to delete app endpoint")
		return util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while deleting app endpoint"))
	}

	appCacheKey := convoy.ApplicationsCacheKey.Get(app.UID).String()
	err = a.cache.Set(ctx, appCacheKey, &app, time.Minute*5)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to update application cache"))
	}

	return nil
}

func (a *AppService) CountGroupApplications(ctx context.Context, groupID string) (int64, error) {
	apps, err := a.appRepo.CountGroupApplications(ctx, groupID)
	if err != nil {
		log.WithError(err).Error("failed to count group applications")
		return 0, util.NewServiceError(http.StatusBadRequest, errors.New("failed to count group applications"))
	}

	return apps, nil
}

func updateEndpointIfFound(endpoints *[]datastore.Endpoint, id string, e models.Endpoint) (*[]datastore.Endpoint, *datastore.Endpoint, error) {
	for i, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			endpoint.TargetURL = e.URL
			endpoint.Description = e.Description

			if e.RateLimit != 0 {
				endpoint.RateLimit = e.RateLimit
			}

			if !util.IsStringEmpty(e.RateLimitDuration) {
				duration, err := time.ParseDuration(e.RateLimitDuration)
				if err != nil {
					return nil, nil, err
				}

				endpoint.RateLimitDuration = duration.String()
			}

			if e.AdvancedSignatures != nil {
				endpoint.AdvancedSignatures = *e.AdvancedSignatures
			}

			if !util.IsStringEmpty(e.HttpTimeout) {
				endpoint.HttpTimeout = e.HttpTimeout
			}
			auth, err := validateEndpointAuthentication(e, &endpoint)
			if err != nil {
				return nil, nil, err
			}

			endpoint.Authentication = auth

			endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
			(*endpoints)[i] = endpoint
			return endpoints, &endpoint, nil
		}
	}
	return endpoints, nil, datastore.ErrEndpointNotFound
}

func validateEndpointAuthentication(e models.Endpoint, endpoint *datastore.Endpoint) (*datastore.EndpointAuthentication, error) {
	if e.Authentication != nil && !util.IsStringEmpty(string(e.Authentication.Type)) {
		if err := util.Validate(e); err != nil {
			return nil, err
		}

		if e.Authentication.ApiKey == nil && e.Authentication.Type == datastore.APIKeyAuthentication {
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("api key field is required"))
		}

		return e.Authentication, nil
	}

	return endpoint.Authentication, nil
}
