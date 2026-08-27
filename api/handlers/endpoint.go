package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	endpointsvc "github.com/frain-dev/convoy/internal/endpoints"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/pkg/cbenablement"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	convoynet "github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/cachedrepo"
	"github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/constants"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

// CreateEndpoint
//
//	@Summary		Create an endpoint
//	@Description	This endpoint creates an endpoint
//	@Tags			Endpoints
//	@Id				CreateEndpoint
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			endpoint	body		models.CreateEndpoint	true	"Endpoint Details"
//	@Success		201			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints [post]
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
	authUser := middleware.GetAuthUserFromContext(r.Context())

	migrator, err := h.Versioning.For(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to create migrator: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid API version", http.StatusBadRequest))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.A.Logger.Errorf("Failed to read request body: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request", http.StatusBadRequest))
		return
	}

	var e models.CreateEndpoint
	err = migrator.Unmarshal(body, &e)
	if err != nil {
		h.A.Logger.Errorf("Failed to parse endpoint creation request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	// Set default content type if not provided
	if e.ContentType == "" {
		e.ContentType = constants.ContentTypeJSON
	}

	err = e.Validate()
	if err != nil {
		h.A.Logger.Errorf("Endpoint creation validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to retrieve project: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusBadRequest))
		return
	}
	if !h.requireJWTProjectManage(w, r, project) {
		return
	}

	if h.IsReqWithPortalLinkToken(authUser) {
		pLink, innerErr := h.retrievePortalLinkFromToken(r)
		if innerErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
			return
		}

		e.OwnerID = pLink.OwnerID
	}

	ce := services.NewCreateEndpointService(
		h.endpointWriteRepo(),
		h.projectRepo(),
		h.A.Licenser,
		h.A.FFlag,
		h.A.FeatureFlagFetcher,
		h.A.EarlyAdopterFeatureFetcher,
		h.A.DB,
		h.A.Logger,
		e,
		project.UID,
	)

	endpoint, err := ce.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}

	resBytes, err := migrator.Marshal(resp)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	serverResponse := util.ServerResponse{
		Status:  true,
		Message: "Endpoint created successfully",
		Data:    resBytes,
	}

	finalBytes, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, finalBytes, http.StatusCreated)
}

// GetEndpoint
//
//	@Summary		Retrieve endpoint
//	@Description	This endpoint fetches an endpoint
//	@Id				GetEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [get]
func (h *Handler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
	migrator, err := h.Versioning.For(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to create migrator: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid API version", http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to retrieve project: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusBadRequest))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, endpointID) {
		return
	}

	endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
	if err != nil {
		h.A.Logger.Errorf("Failed to retrieve endpoint: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Resource not found", http.StatusNotFound))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}

	resBytes, err := migrator.Marshal(resp)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	serverResponse := util.ServerResponse{
		Status:  true,
		Message: "Endpoint fetched successfully",
		Data:    resBytes,
	}

	finalBytes, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, finalBytes, http.StatusOK)
}

