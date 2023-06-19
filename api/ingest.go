package api

import (
	"encoding/json"
	"github.com/frain-dev/convoy/internal/pkg/dedup"
	"io"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/crc"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/verifier"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"
)

func (a *ApplicationHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	// s.AppService.CountProjectApplications()
	// 1. Retrieve mask ID
	maskID := chi.URLParam(r, "maskID")

	// 2. Retrieve source using mask ID.
	source, err := postgres.NewSourceRepo(a.A.DB).FindSourceByMaskID(r.Context(), maskID)
	if err != nil {
		if err == datastore.ErrSourceNotFound {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving source", http.StatusBadRequest))
		return
	}

	if source.Type != datastore.HTTPSource {
		_ = render.Render(w, r, util.NewErrorResponse("Source type needs to be HTTP",
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
		case datastore.ShopifySourceProvider:
			v = verifier.NewShopifyVerifier(verifierConfig.HMac.Secret)
		default:
			_ = render.Render(w, r, util.NewErrorResponse("Provider type undefined",
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
			v = &verifier.NoopVerifier{}
		}
	}

	projectRepo := postgres.NewProjectRepo(a.A.DB)

	g, err := projectRepo.FetchProjectByID(r.Context(), source.ProjectID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	maxIngestSize := g.Config.MaxIngestSize
	if maxIngestSize == 0 {
		cfg, err := config.Get()
		if err != nil {
			a.A.Logger.WithError(err).Error("failed to load config")
			_ = render.Render(w, r, util.NewErrorResponse("failed to load config", http.StatusBadRequest))
			return
		}

		maxIngestSize = cfg.MaxResponseSize
	}

	var checksum string
	if len(source.IdempotencyKeys) > 0 {
		duper := dedup.NewDeDuper(r.Context(), r, postgres.NewEventRepo(a.A.DB))
		exists, err := duper.Exists(source.Name, source.ProjectID, source.IdempotencyKeys)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		if exists {
			_ = render.Render(w, r, util.NewErrorResponse("duplicate event will not be ingested", http.StatusBadRequest))
			return
		}

		checksum, err = duper.GenerateChecksum(source.Name, source.IdempotencyKeys)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
	}

	// 3.1 On Failure
	// Return 400 Bad Request.
	body := io.LimitReader(r.Body, int64(maxIngestSize))
	payload, err := io.ReadAll(body)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err = v.VerifyRequest(r, payload); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if len(payload) == 0 {
		payload = []byte("{}")
	}

	// 3.2 On success
	// Attach Source to Event.
	// Write Event to the Ingestion Queue.
	event := &datastore.Event{
		UID:            ulid.Make().String(),
		EventType:      datastore.EventType(maskID),
		SourceID:       source.UID,
		ProjectID:      source.ProjectID,
		Raw:            string(payload),
		Data:           payload,
		URLQueryParams: r.URL.RawQuery,
		IdempotencyKey: checksum,
		Headers:        httpheader.HTTPHeader(r.Header),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	createEvent := task.CreateEvent{
		Event: *event,
	}

	eventByte, err := json.Marshal(createEvent)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	job := &queue.Job{
		ID:      event.UID,
		Payload: eventByte,
		Delay:   0,
	}

	err = a.A.Queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		a.A.Logger.WithError(err).Error("Error occurred sending new event to the queue")
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// 4. Return 200
	if !util.IsStringEmpty(source.CustomResponse.Body) {
		// send back custom response
		if !util.IsStringEmpty(source.CustomResponse.ContentType) {
			w.Header().Set("Content-Type", source.CustomResponse.ContentType)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(source.CustomResponse.Body))
			return
		}

		render.Status(r, http.StatusOK)
		render.PlainText(w, r, source.CustomResponse.Body)
		return

	}
	_ = render.Render(w, r, util.NewServerResponse("Event received", len(payload), http.StatusOK))
}

func (a *ApplicationHandler) HandleCrcCheck(w http.ResponseWriter, r *http.Request) {
	maskID := chi.URLParam(r, "maskID")

	var source *datastore.Source
	sourceCacheKey := convoy.SourceCacheKey.Get(maskID).String()

	err := a.A.Cache.Get(r.Context(), sourceCacheKey, &source)
	if err != nil {
		a.A.Logger.WithError(err)
	}

	if source == nil {
		source, err = postgres.NewSourceRepo(a.A.DB).FindSourceByMaskID(r.Context(), maskID)
		if err != nil {
			if err == datastore.ErrSourceNotFound {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
				return
			}

			_ = render.Render(w, r, util.NewErrorResponse("error retrieving source", http.StatusBadRequest))
			return
		}

		err = a.A.Cache.Set(r.Context(), sourceCacheKey, &source, time.Hour*24)
		if err != nil {
			a.A.Logger.WithError(err)
		}

	}

	if source.Type != datastore.HTTPSource {
		_ = render.Render(w, r, util.NewErrorResponse("Source type needs to be HTTP", http.StatusBadRequest))
		return
	}

	if util.IsStringEmpty(string(source.Provider)) {
		_ = render.Render(w, r, util.NewErrorResponse("Provider type undefined", http.StatusBadRequest))
		return
	}

	var c crc.Crc

	switch source.Provider {
	case datastore.TwitterSourceProvider:
		c = crc.NewTwitterCrc(source.Verifier.HMac.Secret)
	default:
		_ = render.Render(w, r, util.NewErrorResponse("Provider type is not supported", http.StatusBadRequest))
		return
	}

	sourceRepo := postgres.NewSourceRepo(a.A.DB)
	err = c.HandleRequest(w, r, source, sourceRepo)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
}
