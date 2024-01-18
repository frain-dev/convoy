package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// CreateSource
//
//	@Summary		Create a source
//	@Description	This endpoint creates a source
//	@Tags			Sources
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			source		body		models.CreateSource	true	"Source Details"
//	@Success		200			{object}	util.ServerResponse{data=models.SourceResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/sources [post]
func (h *Handler) CreateSource(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var newSource models.CreateSource
	if err := util.ReadJSON(r, &newSource); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := newSource.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	cs := services.CreateSourceService{
		SourceRepo: postgres.NewSourceRepo(h.A.DB, h.A.Cache),
		Cache:      h.A.Cache,
		NewSource:  &newSource,
		Project:    project,
	}

	source, err := cs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org, err := postgres.NewOrgRepo(h.A.DB, h.A.Cache).FetchOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to find organisation by id")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fillSourceURL(source, baseUrl, org.CustomDomain.ValueOrZero())
	resp := &models.SourceResponse{Source: source}

	_ = render.Render(w, r, util.NewServerResponse("Source created successfully", resp, http.StatusCreated))
}

// GetSource
//
//	@Summary		Retrieve a source
//	@Description	This endpoint retrieves a source by its id
//	@Tags			Sources
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			sourceID	path		string	true	"Source ID"
//	@Success		200			{object}	util.ServerResponse{data=models.SourceResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/sources/{sourceID} [get]
func (h *Handler) GetSource(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	source, err := postgres.NewSourceRepo(h.A.DB, h.A.Cache).FindSourceByID(r.Context(), project.UID, chi.URLParam(r, "sourceID"))
	if err != nil {
		if errors.Is(err, datastore.ErrSourceNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving source", http.StatusBadRequest))
		return
	}

	org, err := postgres.NewOrgRepo(h.A.DB, h.A.Cache).FetchOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to find organisation by id")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fillSourceURL(source, baseUrl, org.CustomDomain.ValueOrZero())
	resp := &models.SourceResponse{Source: source}

	_ = render.Render(w, r, util.NewServerResponse("Source fetched successfully", resp, http.StatusOK))
}

// UpdateSource
//
//	@Summary		Update a source
//	@Description	This endpoint updates a source
//	@Tags			Sources
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			sourceID	path		string				true	"source id"
//	@Param			source		body		models.UpdateSource	true	"Source Details"
//	@Success		200			{object}	util.ServerResponse{data=models.SourceResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/sources/{sourceID} [put]
func (h *Handler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var sourceUpdate models.UpdateSource
	err = util.ReadJSON(r, &sourceUpdate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	if err := sourceUpdate.Validate(); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	source, err := postgres.NewSourceRepo(h.A.DB, h.A.Cache).FindSourceByID(r.Context(), project.UID, chi.URLParam(r, "sourceID"))
	if err != nil {
		if errors.Is(err, datastore.ErrSourceNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving source", http.StatusBadRequest))
		return
	}

	us := services.UpdateSourceService{
		SourceRepo:   postgres.NewSourceRepo(h.A.DB, h.A.Cache),
		Cache:        h.A.Cache,
		Project:      project,
		SourceUpdate: &sourceUpdate,
		Source:       source,
	}

	source, err = us.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	org, err := postgres.NewOrgRepo(h.A.DB, h.A.Cache).FetchOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to find organisation by id")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fillSourceURL(source, baseUrl, org.CustomDomain.ValueOrZero())
	resp := &models.SourceResponse{Source: source}

	_ = render.Render(w, r, util.NewServerResponse("Source updated successfully", resp, http.StatusAccepted))
}

// DeleteSource
//
//	@Summary		Delete a source
//	@Description	This endpoint deletes a source
//	@Tags			Sources
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			sourceID	path		string	true	"source id"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/sources/{sourceID} [delete]
func (h *Handler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	sourceRepo := postgres.NewSourceRepo(h.A.DB, h.A.Cache)

	source, err := sourceRepo.FindSourceByID(r.Context(), project.UID, chi.URLParam(r, "sourceID"))
	if err != nil {
		if errors.Is(err, datastore.ErrSourceNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
			return
		}

		_ = render.Render(w, r, util.NewErrorResponse("error retrieving source", http.StatusBadRequest))
		return
	}

	err = sourceRepo.DeleteSourceByID(r.Context(), project.UID, source.UID, source.VerifierID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete source", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Source deleted successfully", nil, http.StatusOK))
}

// LoadSourcesPaged
//
//	@Summary		List all sources
//	@Description	This endpoint fetches multiple sources
//	@Tags			Sources
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			request		query		models.QueryListSource	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=pagedResponse{content=[]models.SourceResponse}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/sources [get]
func (h *Handler) LoadSourcesPaged(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var q *models.QueryListSource

	data := q.Transform(r)
	sources, paginationData, err := postgres.NewSourceRepo(h.A.DB, h.A.Cache).LoadSourcesPaged(r.Context(), project.UID, data.SourceFilter, data.Pageable)
	if err != nil {
		log.WithError(err).Error("an error occurred while fetching sources")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching sources", http.StatusBadRequest))
		return
	}

	baseUrl, err := h.retrieveHost()
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var org *datastore.Organisation
	orgRepo := postgres.NewOrgRepo(h.A.DB, h.A.Cache)
	org, err = orgRepo.FetchOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	var customDomain string
	if org == nil {
		customDomain = ""
	} else {
		customDomain = org.CustomDomain.ValueOrZero()
	}

	for i := range sources {
		fillSourceURL(&sources[i], baseUrl, customDomain)
	}

	resp := models.NewListResponse(sources, func(source datastore.Source) models.SourceResponse {
		return models.SourceResponse{Source: &source}
	})
	_ = render.Render(w, r, util.NewServerResponse("Sources fetched successfully", pagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))
}

func fillSourceURL(s *datastore.Source, baseUrl string, customDomain string) {
	url := baseUrl
	if len(customDomain) > 0 {
		url = customDomain
	}

	s.URL = fmt.Sprintf("%s/ingest/%s", url, s.MaskID)
}