// GetEndpoints
//
//	@Summary		List all endpoints
//	@Description	This endpoint fetches an endpoints
//	@Tags			Endpoints
//	@Id				GetEndpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			request		query		models.QueryListEndpoint	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=models.PagedResponse{content=[]models.EndpointResponse}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints [get]
func (h *Handler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
	migrator, err := h.Versioning.For(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to create migrator: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid API version", http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to retrieve project: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusBadRequest))
		return
	}

	var q *models.QueryListEndpoint
	data := q.Transform(r)

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if h.IsReqWithPortalLinkToken(authUser) {
		portalLink, innerErr := h.retrievePortalLinkFromToken(r)
		if innerErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
			return
		}

		endpointIDs, innerErr := h.getEndpoints(r, portalLink)
		if innerErr != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
			return
		}

		if len(endpointIDs) == 0 {
			_ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
				models.PagedResponse{Content: endpointIDs, Pagination: &datastore.PaginationData{PerPage: int64(data.Filter.Pageable.PerPage)}}, http.StatusOK))
			return
		}

		data.Filter.EndpointIDs = endpointIDs
	}

	endpoints, paginationData, err := endpointsvc.New(h.A.Logger, h.A.DB).LoadEndpointsPaged(r.Context(), project.UID, data.Filter, data.Pageable)
	if err != nil {
		h.A.Logger.Error("failed to load endpoints", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("Failed to load endpoints", http.StatusBadRequest))
		return
	}

	circuitBreakerEnabled := cbenablement.EnabledForOrg(
		r.Context(), h.A.FFlag, h.A.FeatureFlagFetcher, h.A.AdminManaged, project.OrganisationID)
	if circuitBreakerEnabled && h.A.Licenser.CircuitBreaking() && len(endpoints) > 0 && h.A.CircuitBreakerStore != nil {
		keys := make([]string, len(endpoints))
		for i := 0; i < len(endpoints); i++ {
			keys[i] = fmt.Sprintf("breaker:%s", endpoints[i].UID)
		}

		cbs, err := h.A.CircuitBreakerStore.GetMany(r.Context(), keys...)
		if err != nil {
			_ = render.Render(w, r, util.NewServiceErrResponse(err))
			return
		}

		for i := 0; i < len(cbs); i++ {
			if cbs[i] != nil {
				str, ok := cbs[i].(string)
				if ok {
					var c circuit_breaker.CircuitBreaker
					asBytes := []byte(str)
					innerErr := msgpack.DecodeMsgPack(asBytes, &c)
					if innerErr != nil {
						continue
					}
					rate := c.FailureRate
					endpoints[i].FailureRate = &rate
					state := c.State.String()
					endpoints[i].CBState = &state
				}
			}
		}
	}

	resp := models.NewListResponse(endpoints, func(endpoint datastore.Endpoint) models.EndpointResponse {
		return models.EndpointResponse{Endpoint: &endpoint}
	})

	pagedResp := models.PagedResponse{Content: &resp, Pagination: &paginationData}

	resBytes, err := migrator.Marshal(&pagedResp)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	serverResponse := util.ServerResponse{
		Status:  true,
		Message: "Endpoints fetched successfully",
		Data:    resBytes,
	}

	finalBytes, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, finalBytes, http.StatusOK)
}

// Clients that walk every page (portal status filter) must chunk to this cap.
const maxPeriodFailureRateIDs = 100

// GetEndpointPeriodFailureRates
//
//	@Summary		Endpoint period failure rates
//	@Description	Display-only delivery rates for the given endpoint ids over a date range (default last 7 days). Independent of the list so a slow COUNT cannot delay the table.
//	@Id				GetEndpointPeriodFailureRates
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string		true	"Project ID"
//	@Param			endpointId	query		[]string	false	"Endpoint IDs"
//	@Param			startDate	query		string		false	"Start date"
//	@Param			endDate		query		string		false	"End date"
//	@Success		200			{object}	util.ServerResponse{data=[]models.EndpointPeriodFailureRate}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/period-failure-rates [get]
func (h *Handler) GetEndpointPeriodFailureRates(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	ids := parseEndpointIDs(r.URL.Query()["endpointId"], maxPeriodFailureRateIDs)
	authUser := middleware.GetAuthUserFromContext(r.Context())
	ownedIDs, isPortal, ok := h.portalLinkOwnedEndpointIDs(w, r, authUser)
	if !ok {
		return
	}
	if isPortal {
		ids = intersectIDs(ids, ownedIDs)
	}

	if len(ids) == 0 {
		_ = render.Render(w, r, util.NewServerResponse("Endpoint period failure rates fetched successfully",
			[]models.EndpointPeriodFailureRate{}, http.StatusOK))
		return
	}

	searchParams, perr := models.GetSearchParams(r)
	if perr != nil {
		_ = render.Render(w, r, util.NewErrorResponse(perr.Error(), http.StatusBadRequest))
		return
	}

	endpoints := make([]datastore.Endpoint, len(ids))
	for i, id := range ids {
		endpoints[i].UID = id
	}
	h.enrichEndpointsWithPeriodFailureRate(r.Context(), project.UID, endpoints, searchParams)

	out := make([]models.EndpointPeriodFailureRate, len(endpoints))
	for i := range endpoints {
		out[i] = models.EndpointPeriodFailureRate{
			UID:               endpoints[i].UID,
			PeriodFailureRate: endpoints[i].PeriodFailureRate,
			SuccessCount:      endpoints[i].SuccessCount,
			FailureCount:      endpoints[i].FailureCount,
			RetryCount:        endpoints[i].RetryCount,
		}
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint period failure rates fetched successfully", out, http.StatusOK))
}

func parseEndpointIDs(values []string, limit int) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if len(out) == limit {
			break
		}
	}
	return out
}

