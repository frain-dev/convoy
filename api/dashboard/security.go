package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func (a *DashboardHandler) CreatePersonalAPIKey(w http.ResponseWriter, r *http.Request) {
	var newApiKey models.PersonalAPIKey
	err := json.NewDecoder(r.Body).Decode(&newApiKey)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	user, ok := m.GetAuthUserFromContext(r.Context()).Metadata.(*datastore.User)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	cpk := &services.CreatePersonalAPIKeyService{
		ProjectRepo: postgres.NewProjectRepo(a.A.DB, a.A.Cache),
		UserRepo:    postgres.NewUserRepo(a.A.DB, a.A.Cache),
		APIKeyRepo:  postgres.NewAPIKeyRepo(a.A.DB, a.A.Cache),
		User:        user,
		NewApiKey:   &newApiKey,
	}

	apiKey, keyString, err := cpk.Run(r.Context())
	if err != nil {
		a.A.Logger.WithError(err).Error("failed to create personal api key")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.APIKeyResponse{
		APIKey: models.APIKey{
			Name: apiKey.Name,
			Role: models.Role{
				Type:    apiKey.Role.Type,
				Project: apiKey.Role.Project,
			},
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt,
		},
		UserID:    apiKey.UserID,
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt,
		Key:       keyString,
	}

	_ = render.Render(w, r, util.NewServerResponse("Personal API Key created successfully", resp, http.StatusCreated))
}

func (a *DashboardHandler) RevokePersonalAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := m.GetAuthUserFromContext(r.Context()).Metadata.(*datastore.User)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	rvk := &services.RevokePersonalAPIKeyService{
		ProjectRepo: postgres.NewProjectRepo(a.A.DB, a.A.Cache),
		UserRepo:    postgres.NewUserRepo(a.A.DB, a.A.Cache),
		APIKeyRepo:  postgres.NewAPIKeyRepo(a.A.DB, a.A.Cache),
		UID:         chi.URLParam(r, "keyID"),
		User:        user,
	}

	err := rvk.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("personal api key revoked successfully", nil, http.StatusOK))
}

func (a *DashboardHandler) RegenerateProjectAPIKey(w http.ResponseWriter, r *http.Request) {
	member, err := a.retrieveMembership(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if err = a.A.Authz.Authorize(r.Context(), "project.manage", project); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
		return
	}

	rgp := &services.RegenerateProjectAPIKeyService{
		ProjectRepo: postgres.NewProjectRepo(a.A.DB, a.A.Cache),
		UserRepo:    postgres.NewUserRepo(a.A.DB, a.A.Cache),
		APIKeyRepo:  postgres.NewAPIKeyRepo(a.A.DB, a.A.Cache),
		Project:     project,
		Member:      member,
	}

	apiKey, keyString, err := rgp.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.APIKeyResponse{
		APIKey: models.APIKey{
			Name: apiKey.Name,
			Role: models.Role{
				Type:    apiKey.Role.Type,
				Project: apiKey.Role.Project,
			},
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt,
		},
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt,
		Key:       keyString,
	}

	_ = render.Render(w, r, util.NewServerResponse("api key regenerated successfully", resp, http.StatusOK))
}

func (a *DashboardHandler) GetAPIKeys(w http.ResponseWriter, r *http.Request) {
	pageable := m.GetPageableFromContext(r.Context())

	f := &datastore.ApiKeyFilter{}
	keyType := datastore.KeyType(r.URL.Query().Get("keyType"))
	if keyType.IsValid() {
		f.KeyType = keyType

		if keyType == datastore.PersonalKey {
			user, ok := m.GetAuthUserFromContext(r.Context()).Metadata.(*datastore.User)
			if !ok {
				_ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
				return
			}
			f.UserID = user.UID
		}
	}

	apiKeys, paginationData, err := postgres.NewAPIKeyRepo(a.A.DB, a.A.Cache).LoadAPIKeysPaged(r.Context(), f, &pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to load api keys")
		_ = render.Render(w, r, util.NewErrorResponse("failed to load api keys", http.StatusBadRequest))
		return
	}

	apiKeyByIDResponse := apiKeyByIDResponse(apiKeys)
	_ = render.Render(w, r, util.NewServerResponse("api keys fetched successfully",
		pagedResponse{Content: &apiKeyByIDResponse, Pagination: &paginationData}, http.StatusOK))
}

func apiKeyByIDResponse(apiKeys []datastore.APIKey) []models.APIKeyByIDResponse {
	apiKeyByIDResponse := make([]models.APIKeyByIDResponse, 0)

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
