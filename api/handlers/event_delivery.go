package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// GetEventDelivery
//
//	@Id				GetEventDelivery
//	@Summary		Retrieve an event delivery
//	@Description	This endpoint fetches an event delivery.
//	@Tags			Event Deliveries
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			eventDeliveryID	path		string	true	"event delivery id"
//	@Success		200				{object}	util.ServerResponse{data=models.EventDeliveryResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID} [get]
func (h *Handler) GetEventDelivery(w http.ResponseWriter, r *http.Request) {
	eventDelivery, err := h.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	resp := &models.EventDeliveryResponse{EventDelivery: eventDelivery}
	_ = render.Render(w, r, util.NewServerResponse("Event Delivery fetched successfully",
		resp, http.StatusOK))
}

// ResendEventDelivery
//
//	@Id				ResendEventDelivery
//	@Summary		Retry event delivery
//	@Description	This endpoint retries an event delivery.
//	@Tags			Event Deliveries
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			eventDeliveryID	path		string	true	"event delivery id"
//	@Success		200				{object}	util.ServerResponse{data=models.EventDeliveryResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/resend [put]
func (h *Handler) ResendEventDelivery(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	eventDelivery, err := h.retrieveEventDelivery(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fr := services.RetryEventDeliveryService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(h.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(h.A.DB),
		Queue:             h.A.Queue,
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
//
//	@Summary		Batch retry event delivery
//	@Description	This endpoint batch retries multiple event deliveries at once.
//	@Tags			Event Deliveries
//	@Id				BatchRetryEventDelivery
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string							true	"Project ID"
//	@Param			request		query		models.QueryListEventDelivery	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/eventdeliveries/batchretry [post]
func (h *Handler) BatchRetryEventDelivery(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEventDelivery

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
			_ = render.Render(w, r, util.NewServerResponse("the portal link doesn't contain any endpoints", nil, http.StatusOK))
			return
		}

		data.Filter.EndpointIDs = endpointIDs
	}

	data.Filter.Project = project
	ep := datastore.Pageable{}
	if data.Filter.Pageable == ep {
		data.Filter.Pageable.PerPage = 2000000000
	}

	br := services.BatchRetryEventDeliveryService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(h.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(h.A.DB),
		Queue:             h.A.Queue,
		EventRepo:         postgres.NewEventRepo(h.A.DB),
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
//
//	@Summary		Force retry event delivery
//	@Description	This endpoint enables you retry a previously successful event delivery
//	@Id				ForceResendEventDeliveries
//	@Tags			Event Deliveries
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string		true	"Project ID"
//	@Param			deliveryIds	body		models.IDs	true	"event delivery ids"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/eventdeliveries/forceresend [post]
func (h *Handler) ForceResendEventDeliveries(w http.ResponseWriter, r *http.Request) {
	eventDeliveryIDs := models.IDs{}

	err := json.NewDecoder(r.Body).Decode(&eventDeliveryIDs)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	fr := services.ForceResendEventDeliveriesService{
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(h.A.DB),
		EndpointRepo:      postgres.NewEndpointRepo(h.A.DB),
		Queue:             h.A.Queue,
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

// GetEventDeliveriesPaged
//
//	@Summary		List all event deliveries
//	@Description	This endpoint retrieves all event deliveries paginated.
//	@Tags			Event Deliveries
//	@Accept			json
//	@Id				GetEventDeliveriesPaged
//	@Produce		json
//	@Param			projectID	path		string							true	"Project ID"
//	@Param			request		query		models.QueryListEventDelivery	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=models.PagedResponse{content=[]models.EventDeliveryResponse}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/eventdeliveries [get]
func (h *Handler) GetEventDeliveriesPaged(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEventDelivery

	project, err := h.retrieveProject(r)
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
		event, err := postgres.NewEventRepo(h.A.DB).FindFirstEventWithIdempotencyKey(r.Context(), project.UID, data.IdempotencyKey)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
		data.EventID = event.UID
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

	f := data.Filter

	ed, paginationData, err := postgres.NewEventDeliveryRepo(h.A.DB).LoadEventDeliveriesPaged(r.Context(), project.UID, f.EndpointIDs, f.EventID, f.SubscriptionID, f.Status, f.SearchParams, f.Pageable, f.IdempotencyKey, f.EventType)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch event deliveries")
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
		return
	}

	resp := models.NewListResponse(ed, func(ed datastore.EventDelivery) models.EventDeliveryResponse {
		return models.EventDeliveryResponse{EventDelivery: &ed}
	})

	_ = render.Render(w, r, util.NewServerResponse("Event deliveries fetched successfully",
		models.PagedResponse{Content: resp, Pagination: &paginationData}, http.StatusOK))
}

func (h *Handler) CountAffectedEventDeliveries(w http.ResponseWriter, r *http.Request) {
	var q *models.QueryListEventDelivery

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
			_ = render.Render(w, r, util.NewServerResponse("event deliveries count successful", map[string]interface{}{"num": 0}, http.StatusOK))
			return
		}

		data.Filter.EndpointIDs = endpointIDs
	}

	f := data.Filter
	count, err := postgres.NewEventDeliveryRepo(h.A.DB).CountEventDeliveries(r.Context(), project.UID, f.EndpointIDs, f.EventID, f.Status, f.SearchParams)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("an error occurred while fetching event deliveries")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("event deliveries count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

func (h *Handler) retrieveEventDelivery(r *http.Request) (*datastore.EventDelivery, error) {
	project, err := h.retrieveProject(r)
	if err != nil {
		return &datastore.EventDelivery{}, err
	}

	eventDeliveryID := chi.URLParam(r, "eventDeliveryID")
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(h.A.DB)
	return eventDeliveryRepo.FindEventDeliveryByID(r.Context(), project.UID, eventDeliveryID)
}
