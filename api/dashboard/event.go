package dashboard

import (
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func (a *DashboardHandler) CreateEndpointEvent(w http.ResponseWriter, r *http.Request) {
	var newMessage models.CreateEvent
	err := util.ReadJSON(r, &newMessage)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = newMessage.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	ce := services.CreateEventService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		EventRepo:    postgres.NewEventRepo(a.A.DB),
		Queue:        a.A.Queue,
		NewMessage:   &newMessage,
		Project:      project,
	}

	event, err := ce.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EventResponse{Event: event}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint event created successfully", resp, http.StatusCreated))
}

func (a *DashboardHandler) ReplayEndpointEvent(w http.ResponseWriter, r *http.Request) {
	event, err := a.retrieveEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	rs := services.ReplayEventService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		Queue:        a.A.Queue,
		Event:        event,
	}

	err = rs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EventResponse{Event: event}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint event replayed successfully", resp, http.StatusOK))
}

func (a *DashboardHandler) BatchReplayEvents(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryBatchReplayEvent
	p, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	data.Filter.Project = p

	bs := services.BatchReplayEventService{
		EndpointRepo: postgres.NewEndpointRepo(a.A.DB),
		Queue:        a.A.Queue,
		EventRepo:    postgres.NewEventRepo(a.A.DB),
		Filter:       data.Filter,
	}

	successes, failures, err := bs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
}

func (a *DashboardHandler) CountAffectedEvents(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryCountAffectedEvents
	p, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	count, err := postgres.NewEventRepo(a.A.DB).CountEvents(r.Context(), p.UID, data.Filter)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching event")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("events count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

func (a *DashboardHandler) GetEndpointEvent(w http.ResponseWriter, r *http.Request) {
	event, err := a.retrieveEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.EventResponse{Event: event}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint event fetched successfully", resp, http.StatusOK))
}

func (a *DashboardHandler) GetEventDelivery(w http.ResponseWriter, r *http.Request) {
	eventDelivery, err := a.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.EventDeliveryResponse{EventDelivery: eventDelivery}
	_ = render.Render(w, r, util.NewServerResponse("Event Delivery fetched successfully", resp, http.StatusOK))
}

func (a *DashboardHandler) ResendEventDelivery(w http.ResponseWriter, r *http.Request) {
	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	eventDelivery, err := a.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fr := services.RetryEventDeliveryService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(a.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(a.A.DB),
		Queue:             a.A.Queue,
		EventDelivery:     eventDelivery,
		Project:           project,
	}

	err = fr.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EventDeliveryResponse{EventDelivery: eventDelivery}
	_ = render.Render(w, r, util.NewServerResponse("App event processed for retry successfully",
		resp, http.StatusOK))
}

func (a *DashboardHandler) BatchRetryEventDelivery(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryBatchRetryEventDelivery

	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	data.Filter.Project = project

	br := services.BatchRetryEventDeliveryService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(a.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(a.A.DB),
		Queue:             a.A.Queue,
		EventRepo:         postgres.NewEventRepo(a.A.DB),
		Filter:            data.Filter,
	}

	successes, failures, err := br.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
}

func (a *DashboardHandler) CountAffectedEventDeliveries(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryCountAffectedEventDeliveries

	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	f := data.Filter
	count, err := postgres.NewEventDeliveryRepo(a.A.DB).CountEventDeliveries(r.Context(), project.UID, f.EndpointIDs, f.EventID, f.Status, f.SearchParams)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching event deliveries")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("event deliveries count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

func (a *DashboardHandler) ForceResendEventDeliveries(w http.ResponseWriter, r *http.Request) {
	eventDeliveryIDs := models.IDs{}

	err := json.NewDecoder(r.Body).Decode(&eventDeliveryIDs)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fr := services.ForceResendEventDeliveriesService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(a.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(a.A.DB),
		Queue:             a.A.Queue,
		IDs:               eventDeliveryIDs.IDs,
		Project:           project,
	}

	successes, failures, err := fr.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
}

func (a *DashboardHandler) GetEventsPaged(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEvent
	cfg, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	data.Filter.Project = project

	if cfg.Search.Type == config.TypesenseSearchProvider && !util.IsStringEmpty(data.Filter.Query) {
		searchBackend, err := searcher.NewSearchClient(cfg)
		if err != nil {
			log.FromContext(r.Context()).WithError(err).Error("failed to initialise search backend")
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		se := services.SearchEventService{
			EventRepo: postgres.NewEventRepo(a.A.DB),
			Searcher:  searchBackend,
			Filter:    data.Filter,
		}

		m, paginationData, err := se.Run(r.Context())
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		resp := models.NewListResponse(m, func(event datastore.Event) models.EventResponse {
			return models.EventResponse{Event: &event}
		})
		_ = render.Render(w, r, util.NewServerResponse("Endpoint events fetched successfully",
			pagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))

		return
	}

	m, paginationData, err := postgres.NewEventRepo(a.A.DB).LoadEventsPaged(r.Context(), project.UID, data.Filter)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch events")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
		return
	}

	resp := models.NewListResponse(m, func(event datastore.Event) models.EventResponse {
		return models.EventResponse{Event: &event}
	})
	_ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
		pagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))
}

func (a *DashboardHandler) GetEventDeliveriesPaged(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEventDelivery

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	f := data.Filter
	ed, paginationData, err := postgres.NewEventDeliveryRepo(a.A.DB).LoadEventDeliveriesPaged(r.Context(), project.UID, f.EndpointIDs, f.EventID, f.Status, f.SearchParams, f.Pageable, f.IdempotencyKey)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch event deliveries")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
		return
	}

	resp := models.NewListResponse(ed, func(ed datastore.EventDelivery) models.EventDeliveryResponse {
		return models.EventDeliveryResponse{EventDelivery: &ed}
	})
	_ = render.Render(w, r, util.NewServerResponse("Event deliveries fetched successfully",
		pagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))
}

func (a *DashboardHandler) retrieveEvent(r *http.Request) (*datastore.Event, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.Event{}, err
	}

	eventID := chi.URLParam(r, "eventID")
	eventRepo := postgres.NewEventRepo(a.A.DB)
	return eventRepo.FindEventByID(r.Context(), project.UID, eventID)
}

func (a *DashboardHandler) retrieveEventDelivery(r *http.Request) (*datastore.EventDelivery, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.EventDelivery{}, err
	}

	eventDeliveryID := chi.URLParam(r, "eventDeliveryID")
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB)
	return eventDeliveryRepo.FindEventDeliveryByID(r.Context(), project.UID, eventDeliveryID)
}

func getEndpointIDs(r *http.Request) []string {
	var endpoints []string

	for _, id := range r.URL.Query()["endpointId"] {
		if !util.IsStringEmpty(id) {
			endpoints = append(endpoints, id)
		}
	}

	return endpoints
}