func intersectIDs(requested, allowed []string) []string {
	allow := make(map[string]struct{}, len(allowed))
	for _, id := range allowed {
		allow[id] = struct{}{}
	}
	out := make([]string, 0, len(requested))
	for _, id := range requested {
		if _, ok := allow[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

// periodFailureRateTimeout bounds the display-only COUNT on event_deliveries so a
// slow query cannot sit until WriteTimeout or the ingress idle timeout and 504
// the rates request. The endpoint list no longer waits on this COUNT.
const periodFailureRateTimeout = 2 * time.Second

// enrichEndpointsWithPeriodFailureRate attaches the history failure rate
// ((Failure+Retry)/(Success+Failure+Retry)) and the underlying counts to each endpoint
// over the given range. Retry deliveries are in-flight but have failed at least once,
// so they count toward the rate immediately instead of hiding an ongoing outage behind
// a dash until retries exhaust. Scheduled/Processing (not yet failed) and Discarded
// deliveries stay excluded. It mutates the slice in place, mirroring the circuit
// breaker enrichment.
//
// Failure policy: this is a display-only enrichment, so it fails open. A query error
// or a count that exceeds periodFailureRateTimeout is logged and the endpoints keep
// nil rate/counts (rendered as an em dash), rather than failing the rates response.
// The timeout must stay well under WriteTimeout and typical ingress idle timeouts
// so a slow COUNT cannot 504 the request.
func (h *Handler) enrichEndpointsWithPeriodFailureRate(ctx context.Context, projectID string,
	endpoints []datastore.Endpoint, params datastore.SearchParams) {
	endpointIDs := make([]string, len(endpoints))
	for i := range endpoints {
		endpointIDs[i] = endpoints[i].UID
	}

	statuses := []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.FailureEventStatus, datastore.RetryEventStatus}
	err := loadPeriodFailureRates(ctx, periodFailureRateTimeout, endpoints, func(ctx context.Context) ([]datastore.EndpointStatusDeliveryCount, error) {
		return event_deliveries.New(h.A.Logger, h.A.DB).
			CountDeliveriesByEndpointAndStatus(ctx, projectID, endpointIDs, statuses, params)
	})
	if err != nil {
		h.A.Logger.Error("failed to load period failure rate for endpoints", "error", err)
	}
}

func loadPeriodFailureRates(ctx context.Context, timeout time.Duration, endpoints []datastore.Endpoint,
	countFn func(context.Context) ([]datastore.EndpointStatusDeliveryCount, error)) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	counts, err := countFn(ctx)
	if err != nil {
		return err
	}

	applyPeriodFailureRates(endpoints, counts)
	return nil
}

// applyPeriodFailureRates folds per-status delivery counts into the endpoints'
// transient failure-rate fields. Rate is (Failure+Retry)/(Success+Failure+Retry);
// endpoints with no counted deliveries keep nil rate/counts (rendered as a dash,
// distinct from a genuine 0%).
func applyPeriodFailureRates(endpoints []datastore.Endpoint, counts []datastore.EndpointStatusDeliveryCount) {
	type tally struct{ success, failure, retry int64 }
	byEndpoint := make(map[string]*tally, len(endpoints))
	for _, c := range counts {
		t := byEndpoint[c.EndpointID]
		if t == nil {
			t = &tally{}
			byEndpoint[c.EndpointID] = t
		}
		switch c.Status {
		case datastore.SuccessEventStatus:
			t.success = c.Count
		case datastore.FailureEventStatus:
			t.failure = c.Count
		case datastore.RetryEventStatus:
			t.retry = c.Count
		}
	}

	for i := range endpoints {
		t := byEndpoint[endpoints[i].UID]
		if t == nil {
			continue
		}
		success, failure, retry := t.success, t.failure, t.retry
		endpoints[i].SuccessCount = &success
		endpoints[i].FailureCount = &failure
		endpoints[i].RetryCount = &retry
		if total := success + failure + retry; total > 0 {
			rate := float64(failure+retry) / float64(total)
			endpoints[i].PeriodFailureRate = &rate
		}
	}
}

// UpdateEndpoint
//
//	@Summary		Update an endpoint
//	@Description	This endpoint updates an endpoint
//	@Id				UpdateEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			endpointID	path		string					true	"Endpoint ID"
//	@Param			endpoint	body		models.UpdateEndpoint	true	"Endpoint Details"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [put]
func (h *Handler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
	migrator, err := h.Versioning.For(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to create migrator: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid API version", http.StatusBadRequest))
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
	if !h.requireJWTProjectManage(w, r, project) {
		return
	}

	var e models.UpdateEndpoint
	err = migrator.Unmarshal(body, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, endpointID) {
		return
	}

	endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	// Set default content type if not provided
	if e.ContentType == nil || *e.ContentType == "" {
		defaultContentType := constants.ContentTypeJSON
		e.ContentType = &defaultContentType
	}

	err = e.Validate()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	ce := services.NewUpdateEndpointService(
		h.A.Cache,
		h.endpointWriteRepo(),
		h.projectRepo(),
		h.A.Licenser,
		h.A.FFlag,
		h.A.FeatureFlagFetcher,
		h.A.EarlyAdopterFeatureFetcher,
		h.A.DB,
		h.A.Logger,
		e,
		endpoint,
		project,
	)

	endpoint, err = ce.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}

	resBytes, err := migrator.Marshal(resp)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	serverResponse := util.ServerResponse{
		Status:  true,
		Message: "Endpoint updated successfully",
		Data:    resBytes,
	}

	finalBytes, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, finalBytes, http.StatusAccepted)
}

