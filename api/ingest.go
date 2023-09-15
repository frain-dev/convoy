package api

import (
	"context"
	"encoding/json"
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
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"
)

func createSourceService(a *ApplicationHandler) *services.SourceService {
	sourceRepo := postgres.NewSourceRepo(a.A.DB)

	return services.NewSourceService(sourceRepo, a.A.Cache)
}

func retrieveSourceConfigurationFromMaskId(a *ApplicationHandler, ctx context.Context, maskId string) (*datastore.Source, error) {
	var source *datastore.Source
	var err error

	// Tries to retrieve the source configuration from the cache service
	err = a.A.Cache.Get(ctx, maskId, &source)

	if err != nil {
		a.A.Logger.WithError(err)
	}

	if source == nil {
		// 2. Retrieve source using mask ID.
		source, err = postgres.NewSourceRepo(a.A.DB).FindSourceByMaskID(ctx, maskId)
		if err != nil {
			return nil, err
		}
		err = a.A.Cache.Set(ctx, maskId, &source, time.Minute*2)
		if err != nil {
			a.A.Logger.WithError(err)
		}
	}
	return source, nil
}

func (a *ApplicationHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	// s.AppService.CountProjectApplications()
	// 1. Retrieve mask ID
	maskID := chi.URLParam(r, "maskID")

	source, err := retrieveSourceConfigurationFromMaskId(a, r.Context(), maskID)

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

	// passivel de cache
	var project *datastore.Project

	a.A.Cache.Get(r.Context(), source.ProjectID, &project)

	if project == nil {
		// 2. Retrieve source using mask ID.
		projectRepo := postgres.NewProjectRepo(a.A.DB)
		projectFromDb, err := projectRepo.FetchProjectByID(r.Context(), source.ProjectID)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}
		a.A.Cache.Set(r.Context(), source.ProjectID, &projectFromDb, time.Minute*2)
		project = projectFromDb
	}

	var maxIngestSize uint64
	if project.Config != nil && project.Config.MaxIngestSize == 0 {
		maxIngestSize = project.Config.MaxIngestSize
	}

	if maxIngestSize == 0 {
		cfg, err := config.Get()
		if err != nil {
			a.A.Logger.WithError(err).Error("failed to load config")
			_ = render.Render(w, r, util.NewErrorResponse("failed to load config", http.StatusBadRequest))
			return
		}

		maxIngestSize = cfg.MaxResponseSize
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
		UID:       ulid.Make().String(),
		EventType: datastore.EventType(maskID),
		SourceID:  source.UID,
		ProjectID: source.ProjectID,
		Raw:       string(payload),
		Data:      payload,
		Headers:   httpheader.HTTPHeader(r.Header),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
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
	_ = render.Render(w, r, util.NewServerResponse("Event received", len(payload), http.StatusOK))
}

func (a *ApplicationHandler) HandleCrcCheck(w http.ResponseWriter, r *http.Request) {
	maskID := chi.URLParam(r, "maskID")
	sourceCacheKey := convoy.SourceCacheKey.Get(maskID).String()

	source, err := retrieveSourceConfigurationFromMaskId(a, r.Context(), sourceCacheKey)

	if err != nil {
		if err == datastore.ErrSourceNotFound {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("error retrieving source", http.StatusBadRequest))
		return
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
