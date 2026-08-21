package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	batch_retries "github.com/frain-dev/convoy/internal/batch_retries"
	"github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
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

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, eventDelivery.EndpointID) {
		return
	}

	resp := models.NewEventDeliveryResponse(eventDelivery, h.canViewRawHeaders(authUser))
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

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, eventDelivery.EndpointID) {
		return
	}

	fr := services.RetryEventDeliveryService{
		EventDeliveryRepo: event_deliveries.New(h.A.Logger, h.A.DB),
		EndpointRepo:      endpoints.New(h.A.Logger, h.A.DB),
		Queue:             h.A.Queue,
		EventDelivery:     eventDelivery,
		Project:           project,
		Logger:            h.A.Logger,
	}

	err = fr.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := models.NewEventDeliveryResponse(eventDelivery, h.canViewRawHeaders(authUser))
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

		endpointIDs, err := h.portalScopedEndpointIDs(r, portalLink, data.Filter.EndpointIDs)
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

	data.Filter.ProjectID = project.UID
	pp := &datastore.Pageable{
		PerPage:   10000,
		Sort:      "DESC",
		Direction: datastore.Next,
	}
	pp.SetCursors()

	data.Filter.Pageable = *pp

	br := services.BatchRetryEventDeliveryService{
		BatchRetryRepo:    batch_retries.New(nil, h.A.DB),
		EventDeliveryRepo: event_deliveries.New(h.A.Logger, h.A.DB),
		Queue:             h.A.Queue,
		Filter:            data.Filter,
		ProjectID:         project.UID,
		Logger:            h.A.Logger,
	}

	err = br.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Batch retry processing", nil, http.StatusOK))
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

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		// Resolve the target deliveries up front so we can prove every one of them is
		// owned by the portal link before queueing any re-delivery. Fail closed: if a
		// requested id is foreign or unknown the whole batch is rejected.
		deliveries, innerErr := event_deliveries.New(h.A.Logger, h.A.DB).FindEventDeliveriesByIDs(r.Context(), project.UID, eventDeliveryIDs.IDs)
		if innerErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
			return
		}

		// FindEventDeliveriesByIDs dedupes rows, so compare against the set of resolved
		// ids rather than the raw count; a repeated (but owned) id must not fail closed.
		foundIDs := make(map[string]struct{}, len(deliveries))
		for i := range deliveries {
			foundIDs[deliveries[i].UID] = struct{}{}
		}
		for _, id := range eventDeliveryIDs.IDs {
			if _, ok := foundIDs[id]; !ok {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}
		}

		endpointIDs := make([]string, 0, len(deliveries))
		for i := range deliveries {
			endpointIDs = append(endpointIDs, deliveries[i].EndpointID)
		}

		if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, endpointIDs...) {
			return
		}
	}

	fr := services.ForceResendEventDeliveriesService{
		EventDeliveryRepo: event_deliveries.New(h.A.Logger, h.A.DB),
		EndpointRepo:      endpoints.New(h.A.Logger, h.A.DB),
		Queue:             h.A.Queue,
		IDs:               eventDeliveryIDs.IDs,
		Project:           project,
		Logger:            h.A.Logger,
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
//	@Param			projectID		path		string							true	"Project ID"
//	@Param			request			query		models.QueryListEventDelivery	false	"Query Params"
//	@Success		200				{object}	util.ServerResponse{data=models.PagedResponse{content=[]models.EventDeliveryResponse}}
//	@Failure		400,401,404,504	{object}	util.ServerResponse{data=Stub}
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
		event, err := events.New(h.A.Logger, h.A.DB).FindFirstEventWithIdempotencyKey(r.Context(), project.UID, data.IdempotencyKey)
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

		endpointIDs, err := h.portalScopedEndpointIDs(r, portalLink, data.Filter.EndpointIDs)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}

		if len(endpointIDs) == 0 {
			_ = render.Render(w, r, util.NewServerResponse("Event deliveries fetched successfully",
				models.PagedResponse{Content: []models.EventDeliveryResponse{}, Pagination: &datastore.PaginationData{PerPage: int64(data.Filter.Pageable.PerPage)}}, http.StatusOK))
			return
		}

		data.Filter.EndpointIDs = endpointIDs
	}

	f := data.Filter

	ctx, cancel := context.WithTimeout(r.Context(), events.SearchTimeout)
	defer cancel()

	ed, paginationData, err := event_deliveries.New(h.A.Logger, h.A.DB).LoadEventDeliveriesPaged(ctx, project.UID, f.EndpointIDs, f.EventID, f.SubscriptionID, f.Status, f.SearchParams, f.Pageable, f.IdempotencyKey, f.EventType, f.BrokerMessageId)
	if err != nil {
		if renderEventDeliveriesTimeout(w, r, err) {
			return
		}
		h.A.Logger.ErrorContext(r.Context(), "failed to fetch event deliveries", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
		return
	}

	showRawHeaders := h.canViewRawHeaders(authUser)
	resp := models.NewListResponse(ed, func(ed datastore.EventDelivery) models.EventDeliveryResponse {
		return models.NewEventDeliveryResponse(&ed, showRawHeaders)
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

		endpointIDs, err := h.portalScopedEndpointIDs(r, portalLink, data.Filter.EndpointIDs)
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

	ctx, cancel := context.WithTimeout(r.Context(), events.SearchTimeout)
	defer cancel()

	count, err := event_deliveries.New(h.A.Logger, h.A.DB).CountEventDeliveries(ctx, project.UID, f.EndpointIDs, f.EventID, f.Status, f.SearchParams)
	if err != nil {
		if renderEventDeliveriesTimeout(w, r, err) {
			return
		}
		h.A.Logger.ErrorContext(r.Context(), "an error occurred while fetching event deliveries", "error", err)
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("event deliveries count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

// EventDeliveryStatusTotals serves the dashboard's per-status delivery totals
// from the daily rollup, falling back to one grouped live scan until the
// backfill completes.
//
// This is deliberately not CountAffectedEventDeliveries: that endpoint answers
// "how many deliveries would this batch retry touch", which must stay an exact
// live count, while these totals are display figures at UTC day grain.
func (h *Handler) EventDeliveryStatusTotals(w http.ResponseWriter, r *http.Request) {
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

	endpointIDs := data.Filter.EndpointIDs
	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		portalLink, innerErr := h.retrievePortalLinkFromToken(r)
		if innerErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
			return
		}

		endpointIDs, innerErr = h.portalScopedEndpointIDs(r, portalLink, endpointIDs)
		if innerErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
			return
		}

		// A portal link that resolves to no endpoint has an empty scope, which
		// is not the same as "every endpoint in the project".
		if len(endpointIDs) == 0 {
			_ = render.Render(w, r, util.NewServerResponse("event delivery status totals fetched successfully",
				models.DeliveryStatusTotalsResponse{Totals: map[string]int64{}, Source: string(event_deliveries.StatusTotalsFromLive)}, http.StatusOK))
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), events.SearchTimeout)
	defer cancel()

	totals, source, err := event_deliveries.New(h.A.Logger, h.A.DB).
		StatusTotals(ctx, project.UID, data.Filter.SearchParams, endpointIDs)
	if err != nil {
		if renderEventDeliveriesTimeout(w, r, err) {
			return
		}
		h.A.Logger.ErrorContext(r.Context(), "an error occurred while fetching event delivery status totals", "error", err)
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	out := make(map[string]int64, len(totals))
	for status, count := range totals {
		out[string(status)] = count
	}

	_ = render.Render(w, r, util.NewServerResponse("event delivery status totals fetched successfully",
		models.DeliveryStatusTotalsResponse{Totals: out, Source: string(source)}, http.StatusOK))
}

const eventDeliveriesTimeoutMsg = "Event deliveries took too long. Narrow the date range."

// renderEventDeliveriesTimeout maps a query deadline to 504.
// Failure policy: fail closed. DeadlineExceeded and Postgres 57014 are
// timeouts; other errors are left to the caller. The timeout is
// events.SearchTimeout so a wide date range cannot sit until WriteTimeout
// and return a 500 from a canceled request context.
func renderEventDeliveriesTimeout(w http.ResponseWriter, r *http.Request, err error) bool {
	if !events.IsSearchTimeout(err) {
		return false
	}
	_ = render.Render(w, r, util.NewErrorResponse(eventDeliveriesTimeoutMsg, http.StatusGatewayTimeout))
	return true
}

func (h *Handler) retrieveEventDelivery(r *http.Request) (*datastore.EventDelivery, error) {
	project, err := h.retrieveProject(r)
	if err != nil {
		return &datastore.EventDelivery{}, err
	}

	eventDeliveryID := chi.URLParam(r, "eventDeliveryID")
	eventDeliveryRepo := event_deliveries.New(h.A.Logger, h.A.DB)
	return eventDeliveryRepo.FindEventDeliveryByID(r.Context(), project.UID, eventDeliveryID)
}