// DeleteEndpoint
//
//	@Summary		Delete endpoint
//	@Description	This endpoint deletes an endpoint
//	@Tags			Endpoints
//	@Id				DeleteEndpoint
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [delete]
func (h *Handler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
	if !h.requireJWTProjectManage(w, r, project) {
		return
	}

	endpointID := chi.URLParam(r, "endpointID")

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, endpointID) {
		return
	}

	endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	// Collected before the delete: the cascade removes the rows these ids live on.
	sourceKeys := h.subscriptionSourceKeys(r.Context(), project.UID, endpointID)

	err = h.endpointWriteRepo().DeleteEndpoint(r.Context(), endpoint, project.UID)
	if err != nil {
		h.A.Logger.ErrorContext(r.Context(), "failed to delete endpoint", "error", err)
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete endpoint", http.StatusBadRequest))
		return
	}

	// The repository evicts the endpoint-keyed list; these are the source-keyed
	// lists the same cascade invalidated.
	if len(sourceKeys) > 0 {
		cachedrepo.Invalidate(r.Context(), h.A.Cache, h.A.Logger, sourceKeys...)
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint deleted successfully", nil, http.StatusOK))
}

// ExpireSecret
//
//	@Summary		Roll endpoint secret
//	@Description	This endpoint expires and re-generates the endpoint secret.
//	@Id				ExpireSecret
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			endpointID	path		string				true	"Endpoint ID"
//	@Param			endpoint	body		models.ExpireSecret	true	"Expire Secret Body Parameters"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/expire_secret [put]
func (h *Handler) ExpireSecret(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	var e *models.ExpireSecret
	err = util.ReadJSON(r, &e)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := chi.URLParam(r, "endpointID")

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, endpointID) {
		return
	}

	endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	xs := services.ExpireSecretService{
		Queuer:       h.A.Queue,
		Cache:        h.A.Cache,
		EndpointRepo: h.endpointWriteRepo(),
		ProjectRepo:  h.projectRepo(),
		S:            e,
		Endpoint:     endpoint,
		Project:      project,
		Logger:       h.A.Logger,
	}

	endpoint, err = xs.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}
	_ = render.Render(w, r, util.NewServerResponse("endpoint secret expired successfully",
		resp, http.StatusOK))
}

