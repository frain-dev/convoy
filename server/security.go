package server

import (
	"encoding/json"
	"net/http"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

// CreateAPIKey
// @Summary Create an api key
// @Description This endpoint creates an api key that will be used by the native auth realm
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param apiKey body models.APIKey true "API Key"
// @Success 200 {object} serverResponse{data=models.APIKeyResponse}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys [post]
func (s *Server) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var newApiKey models.APIKey
	err := json.NewDecoder(r.Body).Decode(&newApiKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	member := m.GetOrganisationMemberFromContext(r.Context())
	apiKey, keyString, err := s.securityService.CreateAPIKey(r.Context(), member, &newApiKey)
	if err != nil {
		log.WithError(err).Error("fff")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.APIKeyResponse{
		APIKey: models.APIKey{
			Name: apiKey.Name,
			Role: models.Role{
				Type:  apiKey.Role.Type,
				Group: apiKey.Role.Groups[0],
			},
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt.Time(),
		},
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt.Time(),
		Key:       keyString,
	}

	_ = render.Render(w, r, util.NewServerResponse("API Key created successfully", resp, http.StatusCreated))
}

// CreateAppPortalAPIKey - this serves as a duplicate to generate doc for the ui route of this handler
// @Summary Create an api key for app portal (UI)
// @Description This endpoint creates an api key that will be used by app portal
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param appID path string true "application ID"
// @Success 201 {object} serverResponse{data=models.PortalAPIKeyResponse}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/applications/{appID}/keys [post]

// CreateAppPortalAPIKey
// @Summary Create an api key for app portal (API)
// @Description This endpoint creates an api key that will be used by app portal
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param appID path string true "application ID"
// @Success 201 {object} serverResponse{data=models.PortalAPIKeyResponse}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /security/applications/{appID}/keys [post]
func (s *Server) CreateAppPortalAPIKey(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())
	app := m.GetApplicationFromContext(r.Context())
	baseUrl := m.GetHostFromContext(r.Context())

	apiKey, key, err := s.securityService.CreateAppPortalAPIKey(r.Context(), group, app, &baseUrl)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := models.PortalAPIKeyResponse{
		Key:     key,
		Url:     baseUrl,
		Role:    apiKey.Role,
		GroupID: group.UID,
		AppID:   app.UID,
		Type:    string(apiKey.Type),
	}

	_ = render.Render(w, r, util.NewServerResponse("API Key created successfully", resp, http.StatusCreated))

}

// RevokeAPIKey
// @Summary Revoke API Key
// @Description This endpoint revokes an api key
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param keyID path string true "API Key id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys/{keyID}/revoke [put]
func (s *Server) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	err := s.securityService.RevokeAPIKey(r.Context(), chi.URLParam(r, "keyID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("api key revoked successfully", nil, http.StatusOK))
}

// GetAPIKeyByID
// @Summary Get api key by id
// @Description This endpoint fetches an api key by its id
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param keyID path string true "API Key id"
// @Success 200 {object} serverResponse{data=datastore.APIKey}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys/{keyID} [get]
func (s *Server) GetAPIKeyByID(w http.ResponseWriter, r *http.Request) {
	apiKey, err := s.securityService.GetAPIKeyByID(r.Context(), chi.URLParam(r, "keyID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
	resp := models.APIKeyByIDResponse{
		UID:       apiKey.UID,
		Name:      apiKey.Name,
		Role:      apiKey.Role,
		Type:      apiKey.Type,
		ExpiresAt: apiKey.ExpiresAt,
		UpdatedAt: apiKey.UpdatedAt,
		CreatedAt: apiKey.CreatedAt,
	}

	_ = render.Render(w, r, util.NewServerResponse("api key fetched successfully", resp, http.StatusOK))
}

// UpdateAPIKey
// @Summary update api key
// @Description This endpoint updates an api key
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param keyID path string true "API Key id"
// @Success 200 {object} serverResponse{data=datastore.APIKey}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys/{keyID} [put]
func (s *Server) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	var updateApiKey struct {
		Role auth.Role `json:"role"`
	}

	err := util.ReadJSON(r, &updateApiKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	apiKey, err := s.securityService.UpdateAPIKey(r.Context(), chi.URLParam(r, "keyID"), &updateApiKey.Role)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := models.APIKeyByIDResponse{
		UID:       apiKey.UID,
		Name:      apiKey.Name,
		Role:      apiKey.Role,
		Type:      apiKey.Type,
		ExpiresAt: apiKey.ExpiresAt,
		UpdatedAt: apiKey.UpdatedAt,
		CreatedAt: apiKey.CreatedAt,
	}

	_ = render.Render(w, r, util.NewServerResponse("api key updated successfully", resp, http.StatusOK))
}

// GetAPIKeys
// @Summary Fetch multiple api keys
// @Description This endpoint fetches multiple api keys
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.APIKey}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys [get]
func (s *Server) GetAPIKeys(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())

	apiKeys, paginationData, err := s.securityService.GetAPIKeys(r.Context(), &pageable)
	if err != nil {
		log.WithError(err).Error("failed to load api keys")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	apiKeyByIDResponse := apiKeyByIDResponse(apiKeys)
	_ = render.Render(w, r, util.NewServerResponse("api keys fetched successfully",
		pagedResponse{Content: &apiKeyByIDResponse, Pagination: &paginationData}, http.StatusOK))
}

func apiKeyByIDResponse(apiKeys []datastore.APIKey) []models.APIKeyByIDResponse {
	apiKeyByIDResponse := []models.APIKeyByIDResponse{}

	for _, apiKey := range apiKeys {
		resp := models.APIKeyByIDResponse{
			UID:       apiKey.UID,
			Name:      apiKey.Name,
			Role:      apiKey.Role,
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt,
			UpdatedAt: apiKey.UpdatedAt,
			CreatedAt: apiKey.CreatedAt,
		}

		apiKeyByIDResponse = append(apiKeyByIDResponse, resp)
	}

	return apiKeyByIDResponse

}
