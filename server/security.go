package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/cip8/autoname"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createSecurityService(a *ApplicationHandler) *services.SecurityService {
	groupRepo := mongo.NewGroupRepo(a.A.Store)
	apiKeyRepo := mongo.NewApiKeyRepo(a.A.Store)

	return services.NewSecurityService(groupRepo, apiKeyRepo)
}

// CreateAPIKey
// @Summary Create an api key
// @Description This endpoint creates an api key that will be used by the native auth realm
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param apiKey body models.APIKey true "API Key"
// @Success 200 {object} util.ServerResponse{data=models.APIKeyResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys [post]
func (a *ApplicationHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var newApiKey models.APIKey
	err := json.NewDecoder(r.Body).Decode(&newApiKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	member := m.GetOrganisationMemberFromContext(r.Context())
	securityService := createSecurityService(a)

	apiKey, keyString, err := securityService.CreateAPIKey(r.Context(), member, &newApiKey)
	if err != nil {
		log.WithError(err).Error("failed to create api key")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.APIKeyResponse{
		APIKey: models.APIKey{
			Name: apiKey.Name,
			Role: models.Role{
				Type:  apiKey.Role.Type,
				Group: apiKey.Role.Group,
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

// CreatePersonalAPIKey
// @Summary Create a personal api key
// @Description This endpoint creates a personal api key that can be used to authenticate to this user's context
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param userID path string true "User id"
// @Param apiKey body models.PersonalAPIKey true "API Key"
// @Success 200 {object} util.ServerResponse{data=models.APIKeyResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/users/{userID}/security/personal_api_keys [post]
func (a *ApplicationHandler) CreatePersonalAPIKey(w http.ResponseWriter, r *http.Request) {
	var newApiKey models.PersonalAPIKey
	err := json.NewDecoder(r.Body).Decode(&newApiKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	user, ok := m.GetAuthUserFromContext(r.Context()).Metadata.(*datastore.User)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	securityService := createSecurityService(a)
	apiKey, keyString, err := securityService.CreatePersonalAPIKey(r.Context(), user, &newApiKey)
	if err != nil {
		log.WithError(err).Error("failed to create personal api key")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.APIKeyResponse{
		APIKey: models.APIKey{
			Name: apiKey.Name,
			Role: models.Role{
				Type:  apiKey.Role.Type,
				Group: apiKey.Role.Group,
			},
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt.Time(),
		},
		UserID:    apiKey.UserID,
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt.Time(),
		Key:       keyString,
	}

	_ = render.Render(w, r, util.NewServerResponse("Personal API Key created successfully", resp, http.StatusCreated))
}

// CreateAppAPIKey - this serves as a duplicate to generate doc for the ui route of this handler
// @Summary Create an api key for app portal or the cli (UI)
// @Description This endpoint creates an api key that will be used by app portal or the cli
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param groupID path string true "Group id"
// @Param appID path string true "application ID"
// @Success 201 {object} util.ServerResponse{data=models.PortalAPIKeyResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/groups/{groupID}/apps/{appID}/keys [post]
func _() {}

// CreateAppAPIKey
// @Summary Create an api key for app portal or the cli (API)
// @Description This endpoint creates an api key that will be used by app portal or the cli
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param appID path string true "application ID"
// @Param appAPIKey body models.APIKey true "APIKey details"
// @Success 201 {object} util.ServerResponse{data=models.PortalAPIKeyResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/security/applications/{appID}/keys [post]
func (a *ApplicationHandler) CreateAppAPIKey(w http.ResponseWriter, r *http.Request) {
	var keyType datastore.KeyType
	var newApiKey models.CreateAppApiKey

	if err := util.ReadJSON(r, &newApiKey); err != nil {
		// Disregard the ErrEmptyBody err to ensure backward compatibility
		if !errors.Is(err, util.ErrEmptyBody) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
	}

	group := m.GetGroupFromContext(r.Context())
	app := m.GetApplicationFromContext(r.Context())
	baseUrl := m.GetHostFromContext(r.Context())

	k := string(newApiKey.KeyType)

	if util.IsStringEmpty(k) {
		keyType = datastore.AppPortalKey
	}

	if !util.IsStringEmpty(k) {
		keyType = datastore.KeyType(k)
		if !keyType.IsValidAppKey() {
			_ = render.Render(w, r, util.NewErrorResponse(errors.New("type is not supported").Error(), http.StatusBadRequest))
			return
		}
	}

	if newApiKey.Expiration == 0 {
		newApiKey.Expiration = 7
	}

	if util.IsStringEmpty(newApiKey.Name) {
		newApiKey.Name = autoname.Generate(" ")
	}

	newApiKey.Group = group
	newApiKey.App = app
	newApiKey.BaseUrl = baseUrl
	newApiKey.KeyType = keyType

	securityService := createSecurityService(a)
	apiKey, key, err := securityService.CreateAppAPIKey(r.Context(), &newApiKey)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if !util.IsStringEmpty(baseUrl) && newApiKey.KeyType == datastore.AppPortalKey {
		baseUrl = fmt.Sprintf("%s/app/%s?groupID=%s&appId=%s", baseUrl, key, newApiKey.Group.UID, newApiKey.App.UID)
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

// LoadAppAPIKeysPaged
// @Summary Fetch multiple api keys belonging to an app
// @Description This endpoint fetches multiple api keys belonging to an app
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param appID path string true "application ID"
// @Success 201 {object} util.ServerResponse{data=models.PortalAPIKeyResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/applications/{appID}/keys [get]
func (a *ApplicationHandler) LoadAppAPIKeysPaged(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())
	app := m.GetApplicationFromContext(r.Context())
	pageable := m.GetPageableFromContext(r.Context())

	f := &datastore.ApiKeyFilter{
		GroupID: group.UID,
		AppID:   app.UID,
		KeyType: datastore.CLIKey,
	}

	securityService := createSecurityService(a)
	apiKeys, paginationData, err := securityService.GetAPIKeys(r.Context(), f, &pageable)
	if err != nil {
		log.WithError(err).Error("failed to load api keys")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	apiKeyByIDResponse := apiKeyByIDResponse(apiKeys)
	_ = render.Render(w, r, util.NewServerResponse("api keys fetched successfully",
		pagedResponse{Content: &apiKeyByIDResponse, Pagination: &paginationData}, http.StatusOK))
}

// RevokeAPIKey
// @Summary Revoke API Key
// @Description This endpoint revokes an api key
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param keyID path string true "API Key id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys/{keyID}/revoke [put]
func (a *ApplicationHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	securityService := createSecurityService(a)

	err := securityService.RevokeAPIKey(r.Context(), chi.URLParam(r, "keyID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("api key revoked successfully", nil, http.StatusOK))
}

// RevokePersonalAPIKey
// @Summary Revoke a Personal API Key
// @Description This endpoint revokes a personal api key
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param userID path string true "User id"
// @Param keyID path string true "API Key id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/users/{userID}/security/personal_api_keys/{keyID}/revoke [put]
func (a *ApplicationHandler) RevokePersonalAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := m.GetAuthUserFromContext(r.Context()).Metadata.(*datastore.User)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		return
	}

	securityService := createSecurityService(a)
	err := securityService.RevokePersonalAPIKey(r.Context(), chi.URLParam(r, "keyID"), user)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("personal api key revoked successfully", nil, http.StatusOK))
}

// RevokeAppAPIKey
// @Summary Revoke an App's API Key
// @Description This endpoint revokes app's an api key
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param groupID path string true "Group id"
// @Param appID path string true "application id"
// @Param keyID path string true "API Key id"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/groups/{groupID}/apps/{appID}/keys/{keyID}/revoke [put]
func (a *ApplicationHandler) RevokeAppAPIKey(w http.ResponseWriter, r *http.Request) {
	app := m.GetApplicationFromContext(r.Context())
	group := m.GetGroupFromContext(r.Context())

	securityService := createSecurityService(a)
	key, err := securityService.GetAPIKeyByID(r.Context(), chi.URLParam(r, "keyID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if key.Role.Group != group.UID || key.Role.App != app.UID {
		_ = render.Render(w, r, util.NewErrorResponse(datastore.ErrNotAuthorisedToAccessDocument.Error(), http.StatusForbidden))
		return
	}

	err = securityService.RevokeAPIKey(r.Context(), chi.URLParam(r, "keyID"))
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
// @Success 200 {object} util.ServerResponse{data=datastore.APIKey}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys/{keyID} [get]
func (a *ApplicationHandler) GetAPIKeyByID(w http.ResponseWriter, r *http.Request) {
	securityService := createSecurityService(a)

	apiKey, err := securityService.GetAPIKeyByID(r.Context(), chi.URLParam(r, "keyID"))
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
// @Success 200 {object} util.ServerResponse{data=datastore.APIKey}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys/{keyID} [put]
func (a *ApplicationHandler) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	var updateApiKey struct {
		Role auth.Role `json:"role"`
	}

	err := util.ReadJSON(r, &updateApiKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	securityService := createSecurityService(a)
	apiKey, err := securityService.UpdateAPIKey(r.Context(), chi.URLParam(r, "keyID"), &updateApiKey.Role)
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

// GetAPIKeys - this is a duplicate annotation for the User security route for this handler
// @Summary Fetch multiple api keys
// @Description This endpoint fetches multiple api keys
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param userID path string true "User id"
// @Param keyType query string false "api key type"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.APIKey}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/users/{userID}/security/personal_api_keys [get]
func _() {}

// GetAPIKeys
// @Summary Fetch multiple api keys
// @Description This endpoint fetches multiple api keys
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param orgID path string true "Organisation id"
// @Param keyType query string false "api key type"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.APIKey}}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /ui/organisations/{orgID}/security/keys [get]
func (a *ApplicationHandler) GetAPIKeys(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())
	securityService := createSecurityService(a)
	f := &datastore.ApiKeyFilter{}
	keyType := datastore.KeyType(r.URL.Query().Get("keyType"))
	if keyType.IsValid() {
		f.KeyType = keyType

		if keyType == datastore.PersonalKey {
			user, ok := m.GetAuthUserFromContext(r.Context()).Metadata.(*datastore.User)
			if !ok {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}
			f.UserID = user.UID
		}
	}

	apiKeys, paginationData, err := securityService.GetAPIKeys(r.Context(), f, &pageable)
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
