package server

import (
	"log"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

// LoadConfiguration
// @Summary Fetch configuration
// @Description This endpoint fetches configuration
// @Tags Source
// @Accept  json
// @Produce  json
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.ConfigurationResponse}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /configuration [get]
func (a *ApplicationHandler) LoadConfiguration(w http.ResponseWriter, r *http.Request) {
	config, err := a.S.ConfigService.LoadConfiguration(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	configResponse := []*models.ConfigurationResponse{}
	if config != nil {
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

// CreateConfiguration
// @Summary Create a configuration
// @Description This endpoint creates a configuration
// @Tags Application
// @Accept  json
// @Produce  json
// @Param application body models.Configuration true "Configuration Details"
// @Success 200 {object} util.ServerResponse{data=models.ConfigurationResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /configuration [post]
func (a *ApplicationHandler) CreateConfiguration(w http.ResponseWriter, r *http.Request) {
	var newConfig models.Configuration
	if err := util.ReadJSON(r, &newConfig); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	config, err := a.S.ConfigService.CreateConfiguration(r.Context(), &newConfig)
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

// UpdateConfiguration
// @Summary Update configuration
// @Description This endpoint updates configuration
// @Tags Application
// @Accept  json
// @Produce  json
// @Param application body models.Configuration true "Configuration Details"
// @Success 202 {object} util.ServerResponse{data=models.ConfigurationResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /configuration [put]
func (a *ApplicationHandler) UpdateConfiguration(w http.ResponseWriter, r *http.Request) {
	var newConfig models.Configuration
	if err := util.ReadJSON(r, &newConfig); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	config, err := a.S.ConfigService.UpdateConfiguration(r.Context(), &newConfig)
	if err != nil {
		log.Println(err)
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
