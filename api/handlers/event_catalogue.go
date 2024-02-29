package handlers

import (
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"

	"github.com/go-chi/chi/v5"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/api/models"

	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
)

// AddEventToCatalogue
//
//	@Summary		Adds an event to the event catalogue for the current project.
//	@Description	This endpoint Adds an event to the event catalogue for the current project.
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			event		body		models.AddEventToCatalogue	true	"Event Details"
//	@Success		200			{object}	util.ServerResponse{data=datastore.EventCatalogue}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/catalogue/add_event [post]
func (h *Handler) AddEventToCatalogue(w http.ResponseWriter, r *http.Request) {
	var catalogueEvent models.AddEventToCatalogue
	err := util.ReadJSON(r, &catalogueEvent)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = catalogueEvent.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	adc := services.AddEventToCatalogueService{
		CatalogueRepo:  postgres.NewEventCatalogueRepo(h.A.DB, h.A.Cache),
		EventRepo:      postgres.NewEventRepo(h.A.DB, h.A.Cache),
		CatalogueEvent: &catalogueEvent,
		Project:        project,
	}

	catalogue, err := adc.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Event added to catalogue successfully", catalogue, http.StatusOK))
}

// CreateOpenAPISpecCatalogue
//
//	@Summary		Creates an event catalogue for the current project using an openapi spec.
//	@Description	This endpoint Creates an event catalogue for the current project using an openapi spec.
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			event		body		models.CatalogueOpenAPISpec	true	"Event Details"
//	@Success		201			{object}	util.ServerResponse{data=datastore.EventCatalogue}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/catalogue/add_openapi_spec [post]
func (h *Handler) CreateOpenAPISpecCatalogue(w http.ResponseWriter, r *http.Request) {
	var catalogueOpenAPISpec models.CatalogueOpenAPISpec
	err := util.ReadJSON(r, &catalogueOpenAPISpec)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	err = catalogueOpenAPISpec.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	catalogue := &datastore.EventCatalogue{
		UID:         ulid.Make().String(),
		ProjectID:   project.UID,
		Type:        datastore.OpenAPICatalogueType,
		OpenAPISpec: catalogueOpenAPISpec.OpenAPISpec,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err = postgres.NewEventCatalogueRepo(h.A.DB, h.A.Cache).CreateEventCatalogue(r.Context(), catalogue)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Openapi spec catalogue created successfully", catalogue, http.StatusCreated))
}

// UpdateCatalogue
//
//	@Summary		Adds an openapi spec to the event catalogue for the current project.
//	@Description	This endpoint Adds an openapi spec to the event catalogue for the current project.
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			catalogueID	path		string					true	"Catalogue ID"
//	@Param			event		body		models.UpdateCatalogue	true	"Event Details"
//	@Success		200			{object}	util.ServerResponse{data=datastore.EventCatalogue}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/catalogue/{catalogueID} [put]
func (h *Handler) UpdateCatalogue(w http.ResponseWriter, r *http.Request) {
	var updateCatalogue models.UpdateCatalogue
	err := util.ReadJSON(r, &updateCatalogue)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	uc := services.UpdateCatalogueService{
		CatalogueRepo:   postgres.NewEventCatalogueRepo(h.A.DB, h.A.Cache),
		UpdateCatalogue: &updateCatalogue,
		Project:         project,
	}

	catalogue, err := uc.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Catalogue updated successfully", catalogue, http.StatusOK))
}

// DeleteCatalogue
//
//	@Summary		Deletes the event catalogue for the current project.
//	@Description	This endpoint Deletes the event catalogue for the current project
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			catalogueID	path		string	true	"Catalogue ID"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/catalogue/{catalogueID} [delete]
func (h *Handler) DeleteCatalogue(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	catalogueRepo := postgres.NewEventCatalogueRepo(h.A.DB, h.A.Cache)
	err = catalogueRepo.DeleteEventCatalogue(r.Context(), chi.URLParam(r, "catalogueID"), project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Catalogue deleted successfully", nil, http.StatusOK))
}

// GetCatalogue
//
//	@Summary		Fetches the event catalogue for the current project.
//	@Description	This endpoint Fetches the event catalogue for the current project
//	@Tags			Events
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Success		200			{object}	util.ServerResponse{data=datastore.EventCatalogue}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/catalogue [get]
func (h *Handler) GetCatalogue(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	catalogueRepo := postgres.NewEventCatalogueRepo(h.A.DB, h.A.Cache)
	catalogue, err := catalogueRepo.FindEventCatalogueByProjectID(r.Context(), project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Catalogue fetched successfully", catalogue, http.StatusOK))
}
