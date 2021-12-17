package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *applicationHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var newApiKey models.APIKey
	err := json.NewDecoder(r.Body).Decode(&newApiKey)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	if newApiKey.ExpiresAt != nil && newApiKey.ExpiresAt.Before(time.Now()) {
		_ = render.Render(w, r, newErrorResponse("expiry date is invalid", http.StatusBadRequest))
		return
	}

	if util.IsStringEmpty(newApiKey.Key) {
		newApiKey.Key, err = util.GenerateSecret()
		if err != nil {
			log.WithError(err).Error("failed to generate api key")
			_ = render.Render(w, r, newErrorResponse("failed to generate api key", http.StatusInternalServerError))
			return
		}
	}

	err = newApiKey.Role.Validate("api key")
	if err != nil {
		log.WithError(err).Error("invalid api key role")
		_ = render.Render(w, r, newErrorResponse("invalid api key role", http.StatusBadRequest))
		return
	}

	hashedKey, err := util.ComputeSHA256(newApiKey.Key)
	if err != nil {
		log.WithError(err).Error("failed to hash api key")
		_ = render.Render(w, r, newErrorResponse("failed to hash api key", http.StatusInternalServerError))
		return
	}

	apiKey := &convoy.APIKey{
		UID:       uuid.New().String(),
		Role:      newApiKey.Role,
		Hash:      hashedKey,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	if newApiKey.ExpiresAt != nil {
		apiKey.ExpiresAt = primitive.NewDateTimeFromTime(*newApiKey.ExpiresAt)
	}

	err = a.apiKeyRepo.CreateAPIKey(r.Context(), apiKey)
	if err != nil {
		log.WithError(err).Error("failed to create api key")
		_ = render.Render(w, r, newErrorResponse("failed to create api key", http.StatusInternalServerError))
		return
	}

	resp := map[string]interface{}{
		"key":        newApiKey.Key,
		"role":       apiKey.Role,
		"created_at": apiKey.CreatedAt.Time().Format(time.RFC3339),
		"uid":        apiKey.UID,
	}

	if apiKey.ExpiresAt != 0 {
		resp["expiry_date"] = apiKey.ExpiresAt.Time().Format(time.RFC3339)
	}

	_ = render.Render(w, r, newServerResponse("API Key created successfully", resp, http.StatusCreated))
}

func (a *applicationHandler) RevokeAPIKeys(w http.ResponseWriter, r *http.Request) {
	var uids []string

	err := json.NewDecoder(r.Body).Decode(&uids)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	err = a.apiKeyRepo.RevokeAPIKeys(r.Context(), uids)
	if err != nil {
		log.WithError(err).Error("failed to revoke api keys")
		_ = render.Render(w, r, newErrorResponse("failed to revoke api keys", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("api keys revoked successfully", nil, http.StatusOK))
}

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

func (a *applicationHandler) GetAPIKeys(w http.ResponseWriter, r *http.Request) {
	pageable := getPageableFromContext(r.Context())

	apiKey, paginationData, err := a.apiKeyRepo.LoadAPIKeysPaged(r.Context(), &pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch api key")
		_ = render.Render(w, r, newErrorResponse("failed to fetch api key", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("api keys fetched successfully",
		pagedResponse{Content: &apiKey, Pagination: paginationData}, http.StatusOK))
}
