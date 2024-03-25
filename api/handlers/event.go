package handlers

import (
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// CreateEndpointEvent
//
//	@Summary		Create an event
//	@Description	This endpoint creates an endpoint event
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			event		body		models.CreateEvent	true	"Event Details"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/events [post]
func (h *Handler) CreateEndpointEvent(w http.ResponseWriter, r *http.Request) {
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

	var projectID string
	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		project, err := h.retrieveProject(r)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}

		projectID = project.UID

	} else {
		projectID = chi.URLParam(r, "projectID")
		if util.IsStringEmpty(projectID) {
			_ = render.Render(w, r, util.NewErrorResponse("project id not present in request", http.StatusBadRequest))
			return
		}
	}

	e := task.CreateEvent{
		Params: task.CreateEventTaskParams{
			UID:            ulid.Make().String(),
			ProjectID:      projectID,
			EndpointID:     newMessage.EndpointID,
			EventType:      newMessage.EventType,
			Data:           newMessage.Data,
			CustomHeaders:  newMessage.CustomHeaders,
			IdempotencyKey: newMessage.IdempotencyKey,
		},
		CreateSubscription: !util.IsStringEmpty(newMessage.EndpointID),
	}

	eventByte, err := msgpack.EncodeMsgPack(e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	job := &queue.Job{
		ID:      newMessage.UID,
		Payload: eventByte,
		Delay:   0,
	}

	err = h.A.Queue.Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, job)
	if err != nil {
		log.FromContext(r.Context()).Errorf("Error occurred sending new event to the queue %s", err)
	}

	_ = render.Render(w, r, util.NewServerResponse("Event queued successfully", 200, http.StatusCreated))
}

// CreateBroadcastEvent
//
//	@Summary		Create a broadcast event
//	@Description	This endpoint creates a event that is broadcast to every endpoint whose subscription matches the given event type.
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			event		body		models.BroadcastEvent	true	"Broadcast Event Details"
//	@Success		200			{object}	util.ServerResponse{data=models.EventResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/events/broadcast [post]
func (h *Handler) CreateBroadcastEvent(w http.ResponseWriter, r *http.Request) {
	var newMessage models.BroadcastEvent
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

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	cbe := services.CreateBroadcastEventService{
		Queue:          h.A.Queue,
		BroadcastEvent: &newMessage,
		Project:        project,
	}

	err = cbe.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Broadcast event created successfully", nil, http.StatusCreated))
}

// CreateEndpointFanoutEvent
//
//	@Summary		Fan out an event
//	@Description	This endpoint uses the owner_id to fan out an event to multiple endpoints.
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			event		body		models.FanoutEvent	true	"Event Details"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/events/fanout [post]
func (h *Handler) CreateEndpointFanoutEvent(w http.ResponseWriter, r *http.Request) {
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

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	cf := services.CreateFanoutEventService{
		EndpointRepo:   postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		EventRepo:      postgres.NewEventRepo(h.A.DB, h.A.Cache),
		PortalLinkRepo: postgres.NewPortalLinkRepo(h.A.DB, h.A.Cache),
		Queue:          h.A.Queue,
		NewMessage:     &newMessage,
		Project:        project,
	}

	event, err := cf.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if event.IsDuplicateEvent {
		_ = render.Render(w, r, util.NewServerResponse("Duplicate event received, but will not be sent", nil, http.StatusCreated))
	} else {
		_ = render.Render(w, r, util.NewServerResponse("Endpoint fanout event queued successfully", nil, http.StatusCreated))
	}
}

// CreateDynamicEvent
//
//	@Summary		Dynamic Events
//	@Description	This endpoint does not require creating endpoint and subscriptions ahead of time. Instead, you supply the endpoint and the payload, and Convoy delivers the events
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			event		body		models.DynamicEvent	true	"Event Details"
//	@Success		200			{object}	Stub
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/events/dynamic [post]
func (h *Handler) CreateDynamicEvent(w http.ResponseWriter, r *http.Request) {
	var newMessage models.DynamicEvent
	err := util.ReadJSON(r, &newMessage)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
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
		Queue:        h.A.Queue,
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
//
//	@Summary		Replay event
//	@Description	This endpoint replays an event afresh assuming it is a new event.
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			eventID		path		string	true	"event id"
//	@Success		200			{object}	util.ServerResponse{data=models.EventResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/events/{eventID}/replay [put]
func (h *Handler) ReplayEndpointEvent(w http.ResponseWriter, r *http.Request) {
	event, err := h.retrieveEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	rs := services.ReplayEventService{
		EndpointRepo: postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		Queue:        h.A.Queue,
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
//
//	@Summary		Batch replay events
//	@Description	This endpoint replays multiple events at once.
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			request		query		models.QueryListEvent	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=datastore.Event{data=Stub}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/events/batchreplay [post]
func (h *Handler) BatchReplayEvents(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEvent
	p, err := h.retrieveProject(r)
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
		EndpointRepo: postgres.NewEndpointRepo(h.A.DB, h.A.Cache),
		Queue:        h.A.Queue,
		EventRepo:    postgres.NewEventRepo(h.A.DB, h.A.Cache),
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
//
//	@Summary		Retrieve an event
//	@Description	This endpoint retrieves an event
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			eventID		path		string	true	"event id"
//	@Success		200			{object}	util.ServerResponse{data=models.EventResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/events/{eventID} [get]
func (h *Handler) GetEndpointEvent(w http.ResponseWriter, r *http.Request) {
	event, err := h.retrieveEvent(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.EventResponse{Event: event}
	_ = render.Render(w, r, util.NewServerResponse("Endpoint event fetched successfully",
		resp, http.StatusOK))
}

// GetEventsPaged
//
//	@Summary		List all events
//	@Description	This endpoint fetches app events with pagination
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			request		query		models.QueryListEvent	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=models.PagedResponse{content=[]models.EventResponse}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/events [get]
func (h *Handler) GetEventsPaged(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEvent
	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		portalLink, err := h.retrievePortalLinkFromToken(r)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}

		endpointIDs, err := h.getEndpoints(r, portalLink)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}

		if len(endpointIDs) == 0 {
			_ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
				models.PagedResponse{Content: endpointIDs, Pagination: &datastore.PaginationData{PerPage: int64(data.Filter.Pageable.PerPage)}}, http.StatusOK))
			return
		}

		data.Filter.EndpointIDs = endpointIDs
	}

	data.Filter.Project = project
	eventsPaged, paginationData, err := postgres.NewEventRepo(h.A.DB, h.A.Cache).LoadEventsPaged(r.Context(), project.UID, data.Filter)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch events")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
		return
	}

	resp := models.NewListResponse(eventsPaged, func(event datastore.Event) models.EventResponse {
		return models.EventResponse{Event: &event}
	})
	_ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
		models.PagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))
}

func (h *Handler) CountAffectedEvents(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEvent
	p, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	data, err := q.Transform(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	count, err := postgres.NewEventRepo(h.A.DB, h.A.Cache).CountEvents(r.Context(), p.UID, data.Filter)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching event")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("events count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

func (h *Handler) retrieveEvent(r *http.Request) (*datastore.Event, error) {
	project, err := h.retrieveProject(r)
	if err != nil {
		return &datastore.Event{}, err
	}

	eventID := chi.URLParam(r, "eventID")
	eventRepo := postgres.NewEventRepo(h.A.DB, h.A.Cache)
	return eventRepo.FindEventByID(r.Context(), project.UID, eventID)
}
