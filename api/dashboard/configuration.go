package dashboard

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func (a *DashboardHandler) LoadConfiguration(w http.ResponseWriter, r *http.Request) {
	config, err := postgres.NewConfigRepo(a.A.DB).LoadConfiguration(r.Context())
	if err != nil && !errors.Is(err, datastore.ErrConfigNotFound) {
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
			Configuration: config,
			ApiVersion:    convoy.GetVersion(),
		}

		configResponse = append(configResponse, c)
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration fetched successfully", configResponse, http.StatusOK))
}

func (a *DashboardHandler) CreateConfiguration(w http.ResponseWriter, r *http.Request) {
	var newConfig models.Configuration
	if err := util.ReadJSON(r, &newConfig); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := newConfig.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	cc := services.CreateConfigService{
		ConfigRepo: postgres.NewConfigRepo(a.A.DB),
		NewConfig:  &newConfig,
	}

	config, err := cc.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	c := &models.ConfigurationResponse{
		Configuration: config,
		ApiVersion:    convoy.GetVersion(),
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration created successfully", c, http.StatusCreated))
}

func (a *DashboardHandler) UpdateConfiguration(w http.ResponseWriter, r *http.Request) {
	var newConfig models.Configuration
	if err := util.ReadJSON(r, &newConfig); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := newConfig.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	uc := services.UpdateConfigService{
		ConfigRepo: postgres.NewConfigRepo(a.A.DB),
		Config:     &newConfig,
	}

	config, err := uc.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	c := &models.ConfigurationResponse{
		Configuration: config,
		ApiVersion:    convoy.GetVersion(),
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration updated successfully", c, http.StatusAccepted))
}

func (a *DashboardHandler) IsSignUpEnabled(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Get()
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to load configuration")
		_ = render.Render(w, r, util.NewErrorResponse("failed to load configuration", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Configuration loaded successfully", cfg.Auth.IsSignupEnabled, http.StatusOK))
}
