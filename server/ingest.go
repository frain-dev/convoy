package server

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/crc"
	"github.com/frain-dev/convoy/pkg/verifier"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *applicationHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	// 1. Retrieve mask ID
	maskID := chi.URLParam(r, "maskID")

	// 2. Retrieve source using mask ID.
	source, err := a.sourceRepo.FindSourceByMaskID(r.Context(), maskID)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if source.Type != datastore.HTTPSource {
		_ = render.Render(w, r, newErrorResponse("Source type needs to be HTTP",
			http.StatusBadRequest))
		return
	}

	// 3. Select verifier based of source config.
	// TODO(subomi): Can verifier be nil?
	var v verifier.Verifier
	verifierConfig := source.Verifier

	if !util.IsStringEmpty(string(source.Provider)) {
		switch source.Provider {
		case datastore.GithubSourceProvider:
			v = verifier.NewGithubVerifier(verifierConfig.HMac.Secret)
		case datastore.TwitterSourceProvider:
			v = verifier.NewTwitterVerifier(verifierConfig.HMac.Secret)
		default:
			_ = render.Render(w, r, newErrorResponse("Provider type undefined",
				http.StatusBadRequest))
			return
		}
	} else {
		switch verifierConfig.Type {
		case datastore.HMacVerifier:
			opts := &verifier.HmacOptions{
				Header:   verifierConfig.HMac.Header,
				Hash:     verifierConfig.HMac.Hash,
				Secret:   verifierConfig.HMac.Secret,
				Encoding: string(verifierConfig.HMac.Encoding),
			}
			v = verifier.NewHmacVerifier(opts)

		case datastore.BasicAuthVerifier:
			v = verifier.NewBasicAuthVerifier(
				verifierConfig.BasicAuth.UserName,
				verifierConfig.BasicAuth.Password,
			)
		case datastore.APIKeyVerifier:
			v = verifier.NewAPIKeyVerifier(
				verifierConfig.ApiKey.HeaderValue,
				verifierConfig.ApiKey.HeaderName,
			)
		default:
			_ = render.Render(w, r, newErrorResponse("Source must have a valid verifier",
				http.StatusBadRequest))
			return
		}
	}

	// 3.1 On Failure
	// Return 400 Bad Request.
	body := io.LimitReader(r.Body, config.MaxRequestSize)
	payload, err := io.ReadAll(body)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err = v.VerifyRequest(r, payload); err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// 3.2 On success
	// Attach Source to Event.
	// Write Event to the Ingestion Queue.
	event := &datastore.Event{
		UID:            uuid.New().String(),
		EventType:      datastore.EventType(maskID),
		SourceID:       source.UID,
		GroupID:        source.GroupID,
		Data:           payload,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	eventByte, err := json.Marshal(event)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	job := &queue.Job{
		ID:      event.UID,
		Payload: eventByte,
		Delay:   0,
	}

	err = a.queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		log.Errorf("Error occurred sending new event to the queue %s", err)
	}

	// 4. Return 200
	_ = render.Render(w, r, newServerResponse("Event received", nil, http.StatusOK))
}

func (a *applicationHandler) HandleCrcCheck(w http.ResponseWriter, r *http.Request) {
	maskID := chi.URLParam(r, "maskID")

	var source *datastore.Source
	sourceCacheKey := convoy.SourceCacheKey.Get(maskID).String()

	err := a.cache.Get(r.Context(), sourceCacheKey, &source)
	if err != nil {
		log.Error(err)
	}

	if source == nil {
		source, err = a.sourceRepo.FindSourceByMaskID(r.Context(), maskID)
		if err != nil {
			_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		err = a.cache.Set(r.Context(), sourceCacheKey, &source, time.Hour*24)
		if err != nil {
			log.Error(err)
		}

	}

	if source.Type != datastore.HTTPSource {
		_ = render.Render(w, r, newErrorResponse("Source type needs to be HTTP", http.StatusBadRequest))
		return
	}

	if util.IsStringEmpty(string(source.Provider)) {
		_ = render.Render(w, r, newErrorResponse("Provider type undefined", http.StatusBadRequest))
		return
	}

	var c crc.Crc

	switch source.Provider {
	case datastore.TwitterSourceProvider:
		c = crc.NewTwitterCrc(source.Verifier.HMac.Secret)
	default:
		_ = render.Render(w, r, newErrorResponse("Provider type is not supported", http.StatusBadRequest))
		return
	}

	res := c.HandleRequest(r)
	data, err := json.Marshal(res)
	if err != nil {
		log.Errorf("Unable to marshal response data - %s", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
}
