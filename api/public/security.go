package public

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cip8/autoname"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

func createSecurityService(a *PublicHandler) *services.SecurityService {
	projectRepo := postgres.NewProjectRepo(a.A.DB)
	apiKeyRepo := postgres.NewAPIKeyRepo(a.A.DB)

	return services.NewSecurityService(projectRepo, apiKeyRepo)
}

func (a *PublicHandler) CreateEndpointAPIKey(w http.ResponseWriter, r *http.Request) {
	var keyType datastore.KeyType
	var newApiKey models.CreateEndpointApiKey

	if err := util.ReadJSON(r, &newApiKey); err != nil {
		// Disregard the ErrEmptyBody err to ensure backward compatibility
		if !errors.Is(err, util.ErrEmptyBody) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
	}

	project := m.GetProjectFromContext(r.Context())
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

	newApiKey.Project = project
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
		baseUrl = fmt.Sprintf("%s/endpoint/%s?projectID=%s&endpointId=%s", baseUrl, key, newApiKey.Project.UID, newApiKey.Endpoint.UID)
	}

	resp := models.PortalAPIKeyResponse{
		Key:        key,
		Url:        baseUrl,
		Role:       apiKey.Role,
		ProjectID:  project.UID,
		EndpointID: endpoint.UID,
		Type:       string(apiKey.Type),
	}

	_ = render.Render(w, r, util.NewServerResponse("API Key created successfully", resp, http.StatusCreated))
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
