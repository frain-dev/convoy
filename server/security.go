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

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createSecurityService(a *ApplicationHandler) *services.SecurityService {
	groupRepo := mongo.NewGroupRepo(a.A.Store)
	apiKeyRepo := mongo.NewApiKeyRepo(a.A.Store)

	return services.NewSecurityService(groupRepo, apiKeyRepo)
}

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
		a.A.Logger.WithError(err).Error("failed to create api key")
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
		a.A.Logger.WithError(err).Error("failed to create personal api key")
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

func _() {}

// CreateEndpointAPIKey
// @Summary Create an api key for endpoint portal or the cli (API)
// @Description This endpoint creates an api key that will be used by endpoint portal or the cli
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project id"
// @Param endpointID path string true "endpoint ID"
// @Param appAPIKey body models.APIKey true "APIKey details"
// @Success 201 {object} util.ServerResponse{data=models.PortalAPIKeyResponse}
// @Failure 400,401,500 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/security/endpoints/{endpointID}/keys [post]
func (a *ApplicationHandler) CreateEndpointAPIKey(w http.ResponseWriter, r *http.Request) {
	var keyType datastore.KeyType
	var newApiKey models.CreateEndpointApiKey

	if err := util.ReadJSON(r, &newApiKey); err != nil {
		// Disregard the ErrEmptyBody err to ensure backward compatibility
		if !errors.Is(err, util.ErrEmptyBody) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
	}

	group := m.GetGroupFromContext(r.Context())
	endpoint := m.GetEndpointFromContext(r.Context())
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
	newApiKey.Endpoint = endpoint
	newApiKey.BaseUrl = baseUrl
	newApiKey.KeyType = keyType

	securityService := createSecurityService(a)
	apiKey, key, err := securityService.CreateEndpointAPIKey(r.Context(), &newApiKey)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if !util.IsStringEmpty(baseUrl) && newApiKey.KeyType == datastore.AppPortalKey {
		baseUrl = fmt.Sprintf("%s/endpoint/%s?groupID=%s&endpointId=%s", baseUrl, key, newApiKey.Group.UID, newApiKey.Endpoint.UID)
	}

	resp := models.PortalAPIKeyResponse{
		Key:        key,
		Url:        baseUrl,
		Role:       apiKey.Role,
		GroupID:    group.UID,
		EndpointID: endpoint.UID,
		Type:       string(apiKey.Type),
	}

	_ = render.Render(w, r, util.NewServerResponse("API Key created successfully", resp, http.StatusCreated))
}

func (a *ApplicationHandler) LoadEndpointAPIKeysPaged(w http.ResponseWriter, r *http.Request) {
	group := m.GetGroupFromContext(r.Context())
	endpoint := m.GetEndpointFromContext(r.Context())
	pageable := m.GetPageableFromContext(r.Context())

	f := &datastore.ApiKeyFilter{
		GroupID:    group.UID,
		EndpointID: endpoint.UID,
		KeyType:    datastore.CLIKey,
	}

	securityService := createSecurityService(a)
	apiKeys, paginationData, err := securityService.GetAPIKeys(r.Context(), f, &pageable)
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to load api keys")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	apiKeyByIDResponse := apiKeyByIDResponse(apiKeys)
	_ = render.Render(w, r, util.NewServerResponse("api keys fetched successfully",
		pagedResponse{Content: &apiKeyByIDResponse, Pagination: &paginationData}, http.StatusOK))
}

func (a *ApplicationHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	securityService := createSecurityService(a)

	err := securityService.RevokeAPIKey(r.Context(), chi.URLParam(r, "keyID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("api key revoked successfully", nil, http.StatusOK))
}

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

func (a *ApplicationHandler) RevokeEndpointAPIKey(w http.ResponseWriter, r *http.Request) {
	endpoint := m.GetEndpointFromContext(r.Context())
	group := m.GetGroupFromContext(r.Context())

	securityService := createSecurityService(a)
	key, err := securityService.GetAPIKeyByID(r.Context(), chi.URLParam(r, "keyID"))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if key.Role.Group != group.UID || key.Role.Endpoint != endpoint.UID {
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

func _() {}

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
		a.A.Logger.WithError(err).Error("failed to load api keys")
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
