package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

func createEventService(a *ApplicationHandler) *services.EventService {
	sourceRepo := postgres.NewSourceRepo(a.A.DB)
	endpointRepo := postgres.NewEndpointRepo(a.A.DB)
	subRepo := postgres.NewSubscriptionRepo(a.A.DB)
	eventRepo := postgres.NewEventRepo(a.A.DB)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.A.DB)
	deviceRepo := postgres.NewDeviceRepo(a.A.DB)

	return services.NewEventService(
		endpointRepo, eventRepo, eventDeliveryRepo,
		a.A.Queue, a.A.Cache, a.A.Searcher, subRepo, sourceRepo, deviceRepo,
	)
}

// CreateEndpointEvent
// @Summary Create an event
// @Description This endpoint creates an endpoint event
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param event body models.Event true "Event Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/events [post]
func (a *ApplicationHandler) CreateEndpointEvent(w http.ResponseWriter, r *http.Request) {
	var newMessage models.Event
	err := util.ReadJSON(r, &newMessage)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	g := m.GetProjectFromContext(r.Context())
	eventService := createEventService(a)

	event, err := eventService.CreateEvent(r.Context(), &newMessage, g)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint event created successfully", event, http.StatusCreated))
}

// CreateEndpointFanoutEvent
// @Summary Fan out an event
// @Description This endpoint uses the owner_id to fan out an event to multiple endpoints.
// @Tags Events
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param event body models.FanoutEvent true "Event Details"
// @Success 200 {object} util.ServerResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/events/fanout [post]
func (a *ApplicationHandler) CreateEndpointFanoutEvent(w http.ResponseWriter, r *http.Request) {
	var newMessage models.FanoutEvent
	err := util.ReadJSON(r, &newMessage)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	g := m.GetProjectFromContext(r.Context())
	eventService := createEventService(a)

	event, err := eventService.CreateFanoutEvent(r.Context(), &newMessage, g)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint event created successfully", event, http.StatusCreated))
}

// ReplayEndpointEvent
// @Summary Replay event
// @Description This endpoint replays an event afresh assuming it is a new event.
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param eventID path string true "event id"
// @Success 200 {object} util.ServerResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/events/{eventID}/replay [put]
func (a *ApplicationHandler) ReplayEndpointEvent(w http.ResponseWriter, r *http.Request) {
	g := m.GetProjectFromContext(r.Context())
	event := m.GetEventFromContext(r.Context())
	eventService := createEventService(a)

	err := eventService.ReplayEvent(r.Context(), event, g)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Endpoint event replayed successfully", event, http.StatusOK))
}

// BatchReplayEvents
// @Summary Batch replay events
// @Description This endpoint replays multiple events at once.
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param startDate query string false "start date"
// @Param endDate query string false "end date"
// @Param source query string false "Source ID"
// @Success 200 {object} util.ServerResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/events/batchreplay [post]
func (a *ApplicationHandler) BatchReplayEvents(w http.ResponseWriter, r *http.Request) {
	p := m.GetProjectFromContext(r.Context())
	eventService := createEventService(a)

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	f := &datastore.Filter{
		Project: p,
		Pageable: datastore.Pageable{
			Direction:  datastore.Next,
			PerPage:    1000000000000, // large number so we get everything in most cases
			NextCursor: datastore.DefaultCursor,
		},
		SourceID:     r.URL.Query().Get("sourceId"),
		EndpointID:   r.URL.Query().Get("endpointId"),
		SearchParams: searchParams,
	}

	successes, failures, err := eventService.BatchReplayEvents(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
}

func (a *ApplicationHandler) CountAffectedEvents(w http.ResponseWriter, r *http.Request) {
	p := m.GetProjectFromContext(r.Context())
	eventService := createEventService(a)

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	f := &datastore.Filter{
		Project: p,
		Pageable: datastore.Pageable{
			Direction:  datastore.Next,
			PerPage:    1000000000000, // large number so we get everything in most cases
			NextCursor: datastore.DefaultCursor,
		},
		SourceID:     r.URL.Query().Get("sourceId"),
		EndpointID:   r.URL.Query().Get("endpointId"),
		SearchParams: searchParams,
	}

	count, err := eventService.CountAffectedEvents(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("events count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

// GetEndpointEvent
// @Summary Retrieve an event
// @Description This endpoint retrieves an event
// @Tags Events
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param eventID path string true "event id"
// @Success 200 {object} util.ServerResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/events/{eventID} [get]
func (a *ApplicationHandler) GetEndpointEvent(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("Endpoint event fetched successfully",
		*m.GetEventFromContext(r.Context()), http.StatusOK))
}

// GetEventDelivery
// @Summary Retrieve an event delivery
// @Description This endpoint fetches an event delivery.
// @Tags Event Deliveries
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} util.ServerResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID} [get]
func (a *ApplicationHandler) GetEventDelivery(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("Event Delivery fetched successfully",
		*m.GetEventDeliveryFromContext(r.Context()), http.StatusOK))
}