// PauseEndpoint
//
//	@Summary		Pause endpoint
//	@Description	Toggles an endpoint's status between active and paused states
//	@Id				PauseEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/pause [put]
func (h *Handler) PauseEndpoint(w http.ResponseWriter, r *http.Request) {
	migrator, err := h.Versioning.For(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to create migrator: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid API version", http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}
	if !h.requireJWTProjectManage(w, r, project) {
		return
	}

	endpointID := chi.URLParam(r, "endpointID")

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, endpointID) {
		return
	}

	ps := services.PauseEndpointService{
		EndpointRepo: h.endpointWriteRepo(),
		ProjectID:    project.UID,
		EndpointId:   endpointID,
		Logger:       h.A.Logger,
	}

	endpoint, err := ps.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}

	resBytes, err := migrator.Marshal(resp)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	serverResponse := util.ServerResponse{
		Status:  true,
		Message: "endpoint status updated successfully",
		Data:    resBytes,
	}

	finalBytes, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, finalBytes, http.StatusAccepted)
}

// ActivateEndpoint
//
//	@Summary		Activate endpoint
//	@Description	Activated an inactive endpoint
//	@Id				ActivateEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/activate [post]
func (h *Handler) ActivateEndpoint(w http.ResponseWriter, r *http.Request) {
	migrator, err := h.Versioning.For(r)
	if err != nil {
		h.A.Logger.Errorf("Failed to create migrator: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid API version", http.StatusBadRequest))
		return
	}

	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}
	if !h.requireJWTProjectManage(w, r, project) {
		return
	}

	endpointID := chi.URLParam(r, "endpointID")

	authUser := middleware.GetAuthUserFromContext(r.Context())
	if !h.ensurePortalLinkOwnsEndpoints(w, r, authUser, endpointID) {
		return
	}

	aes := services.ActivateEndpointService{
		EndpointRepo: h.endpointWriteRepo(),
		Queue:        h.A.Queue,
		Licenser:     h.A.Licenser,
		Project:      project,
		ProjectID:    project.UID,
		EndpointId:   endpointID,
		Logger:       h.A.Logger,
	}

	endpoint, err := aes.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	if h.A.CircuitBreakerStore != nil {
		key := fmt.Sprintf("breaker:%s", endpoint.UID)
		cbs, cbErr := h.A.CircuitBreakerStore.GetOne(r.Context(), key)
		if cbErr != nil && !errors.Is(cbErr, circuit_breaker.ErrCircuitBreakerNotFound) {
			h.A.Logger.Error("failed to find circuit breaker", "error", cbErr)
		}

		if len(cbs) > 0 {
			c, innerErr := circuit_breaker.NewCircuitBreakerFromStore([]byte(cbs), h.A.Logger)
			if innerErr != nil {
				h.A.Logger.Error("failed to decode circuit breaker", "error", innerErr)
			} else {
				c.Reset(time.Now())
				b, msgPackErr := msgpack.EncodeMsgPack(c)
				if msgPackErr != nil {
					h.A.Logger.Error("failed to encode circuit breaker", "error", msgPackErr)
				} else if setErr := h.A.CircuitBreakerStore.SetOne(r.Context(), key, b, time.Minute*5); setErr != nil {
					h.A.Logger.Error("failed to persist circuit breaker", "error", setErr)
				}
			}
		}
	}

	resp := &models.EndpointResponse{Endpoint: endpoint}

	resBytes, err := migrator.Marshal(resp)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	serverResponse := util.ServerResponse{
		Status:  true,
		Message: "endpoint status successfully activated",
		Data:    resBytes,
	}

	finalBytes, err := json.Marshal(serverResponse)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	util.WriteResponse(w, r, finalBytes, http.StatusAccepted)
}

