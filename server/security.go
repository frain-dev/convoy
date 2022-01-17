package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/pbkdf2"
)

// CreateAPIKey
// @Summary Create an api key
// @Description This endpoint creates an api key that will be used by the native auth realm
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param apiKey body models.APIKey true "API Key"
// @Success 200 {object} serverResponse{data=models.APIKeyResponse}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /security/keys [post]
func (a *applicationHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var newApiKey models.APIKey
	err := json.NewDecoder(r.Body).Decode(&newApiKey)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	if newApiKey.ExpiresAt != (time.Time{}) && newApiKey.ExpiresAt.Before(time.Now()) {
		_ = render.Render(w, r, newErrorResponse("expiry date is invalid", http.StatusBadRequest))
		return
	}

	err = newApiKey.Role.Validate("api key")
	if err != nil {
		log.WithError(err).Error("invalid api key role")
		_ = render.Render(w, r, newErrorResponse("invalid api key role", http.StatusBadRequest))
		return
	}

	groups, err := a.groupRepo.FetchGroupsByIDs(r.Context(), newApiKey.Role.Groups)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("invalid group", http.StatusBadRequest))
		return
	}

	if len(groups) != len(newApiKey.Role.Groups) {
		_ = render.Render(w, r, newErrorResponse("cannot find group", http.StatusBadRequest))
		return
	}

	maskID, key := util.GenerateAPIKey()

	salt, err := util.GenerateSecret()
	if err != nil {
		log.WithError(err).Error("failed to generate salt")
		_ = render.Render(w, r, newErrorResponse("something went wrong", http.StatusInternalServerError))
		return
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	apiKey := &datastore.APIKey{
		UID:            uuid.New().String(),
		MaskID:         maskID,
		Name:           newApiKey.Name,
		Type:           newApiKey.Type,
		Role:           newApiKey.Role,
		Hash:           encodedKey,
		Salt:           salt,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	if newApiKey.ExpiresAt != (time.Time{}) {
		apiKey.ExpiresAt = primitive.NewDateTimeFromTime(newApiKey.ExpiresAt)
	}

	err = a.apiKeyRepo.CreateAPIKey(r.Context(), apiKey)
	if err != nil {
		log.WithError(err).Error("failed to create api key")
		_ = render.Render(w, r, newErrorResponse("failed to create api key", http.StatusInternalServerError))
		return
	}

	resp := models.APIKeyResponse{
		APIKey: models.APIKey{
			Name:      apiKey.Name,
			Role:      apiKey.Role,
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt.Time(),
		},
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt.Time(),
		Key:       key,
	}

	_ = render.Render(w, r, newServerResponse("API Key created successfully", resp, http.StatusCreated))
}

// RevokeAPIKey
// @Summary Revoke API Key
// @Description This endpoint revokes an api key
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param keyID path string true "API Key id"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /security/keys/{keyID}/revoke [put]
func (a *applicationHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "keyID")

	if util.IsStringEmpty(uid) {
		_ = render.Render(w, r, newErrorResponse("key id is empty", http.StatusBadRequest))
		return
	}

	err := a.apiKeyRepo.RevokeAPIKeys(r.Context(), []string{uid})
	if err != nil {
		log.WithError(err).Error("failed to revoke api key")
		_ = render.Render(w, r, newErrorResponse("failed to revoke api key", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("api key revoked successfully", nil, http.StatusOK))
}

// GetAPIKeyByID
// @Summary Get api key by id
// @Description This endpoint fetches an api key by its id
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param keyID path string true "API Key id"
// @Success 200 {object} serverResponse{data=datastore.APIKey}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /security/keys/{keyID} [get]
func (a *applicationHandler) GetAPIKeyByID(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "keyID")

	if util.IsStringEmpty(uid) {
		_ = render.Render(w, r, newErrorResponse("key id is empty", http.StatusBadRequest))
		return
	}

	apiKey, err := a.apiKeyRepo.FindAPIKeyByID(r.Context(), uid)
	if err != nil {
		log.WithError(err).Error("failed to fetch api key")
		_ = render.Render(w, r, newErrorResponse("failed to fetch api key", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("api key fetched successfully", apiKey, http.StatusOK))
}

// GetAPIKeys
// @Summary Fetch multiple api keys
// @Description This endpoint fetches multiple api keys
// @Tags APIKey
// @Accept  json
// @Produce  json
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.APIKey}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /security/keys [get]
func (a *applicationHandler) GetAPIKeys(w http.ResponseWriter, r *http.Request) {
	pageable := getPageableFromContext(r.Context())

	apiKeys, paginationData, err := a.apiKeyRepo.LoadAPIKeysPaged(r.Context(), &pageable)
	if err != nil {
		log.WithError(err).Error("failed to load api keys")
		_ = render.Render(w, r, newErrorResponse("failed to load api keys", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("api keys fetched successfully",
		pagedResponse{Content: &apiKeys, Pagination: &paginationData}, http.StatusOK))
}