// ResendEventDelivery
// @Summary Retry event delivery
// @Description This endpoint retries an event delivery.
// @Tags Event Deliveries
// @Accept  json
// @Produce  json
// @Param projectID path string true "Project ID"
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} util.ServerResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/eventdeliveries/{eventDeliveryID}/resend [put]
func (a *ApplicationHandler) ResendEventDelivery(w http.ResponseWriter, r *http.Request) {
	eventDelivery := m.GetEventDeliveryFromContext(r.Context())
	eventService := createEventService(a)

	err := eventService.ResendEventDelivery(r.Context(), eventDelivery, m.GetProjectFromContext(r.Context()))
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App event processed for retry successfully",
		eventDelivery, http.StatusOK))
}

// BatchRetryEventDelivery
// @Summary Batch retry event delivery
// @Description This endpoint batch retries multiple event deliveries at once.
// @Tags Event Deliveries
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param deliveryIds body Stub{ids=[]string} true "event delivery ids"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/eventdeliveries/batchretry [post]
func (a *ApplicationHandler) BatchRetryEventDelivery(w http.ResponseWriter, r *http.Request) {
	var endpoints []string
	status := make([]datastore.EventDeliveryStatus, 0)

	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := m.GetEndpointIDFromContext(r)
	endpointIDs := m.GetEndpointIDsFromContext(r)

	if !util.IsStringEmpty(endpointID) {
		endpoints = []string{endpointID}
	}

	if len(endpointIDs) > 0 {
		endpoints = endpointIDs
	}

	f := &datastore.Filter{
		Project:     m.GetProjectFromContext(r.Context()),
		EndpointIDs: endpoints,
		EventID:     r.URL.Query().Get("eventId"),
		Status:      status,
		Pageable: datastore.Pageable{
			Direction:  datastore.Next,
			PerPage:    1000000000000, // large number so we get everything in most cases
			NextCursor: datastore.DefaultCursor,
		},
		SearchParams: searchParams,
	}

	eventService := createEventService(a)
	successes, failures, err := eventService.BatchRetryEventDelivery(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
}

func (a *ApplicationHandler) CountAffectedEventDeliveries(w http.ResponseWriter, r *http.Request) {
	var endpoints []string
	status := make([]datastore.EventDeliveryStatus, 0)
	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := m.GetEndpointIDFromContext(r)
	endpointIDs := m.GetEndpointIDsFromContext(r)

	if !util.IsStringEmpty(endpointID) {
		endpoints = []string{endpointID}
	}

	if len(endpointIDs) > 0 {
		endpoints = endpointIDs
	}

	f := &datastore.Filter{
		Project:      m.GetProjectFromContext(r.Context()),
		EndpointIDs:  endpoints,
		EventID:      r.URL.Query().Get("eventId"),
		Status:       status,
		SearchParams: searchParams,
	}

	eventService := createEventService(a)
	count, err := eventService.CountAffectedEventDeliveries(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("event deliveries count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

// ForceResendEventDeliveries
// @Summary Force retry event delivery
// @Description This endpoint enables you retry a previously successful event delivery
// @Tags Event Deliveries
// @Accept json
// @Produce json
// @Param projectID path string true "Project ID"
// @Param deliveryIds body Stub{ids=[]string} true "event delivery ids"
// @Success 200 {object} util.ServerResponse{data=Stub}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/eventdeliveries/forceresend [post]
func (a *ApplicationHandler) ForceResendEventDeliveries(w http.ResponseWriter, r *http.Request) {
	eventDeliveryIDs := models.IDs{}

	err := json.NewDecoder(r.Body).Decode(&eventDeliveryIDs)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	eventService := createEventService(a)
	successes, failures, err := eventService.ForceResendEventDeliveries(r.Context(), eventDeliveryIDs.IDs, m.GetProjectFromContext(r.Context()))
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
// @Param appId query string false "application id"
// @Param projectID path string true "Project ID"
// @Param sourceId query string false "source id"
// @Param startDate query string false "start date"
// @Param endDate query string false "end date"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.Event{data=Stub}}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/events [get]
func (a *ApplicationHandler) GetEventsPaged(w http.ResponseWriter, r *http.Request) {
	var endpoints []string

	cfg, err := config.Get()
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	pageable := m.GetPageableFromContext(r.Context())
	project := m.GetProjectFromContext(r.Context())
	query := r.URL.Query().Get("query")
	endpointID := m.GetEndpointIDFromContext(r)
	endpointIDs := m.GetEndpointIDsFromContext(r)

	if !util.IsStringEmpty(endpointID) {
		endpoints = []string{endpointID}
	}

	if len(endpointIDs) > 0 {
		endpoints = endpointIDs
	}

	f := &datastore.Filter{
		Query:        query,
		Project:      project,
		EndpointID:   endpointID,
		EndpointIDs:  endpoints,
		SourceID:     m.GetSourceIDFromContext(r),
		Pageable:     pageable,
		SearchParams: searchParams,
	}

	if cfg.Search.Type == config.TypesenseSearchProvider && !util.IsStringEmpty(query) {
		eventService := createEventService(a)
		m, paginationData, err := eventService.Search(r.Context(), f)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
		_ = render.Render(w, r, util.NewServerResponse("Endpoint events fetched successfully",
			pagedResponse{Content: &m, Pagination: &paginationData}, http.StatusOK))

		return
	}

	eventService := createEventService(a)
	m, paginationData, err := eventService.GetEventsPaged(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
		pagedResponse{Content: &m, Pagination: &paginationData}, http.StatusOK))
}

// GetEventDeliveriesPaged
// @Summary List all event deliveries
// @Description This endpoint retrieves all event deliveries paginated.
// @Tags Event Deliveries
// @Accept json
// @Produce json
// @Param appId query string false "application id"
// @Param projectID path string true "Project ID"
// @Param eventId query string false "event id"
// @Param startDate query string false "start date"
// @Param endDate query string false "end date"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Param status query []string false "status"
// @Success 200 {object} util.ServerResponse{data=pagedResponse{content=[]datastore.EventDelivery{data=Stub}}}
// @Failure 400,401,404 {object} util.ServerResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /api/v1/projects/{projectID}/eventdeliveries [get]
func (a *ApplicationHandler) GetEventDeliveriesPaged(w http.ResponseWriter, r *http.Request) {
	status := make([]datastore.EventDeliveryStatus, 0)
	var endpoints []string
	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	endpointID := m.GetEndpointIDFromContext(r)
	endpointIDs := m.GetEndpointIDsFromContext(r)

	if !util.IsStringEmpty(endpointID) {
		endpoints = []string{endpointID}
	}

	if len(endpointIDs) > 0 {
		endpoints = endpointIDs
	}

	f := &datastore.Filter{
		Project:      m.GetProjectFromContext(r.Context()),
		EventID:      r.URL.Query().Get("eventId"),
		EndpointIDs:  endpoints,
		Status:       status,
		Pageable:     m.GetPageableFromContext(r.Context()),
		SearchParams: searchParams,
	}

	eventService := createEventService(a)
	ed, paginationData, err := eventService.GetEventDeliveriesPaged(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Event deliveries fetched successfully",
		pagedResponse{Content: &ed, Pagination: &paginationData}, http.StatusOK))
}

func getSearchParams(r *http.Request) (datastore.SearchParams, error) {
	var searchParams datastore.SearchParams
	format := "2006-01-02T15:04:05"
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	var err error

	var startT time.Time
	if len(startDate) == 0 {
		startT = time.Unix(0, 0)
	} else {
		startT, err = time.Parse(format, startDate)
		if err != nil {
			return searchParams, errors.New("please specify a startDate in the format " + format)
		}
	}
	var endT time.Time
	if len(endDate) == 0 {
		now := time.Now()
		endT = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	} else {
		endT, err = time.Parse(format, endDate)
		if err != nil {
			return searchParams, errors.New("please specify a correct endDate in the format " + format + " or none at all")
		}
	}

	if err := m.EnsurePeriod(startT, endT); err != nil {
		return searchParams, err
	}

	searchParams = datastore.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	return searchParams, nil
}

func fetchDeliveryAttempts() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			e := m.GetEventDeliveryFromContext(r.Context())

			r = r.WithContext(m.SetDeliveryAttemptsInContext(r.Context(), (*[]datastore.DeliveryAttempt)(&e.DeliveryAttempts)))
			next.ServeHTTP(w, r)
		})
	}
}

func FindMessageDeliveryAttempt(attempts *[]datastore.DeliveryAttempt, id string) (*datastore.DeliveryAttempt, error) {
	for _, a := range *attempts {
		if a.UID == id {
			return &a, nil
		}
	}
	return nil, datastore.ErrEventDeliveryAttemptNotFound
}
