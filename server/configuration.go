package server

import (
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createConfigService(a *ApplicationHandler) *services.ConfigService {
	configRepo := postgres.NewConfigRepo(a.A.DB)

	return services.NewConfigService(
		configRepo,
	)
}

func (a *ApplicationHandler) LoadConfiguration(w http.ResponseWriter, r *http.Request) {
	configService := createConfigService(a)
	config, err := configService.LoadConfiguration(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	configResponse := []*models.ConfigurationResponse{}
	if config != nil {
		if config.StoragePolicy.Type == datastore.S3 {
			policy := &datastore.S3Storage{}
			policy.Bucket = config.StoragePolicy.S3.Bucket
			policy.Endpoint = config.StoragePolicy.S3.Endpoint
			policy.Region = config.StoragePolicy.S3.Region
			config.StoragePolicy.S3 = policy
		}

		c := &models.ConfigurationResponse{
			UID:                config.UID,
			IsAnalyticsEnabled: config.IsAnalyticsEnabled,
			IsSignupEnabled:    config.IsSignupEnabled,
			StoragePolicy:      config.StoragePolicy,
			ApiVersion:         convoy.GetVersion(),
			CreatedAt:          config.CreatedAt,
			UpdatedAt:          config.UpdatedAt,
			DeletedAt:          config.DeletedAt,
		}

		configResponse = append(configResponse, c)
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration fetched successfully", configResponse, http.StatusOK))
}

func (a *ApplicationHandler) CreateConfiguration(w http.ResponseWriter, r *http.Request) {
	var newConfig models.Configuration
	if err := util.ReadJSON(r, &newConfig); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	configService := createConfigService(a)
	config, err := configService.CreateConfiguration(r.Context(), &newConfig)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	c := &models.ConfigurationResponse{
		UID:                config.UID,
		IsAnalyticsEnabled: config.IsAnalyticsEnabled,
		IsSignupEnabled:    config.IsSignupEnabled,
		StoragePolicy:      config.StoragePolicy,
		ApiVersion:         convoy.GetVersion(),
		CreatedAt:          config.CreatedAt,
		UpdatedAt:          config.UpdatedAt,
		DeletedAt:          config.DeletedAt,
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration created successfully", c, http.StatusCreated))
}

func (a *ApplicationHandler) UpdateConfiguration(w http.ResponseWriter, r *http.Request) {
	var newConfig models.Configuration
	if err := util.ReadJSON(r, &newConfig); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	configService := createConfigService(a)
	config, err := configService.UpdateConfiguration(r.Context(), &newConfig)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	c := &models.ConfigurationResponse{
		UID:                config.UID,
		IsAnalyticsEnabled: config.IsAnalyticsEnabled,
		IsSignupEnabled:    config.IsSignupEnabled,
		StoragePolicy:      config.StoragePolicy,
		ApiVersion:         convoy.GetVersion(),
		CreatedAt:          config.CreatedAt,
		UpdatedAt:          config.UpdatedAt,
		DeletedAt:          config.DeletedAt,
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration updated successfully", c, http.StatusAccepted))
}
