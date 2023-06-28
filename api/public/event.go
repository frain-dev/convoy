package public

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

// CreateEndpointEvent
// @Summary Create an event
// @Description This endpoint creates an endpoint event
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param event body models.CreateEvent true "Event Details"
// @Success 200 {object} util.ServerResponse{data=models.EventResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/events [post]
func (a *PublicHandler) CreateEndpointEvent(w http.ResponseWriter, r *http.Request) {
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
	if event.IsDuplicateEvent {
		_ = render.Render(w, r, util.NewServerResponse("Duplicate event received, but will not be sent", resp, http.StatusCreated))
	} else {
		_ = render.Render(w, r, util.NewServerResponse("Endpoint event created successfully", resp, http.StatusCreated))
	}
}

// CreateEndpointFanoutEvent
// @Summary Fan out an event
// @Description This endpoint uses the owner_id to fan out an event to multiple endpoints.
// @Tags Events
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param event body models.FanoutEvent true "Event Details"
// @Success 200 {object} util.ServerResponse{data=models.EventResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/events/fanout [post]
func (a *PublicHandler) CreateEndpointFanoutEvent(w http.ResponseWriter, r *http.Request) {
	var newMessage models.FanoutEvent
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

	cf := services.CreateFanoutEventService{
		EndpointRepo:   postgres.NewEndpointRepo(a.A.DB),
		EventRepo:      postgres.NewEventRepo(a.A.DB),
		PortalLinkRepo: postgres.NewPortalLinkRepo(a.A.DB),
		Queue:          a.A.Queue,
		NewMessage:     &newMessage,
		Project:        project,
	}

	event, err := cf.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EventResponse{Event: event}
	if event.IsDuplicateEvent {
		_ = render.Render(w, r, util.NewServerResponse("Duplicate event received, but will not be sent", resp, http.StatusCreated))
	} else {
		_ = render.Render(w, r, util.NewServerResponse("Endpoint event created successfully", resp, http.StatusCreated))
	}
}

// CreateDynamicEvent
// @Summary Dynamic Events
// @Description This endpoint does not require creating endpoint and subscriptions ahead of time. Instead, you supply the endpoint and the payload, and Convoy delivers the events
// @Tags Events
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param event body models.DynamicEvent true "Event Details"
// @Success 200 {object} Stub
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/events/dynamic [post]
func (a *PublicHandler) CreateDynamicEvent(w http.ResponseWriter, r *http.Request) {
	var newMessage models.DynamicEvent
	err := util.ReadJSON(r, &newMessage)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	err = newMessage.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	cde := services.CreateDynamicEventService{
		Queue:        a.A.Queue,
		DynamicEvent: &newMessage,
		Project:      project,
	}

	err = cde.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Dynamic event created successfully", nil, http.StatusCreated))
}

// ReplayEndpointEvent
// @Summary Replay event
// @Description This endpoint replays an event afresh assuming it is a new event.
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param eventID path string true "event id"
// @Success 200 {object} util.ServerResponse{data=models.EventResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/events/{eventID}/replay [put]
func (a *PublicHandler) ReplayEndpointEvent(w http.ResponseWriter, r *http.Request) {
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

// BatchReplayEvents
// @Summary Batch replay events
// @Description This endpoint replays multiple events at once.
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param request query models.QueryBatchReplayEventResponse false "Query Params"
// @Success 200 {object} util.ServerResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/events/batchreplay [post]
func (a *PublicHandler) BatchReplayEvents(w http.ResponseWriter, r *http.Request) {
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

// GetEndpointEvent
// @Summary Retrieve an event
// @Description This endpoint retrieves an event
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param eventID path string true "event id"
// @Success 200 {object} util.ServerResponse{data=models.EventResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/events/{eventID} [get]
func (a *PublicHandler) GetEndpointEvent(w http.ResponseWriter, r *http.Request) {
	event, err := a.retrieveEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.EventResponse{Event: event}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint event fetched successfully",
		resp, http.StatusOK))
}

