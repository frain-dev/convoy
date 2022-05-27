package server

import (
	"io"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/verifier"
	"github.com/frain-dev/convoy/queue"
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
	}

	// 3. Select verifier based of source config.
	// TODO(subomi): Can verifier be nil?
	var v verifier.Verifier
	verifierConfig := source.Verifier

	switch verifierConfig.Type {
	case datastore.HMacVerifier:
		v = verifier.NewHmacVerifier(
			verifierConfig.HMac.Header,
			verifierConfig.HMac.Hash,
			verifierConfig.HMac.Secret,
			string(verifierConfig.HMac.Encoding),
		)
	case datastore.BasicAuthVerifier:
		v = verifier.NewBasicAuthVerifier(
			verifierConfig.BasicAuth.UserName,
			verifierConfig.BasicAuth.Password,
		)
	case datastore.APIKeyVerifier:
		v = verifier.NewAPIKeyVerifier(
			verifierConfig.ApiKey.APIKey,
			verifierConfig.ApiKey.APIKeyHeader,
		)
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
	g, err := a.groupRepo.FetchGroupByID(r.Context(), source.GroupID)
	if err != nil {
		log.Errorf("Error occurred retrieving group")
		return
	}

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

	taskName := convoy.CreateEventProcessor.SetPrefix(g.Name)
	job := &queue.Job{
		ID:    event.UID,
		Event: event,
	}
	err = a.createEventQueue.Publish(r.Context(), taskName, job, 0)
	if err != nil {
		log.Errorf("Error occurred sending new event to the queue %s", err)
	}

	// 4. Return 200
	_ = render.Render(w, r, newServerResponse("Event received", nil, http.StatusOK))
}
