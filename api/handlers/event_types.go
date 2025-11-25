package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
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
//	@Success		200			{object}	util.ServerResponse{data=[]models.EventTypeResponse}
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

	resp := models.NewListResponse(eventTypes, func(eventType datastore.ProjectEventType) models.EventTypeResponse {
		return models.EventTypeResponse{ProjectEventType: &eventType}
	})
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
		UID:         ulid.Make().String(),
		Category:    newEventType.Category,
		Description: newEventType.Description,
	}

	b, err2 := json.Marshal(newEventType.JSONSchema)
	if err2 != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err2.Error(), http.StatusBadRequest))
	}
	pe.JSONSchema = b

	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)
	err = eventTypeRepo.CreateEventType(r.Context(), pe)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp := &models.EventTypeResponse{ProjectEventType: pe}
	_ = render.Render(w, r, util.NewServerResponse("Event type created successfully", resp, http.StatusCreated))
}

// UpdateEventType
//
//	@Summary		Updates an event type
//	@Description	This endpoint updates an event type
//	@Id				UpdateEventType
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

	if ue.JSONSchema != nil {
		b, err2 := json.Marshal(ue.JSONSchema)
		if err2 != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err2.Error(), http.StatusBadRequest))
		}
		pe.JSONSchema = b
	}

	err = eventTypeRepo.UpdateEventType(r.Context(), pe)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp := &models.EventTypeResponse{ProjectEventType: pe}
	_ = render.Render(w, r, util.NewServerResponse("Event type created successfully", resp, http.StatusAccepted))
}

// DeprecateEventType
//
//	@Summary		Deprecates an event type
//	@Description	This endpoint deprecates an event type
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

	resp := &models.EventTypeResponse{ProjectEventType: pe}
	_ = render.Render(w, r, util.NewServerResponse("Event type deprecated successfully", resp, http.StatusOK))
}

// ImportOpenApiSpec
//
//	@Summary		Import event types from OpenAPI spec
//	@Description	This endpoint imports event types from an OpenAPI specification
//	@Id				ImportOpenApiSpec
//	@Tags			EventTypes
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			spec		body		models.ImportOpenAPISpec	true	"OpenAPI specification"
//	@Success		200			{object}	util.ServerResponse{data=[]models.EventTypeResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/event-types/import [post]
func (h *Handler) ImportOpenApiSpec(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var body models.ImportOpenAPISpec
	err = util.ReadJSON(r, &body)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)
	importService, err := services.NewImportOpenapiSpecService(body.Spec, project.UID, eventTypeRepo)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	eventTypes, err := importService.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	resp := models.NewListResponse(eventTypes, func(eventType datastore.ProjectEventType) models.EventTypeResponse {
		return models.EventTypeResponse{ProjectEventType: &eventType}
	})
	_ = render.Render(w, r, util.NewServerResponse("Event types imported successfully", resp, http.StatusOK))
}