// TestOAuth2Connection
//
//	@Summary		Test OAuth2 connection
//	@Description	This endpoint tests the OAuth2 connection by attempting to exchange a token
//	@Tags			Endpoints
//	@Id				TestOAuth2Connection
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			oauth2		body		models.TestOAuth2Request	true	"OAuth2 Configuration"
//	@Success		200			{object}	util.ServerResponse{data=models.TestOAuth2Response}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/oauth2/test [post]
func (h *Handler) TestOAuth2Connection(w http.ResponseWriter, r *http.Request) {
	var testReq models.TestOAuth2Request
	err := util.ReadJSON(r, &testReq)
	if err != nil {
		h.A.Logger.Errorf("Failed to parse OAuth2 test request: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
		return
	}

	err = testReq.Validate()
	if err != nil {
		h.A.Logger.Errorf("OAuth2 test request validation failed: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
		return
	}

	// Transform API model to datastore model
	if testReq.OAuth2 == nil {
		_ = render.Render(w, r, util.NewErrorResponse("OAuth2 configuration is required", http.StatusBadRequest))
		return
	}
	oauth2Config := testReq.OAuth2.Transform()
	if oauth2Config == nil {
		_ = render.Render(w, r, util.NewErrorResponse("OAuth2 configuration is required", http.StatusBadRequest))
		return
	}

	// Create a temporary endpoint for testing
	testEndpoint := &datastore.Endpoint{
		UID: "test",
		Authentication: &datastore.EndpointAuthentication{
			Type:   datastore.OAuth2Authentication,
			OAuth2: oauth2Config,
		},
	}

	oauth2Service, err := h.newOAuth2TokenTestService()
	if err != nil {
		h.A.Logger.Errorf("Failed to configure OAuth2 test client: %v", err)
		_ = render.Render(w, r, util.NewErrorResponse("Failed to configure OAuth2 test client", http.StatusInternalServerError))
		return
	}

	// Get authorization header (includes token type)
	authHeader, err := oauth2Service.GetAuthorizationHeader(r.Context(), testEndpoint)
	if err != nil {
		h.A.Logger.Errorf("OAuth2 token exchange failed: %v", err)
		_ = render.Render(w, r, util.NewServerResponse(
			"OAuth2 connection test failed",
			models.TestOAuth2Response{
				Success: false,
				Error:   err.Error(),
			},
			http.StatusOK,
		))
		return
	}

	// Parse token type and access token from authorization header
	// Format: "TokenType access_token" (e.g., "Bearer token123" or "CustomType token123")
	parts := strings.SplitN(authHeader, " ", 2)
	tokenType := "Bearer" // Default
	var accessToken string
	if len(parts) == 2 {
		tokenType = parts[0]
		accessToken = parts[1]
	} else {
		// Fallback if format is unexpected
		accessToken = authHeader
	}

	// Get the cached token to return full response details (including expires_at)
	cacheKey := "oauth2_token:test"
	var cachedToken services.CachedToken
	err = h.A.Cache.Get(r.Context(), cacheKey, &cachedToken)

	var expiresAt time.Time
	if err == nil {
		// Use token type from cache if available (more accurate)
		if cachedToken.TokenType != "" {
			tokenType = cachedToken.TokenType
		}
		if cachedToken.AccessToken != "" {
			accessToken = cachedToken.AccessToken
		}
		expiresAt = cachedToken.ExpiresAt
	}

	// Return full response with token details
	resp := models.TestOAuth2Response{
		Success:     true,
		AccessToken: accessToken,
		TokenType:   tokenType,
		ExpiresAt:   expiresAt,
		Message:     "OAuth2 connection successful",
	}

	// Clean up test cache entry
	_ = h.A.Cache.Delete(r.Context(), cacheKey)

	_ = render.Render(w, r, util.NewServerResponse("OAuth2 connection test successful", resp, http.StatusOK))
}

func (h *Handler) newOAuth2TokenTestService() (*services.OAuth2TokenService, error) {
	dispatcher, err := convoynet.NewDispatcher(
		h.A.Licenser,
		h.A.FFlag,
		convoynet.LoggerOption(h.A.Logger),
		convoynet.ProxyOption(h.A.Cfg.Server.HTTP.HttpProxy, h.A.Cfg.Server.HTTP.NoProxy),
		convoynet.AllowListOption(h.A.Cfg.Dispatcher.AllowList),
		convoynet.BlockListOption(h.A.Cfg.Dispatcher.BlockList),
	)
	if err != nil {
		return nil, err
	}

	return services.NewOAuth2TokenService(
		h.A.Cache,
		h.A.Logger,
		services.WithOAuth2HTTPClient(dispatcher.HTTPClient()),
		services.WithOAuth2Context(dispatcher.ContextWithRules),
	), nil
}

func (h *Handler) retrieveEndpoint(ctx context.Context, endpointID, projectID string) (*datastore.Endpoint, error) {
	endpointRepo := endpointsvc.New(h.A.Logger, h.A.DB)
	return endpointRepo.FindEndpointByID(ctx, endpointID, projectID)
}
