package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (a *applicationHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var newApiKey models.APIKey
	err := json.NewDecoder(r.Body).Decode(&newApiKey)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	if newApiKey.ExpiresDate != nil && newApiKey.ExpiresDate.Before(time.Now()) {
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

	if newApiKey.ExpiresDate != nil {
		apiKey.ExpiresAt = primitive.NewDateTimeFromTime(*newApiKey.ExpiresDate)
	}

	err = a.apiKeyRepo.CreateAPIKey(r.Context(), apiKey)
	if err != nil {
		log.WithError(err).Error("failed to create api key")
		_ = render.Render(w, r, newErrorResponse("failed to create api key", http.StatusInternalServerError))
		return
	}

	resp := map[string]interface{}{
		"key":         newApiKey.Key,
		"role":        newApiKey.Role,
		"expiry_date": newApiKey.ExpiresDate.Format(time.RFC3339),
		"created_at":  apiKey.CreatedAt.Time().Format(time.RFC3339),
		"uid":         apiKey.UID,
	}

	_ = render.Render(w, r, newServerResponse("API Key created successfully", resp, http.StatusCreated))
}

func (a *applicationHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("keyID")

	if util.IsStringEmpty(uid) {
		_ = render.Render(w, r, newErrorResponse("key id is empty", http.StatusBadRequest))
		return
	}

	apiKey, err := a.apiKeyRepo.FindAPIKeyByID(r.Context(), uid)
	if err != nil {
		event := "failed to fetch api key by id"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, mongo.ErrNoDocuments) {
			event = err.Error()
			statusCode = http.StatusNotFound
		}

		log.WithError(err).Error(event)
		_ = render.Render(w, r, newErrorResponse(event, statusCode))
		return
	}

	apiKey.Revoked = true
	err = a.apiKeyRepo.UpdateAPIKey(r.Context(), apiKey)
	if err != nil {
		log.WithError(err).Error("failed to update api key")
		_ = render.Render(w, r, newErrorResponse("failed to update api key", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("api key revoked successfully", nil, http.StatusOK))
}

func (a *applicationHandler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("keyID")

	if util.IsStringEmpty(uid) {
		_ = render.Render(w, r, newErrorResponse("key id is empty", http.StatusBadRequest))
		return
	}

	err := a.apiKeyRepo.DeleteAPIKey(r.Context(), uid)
	if err != nil {
		log.WithError(err).Error("failed to delete api key")
		_ = render.Render(w, r, newErrorResponse("failed to delete api key", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("api key deleted successfully", nil, http.StatusOK))
}
