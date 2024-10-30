package handlers

import (
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"
	"net/http"
)

// GetEventTypes
//
//	@Summary		Retrieves a project's event types
//	@Description	This endpoint fetches the project's event types
//	@Id				GetEventTypes
//	@Tags			EventTypes
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Success		200			{object}	util.ServerResponse{data=models.EventTypeListResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/event-types [get]
func (h *Handler) GetEventTypes(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)
	eventTypes, err := eventTypeRepo.FetchAllEventTypes(r.Context(), project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp := &models.EventTypeListResponse{EventTypes: eventTypes}
	_ = render.Render(w, r, util.NewServerResponse("Event types fetched successfully", resp, http.StatusOK))
}

// CreateEventType
//
//	@Summary		Create an event type
//	@Description	This endpoint creates an event type
//	@Id				CreateEventType
//	@Tags			EventTypes
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			eventType	body		models.CreateEventType	true	"Event Type Details"
//	@Success		201			{object}	util.ServerResponse{data=models.EventTypeResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/event-types [post]
func (h *Handler) CreateEventType(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var newEventType models.CreateEventType
	err = util.ReadJSON(r, &newEventType)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = newEventType.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	pe := &datastore.ProjectEventType{
		ProjectId:   project.UID,
		Name:        newEventType.Name,
		ID:          ulid.Make().String(),
		Category:    newEventType.Category,
		Description: newEventType.Description,
	}

	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)
	err = eventTypeRepo.CreateEventType(r.Context(), pe)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp := &models.EventTypeResponse{EventType: pe}
	_ = render.Render(w, r, util.NewServerResponse("Event type created successfully", resp, http.StatusCreated))
}

// UpdateEventType
//
//	@Summary		Updates an event type
//	@Description	This endpoint updates an event type
//	@Id				CreateEventType
//	@Tags			EventTypes
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			eventType	body		models.UpdateEventType	true	"Event Type Details"
//	@Success		201			{object}	util.ServerResponse{data=models.EventTypeResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/event-types/{eventTypeId} [put]
func (h *Handler) UpdateEventType(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	eventTypeId := chi.URLParam(r, "eventTypeId")

	var ue models.UpdateEventType
	err = util.ReadJSON(r, &ue)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)
	pe, err := eventTypeRepo.FetchEventTypeById(r.Context(), eventTypeId, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if !util.IsStringEmpty(ue.Description) {
		pe.Description = ue.Description
	}

	if !util.IsStringEmpty(ue.Category) {
		pe.Category = ue.Category
	}

	err = eventTypeRepo.UpdateEventType(r.Context(), pe)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp := &models.EventTypeResponse{EventType: pe}
	_ = render.Render(w, r, util.NewServerResponse("Event type created successfully", resp, http.StatusAccepted))
}

// DeprecateEventType
//
//	@Summary		Create an event type
//	@Description	This endpoint creates an event type
//	@Id				DeprecateEventType
//	@Tags			EventTypes
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			eventTypeId	path		string	true	"Event Type ID"
//	@Success		201			{object}	util.ServerResponse{data=models.EventTypeResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/event-types/{eventTypeId}/deprecate [post]
func (h *Handler) DeprecateEventType(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	eventTypeId := chi.URLParam(r, "eventTypeId")
	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)
	pe, err := eventTypeRepo.DeprecateEventType(r.Context(), eventTypeId, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp := &models.EventTypeResponse{EventType: pe}
	_ = render.Render(w, r, util.NewServerResponse("Event type deprecated successfully", resp, http.StatusOK))
}