// GetEventDelivery
// @Summary Retrieve an event delivery
// @Description This endpoint fetches an event delivery.
// @Tags Event Deliveries
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} util.ServerResponse{data=models.EventDeliveryResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/eventdeliveries/{eventDeliveryID} [get]
func (a *PublicHandler) GetEventDelivery(w http.ResponseWriter, r *http.Request) {
	eventDelivery, err := a.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.EventDeliveryResponse{EventDelivery: eventDelivery}
	_ = render.Render(w, r, util.NewServerResponse("Event Delivery fetched successfully",
		resp, http.StatusOK))
}

// ResendEventDelivery
// @Summary Retry event delivery
// @Description This endpoint retries an event delivery.
// @Tags Event Deliveries
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} util.ServerResponse{data=models.EventDeliveryResponse}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/resend [put]
func (a *PublicHandler) ResendEventDelivery(w http.ResponseWriter, r *http.Request) {
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

// BatchRetryEventDelivery
// @Summary Batch retry event delivery
// @Description This endpoint batch retries multiple event deliveries at once.
// @Tags Event Deliveries
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param request query models.QueryBatchRetryEventDelivery false "Query Params"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/eventdeliveries/batchretry [post]
func (a *PublicHandler) BatchRetryEventDelivery(w http.ResponseWriter, r *http.Request) {
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

// ForceResendEventDeliveries
// @Summary Force retry event delivery
// @Description This endpoint enables you retry a previously successful event delivery
// @Tags Event Deliveries
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param deliveryIds body Stub{ids=models.IDs} true "event delivery ids"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/eventdeliveries/forceresend [post]
func (a *PublicHandler) ForceResendEventDeliveries(w http.ResponseWriter, r *http.Request) {
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

// GetEventsPaged
// @Summary List all events
// @Description This endpoint fetches app events with pagination
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param request query models.QueryListEvent false "Query Params"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.EventResponse}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/events [get]
func (a *PublicHandler) GetEventsPaged(w http.ResponseWriter, r *http.Request) {
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

		events, paginationData, err := se.Run(r.Context())
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		resp := models.NewListResponse(events, func(event datastore.Event) models.EventResponse {
			return models.EventResponse{Event: &event}
		})
		_ = render.Render(w, r, util.NewServerResponse("Endpoint events fetched successfully",
			pagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))

		return
	}

	eventsPaged, paginationData, err := postgres.NewEventRepo(a.A.DB).LoadEventsPaged(r.Context(), project.UID, data.Filter)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch events")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
		return
	}

	resp := models.NewListResponse(eventsPaged, func(event datastore.Event) models.EventResponse {
		return models.EventResponse{Event: &event}
	})
	_ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
		pagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))
}

// GetEventDeliveriesPaged
// @Summary List all event deliveries
// @Description This endpoint retrieves all event deliveries paginated.
// @Tags Event Deliveries
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param request query models.QueryListEventDelivery false "Query Params"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]models.EventDeliveryResponse}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /v1/projects/{projectID}/eventdeliveries [get]
func (a *PublicHandler) GetEventDeliveriesPaged(w http.ResponseWriter, r *http.Request) {
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

	// if the idempotency key query is set, find the first event with the key
	if len(data.IdempotencyKey) > 0 {
		event, err := postgres.NewEventRepo(a.A.DB).FindFirstEventWithIdempotencyKey(r.Context(), project.UID, data.IdempotencyKey)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
		data.EventID = event.UID
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

func (a *PublicHandler) retrieveEvent(r *http.Request) (*datastore.Event, error) {
	project, err := a.retrieveProject(r)
	if err != nil {
		return &datastore.Event{}, err
	}

	eventID := chi.URLParam(r, "eventID")
	eventRepo := postgres.NewEventRepo(a.A.DB)
	return eventRepo.FindEventByID(r.Context(), project.UID, eventID)
}

func (a *PublicHandler) retrieveEventDelivery(r *http.Request) (*datastore.EventDelivery, error) {
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
