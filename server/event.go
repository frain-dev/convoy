package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CreateAppEvent
// @Summary Create app event
// @Description This endpoint creates an app event
// @Tags Events
// @Accept  json
// @Produce  json
// @Param event body models.Event true "Event Details"
// @Success 200 {object} serverResponse{data=convoy.Event{data=Stub}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /events [post]
func (a *applicationHandler) CreateAppEvent(w http.ResponseWriter, r *http.Request) {

	var newMessage models.Event
	err := json.NewDecoder(r.Body).Decode(&newMessage)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	eventType := newMessage.EventType
	if util.IsStringEmpty(eventType) {
		_ = render.Render(w, r, newErrorResponse("please provide an event_type", http.StatusBadRequest))
		return
	}
	d := newMessage.Data
	if d == nil {
		_ = render.Render(w, r, newErrorResponse("please provide your data", http.StatusBadRequest))
		return
	}

	app, err := a.appRepo.FindApplicationByID(r.Context(), newMessage.AppID)
	if err != nil {

		msg := "an error occurred while retrieving app details"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, convoy.ErrApplicationNotFound) {
			msg = err.Error()
			statusCode = http.StatusNotFound
		}

		log.Debugln("error while fetching app - ", err)

		_ = render.Render(w, r, newErrorResponse(msg, statusCode))
		return
	}

	if len(app.Endpoints) == 0 {
		_ = render.Render(w, r, newErrorResponse("app has no configured endpoints", http.StatusBadRequest))
		return
	}

	matchedEndpoints := matchEndpointsForDelivery(eventType, app.Endpoints, nil)

	event := &convoy.Event{
		UID:              uuid.New().String(),
		EventType:        convoy.EventType(eventType),
		MatchedEndpoints: len(matchedEndpoints),
		Data:             d,
		CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		AppMetadata: &convoy.AppMetadata{
			Title:        app.Title,
			UID:          app.UID,
			GroupID:      app.GroupID,
			SupportEmail: app.SupportEmail,
		},
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	err = a.eventRepo.CreateEvent(r.Context(), event)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while creating event", http.StatusInternalServerError))
		return
	}

	cfg, err := config.Get()
	if err != nil {
		log.Errorln("error fetching config - ", err)
		_ = render.Render(w, r, newErrorResponse("an error has occurred while fetching config", http.StatusInternalServerError))
		return
	}

	var intervalSeconds uint64
	var retryLimit uint64
	if cfg.Strategy.Type == config.DefaultStrategyProvider {
		intervalSeconds = cfg.Strategy.Default.IntervalSeconds
		retryLimit = cfg.Strategy.Default.RetryLimit
	} else {
		_ = render.Render(w, r, newErrorResponse("retry strategy not defined in configuration", http.StatusInternalServerError))
		return
	}

	eventStatus := convoy.ScheduledEventStatus

	for _, v := range matchedEndpoints {
		if v.Status != convoy.ActiveEndpointStatus {
			eventStatus = convoy.DiscardedEventStatus
		}

		eventDelivery := &convoy.EventDelivery{
			UID: uuid.New().String(),
			EventMetadata: &convoy.EventMetadata{
				UID:       event.UID,
				EventType: event.EventType,
			},
			EndpointMetadata: &convoy.EndpointMetadata{
				UID:       v.UID,
				TargetURL: v.TargetURL,
				Status:    v.Status,
				Secret:    v.Secret,
				Sent:      false,
			},
			AppMetadata: &convoy.AppMetadata{
				UID:          app.UID,
				GroupID:      app.GroupID,
				SupportEmail: app.SupportEmail,
			},
			Metadata: &convoy.Metadata{
				Data:            event.Data,
				Strategy:        cfg.Strategy.Type,
				NumTrials:       0,
				IntervalSeconds: intervalSeconds,
				RetryLimit:      retryLimit,
				NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
			},
			Status:           eventStatus,
			DeliveryAttempts: make([]convoy.DeliveryAttempt, 0),
			DocumentStatus:   convoy.ActiveDocumentStatus,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		}
		err = a.eventDeliveryRepo.CreateEventDelivery(r.Context(), eventDelivery)
		if err != nil {
			log.WithError(err).Error("error occurred creating event delivery")
		}

		err = a.eventQueue.Write(r.Context(), convoy.EventProcessor, eventDelivery, 1*time.Second)
		if err != nil {
			log.Errorf("Error occurred sending new event to the queue %s", err)
		}
	}

	_ = render.Render(w, r, newServerResponse("App event created successfully", event, http.StatusCreated))
}

// GetAppEvent
// @Summary Get app event
// @Description This endpoint fetches an app event
// @Tags Events
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Success 200 {object} serverResponse{data=convoy.Event{data=Stub}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /events/{eventID} [get]
func (a *applicationHandler) GetAppEvent(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event fetched successfully",
		*getEventFromContext(r.Context()), http.StatusOK))
}

// GetEventDelivery
// @Summary Get event delivery
// @Description This endpoint fetches an event delivery.
// @Tags EventDelivery
// @Accept json
// @Produce json
// @Param eventID path string true "event id"
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} serverResponse{data=convoy.Event{data=Stub}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries/{eventDeliveryID} [get]
func (a *applicationHandler) GetEventDelivery(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Event Delivery fetched successfully",
		*getEventDeliveryFromContext(r.Context()), http.StatusOK))
}

// ResendEventDelivery
// @Summary Resend an app event
// @Description This endpoint resends an app event
// @Tags EventDelivery
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Success 200 {object} serverResponse{data=convoy.Event{data=Stub}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries/{eventDeliveryID}/resend [put]
func (a *applicationHandler) ResendEventDelivery(w http.ResponseWriter, r *http.Request) {

	eventDelivery := getEventDeliveryFromContext(r.Context())

	if eventDelivery.Status == convoy.SuccessEventStatus {
		_ = render.Render(w, r, newErrorResponse("event already sent", http.StatusBadRequest))
		return
	}

	switch eventDelivery.Status {
	case convoy.ScheduledEventStatus,
		convoy.ProcessingEventStatus,
		convoy.SuccessEventStatus,
		convoy.RetryEventStatus:
		_ = render.Render(w, r, newErrorResponse("cannot resend event that did not fail previously", http.StatusBadRequest))
		return
	}

	// Retry to Inactive endpoints.
	// System cannot handle more than one endpoint per url at this point.
	e := eventDelivery.EndpointMetadata
	endpoint, err := a.appRepo.FindApplicationEndpointByID(context.Background(), eventDelivery.AppMetadata.UID, e.UID)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("cannot find endpoint", http.StatusInternalServerError))
		return
	}

	if endpoint.Status == convoy.PendingEndpointStatus {
		_ = render.Render(w, r, newErrorResponse("endpoint is being re-activated", http.StatusBadRequest))
		return
	}

	if endpoint.Status == convoy.InactiveEndpointStatus {
		pendingEndpoints := []string{e.UID}

		err = a.appRepo.UpdateApplicationEndpointsStatus(context.Background(), eventDelivery.AppMetadata.UID, pendingEndpoints, convoy.PendingEndpointStatus)
		if err != nil {
			_ = render.Render(w, r, newErrorResponse("failed to update endpoint status", http.StatusInternalServerError))
			return
		}
	}

	eventDelivery.Status = convoy.ScheduledEventStatus
	err = a.eventDeliveryRepo.UpdateStatusOfEventDelivery(r.Context(), *eventDelivery, convoy.ScheduledEventStatus)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while trying to resend event", http.StatusInternalServerError))
		return
	}

	err = a.eventQueue.Write(r.Context(), convoy.EventProcessor, eventDelivery, 1*time.Second)
	if err != nil {
		log.WithError(err).Errorf("Error occurred re-enqueing old event - %s", eventDelivery.UID)
	}

	_ = render.Render(w, r, newServerResponse("App event processed for retry successfully",
		eventDelivery, http.StatusOK))
}

// GetEventsPaged
// @Summary Get app events with pagination
// @Description This endpoint fetches app events with pagination
// @Tags Events
// @Accept  json
// @Produce  json
// @Param appId query string false "application id"
// @Param groupId query string false "group id"
// @Param startDate query string false "start date"
// @Param endDate query string false "end date"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]convoy.Event{data=Stub}}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /events [get]
func (a *applicationHandler) GetEventsPaged(w http.ResponseWriter, r *http.Request) {

	pageable := getPageableFromContext(r.Context())
	group := getGroupFromContext(r.Context())
	appID := r.URL.Query().Get("appId")

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	m, paginationData, err := a.eventRepo.LoadEventsPaged(r.Context(), group.UID, appID, searchParams, pageable)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
		log.Errorln("error while fetching events - ", err)
		return
	}

	_ = render.Render(w, r, newServerResponse("App events fetched successfully",
		pagedResponse{Content: &m, Pagination: &paginationData}, http.StatusOK))
}

// GetEventDeliveries
// @Summary Get event deliveries
// @Description This endpoint fetch event deliveries.
// @Tags EventDelivery
// @Accept json
// @Produce json
// @Param appId query string false "application id"
// @Param groupId query string false "group id"
// @Param eventId query string false "event id"
// @Param startDate query string false "start date"
// @Param endDate query string false "end date"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Param status query string false "status"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]convoy.EventDelivery{data=Stub}}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries [get]
func (a *applicationHandler) GetEventDeliveriesPaged(w http.ResponseWriter, r *http.Request) {

	pageable := getPageableFromContext(r.Context())
	group := getGroupFromContext(r.Context())
	appID := r.URL.Query().Get("appId")
	eventID := r.URL.Query().Get("eventId")
	status := r.URL.Query().Get("status")

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	ed, paginationData, err := a.eventDeliveryRepo.LoadEventDeliveriesPaged(r.Context(), group.UID, appID, eventID, convoy.EventDeliveryStatus(status), searchParams, pageable)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
		log.WithError(err)
		return
	}

	_ = render.Render(w, r, newServerResponse("Event deliveries fetched successfully",
		pagedResponse{Content: &ed, Pagination: &paginationData}, http.StatusOK))
}

func getSearchParams(r *http.Request) (models.SearchParams, error) {
	var searchParams models.SearchParams
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
			log.Errorln("error parsing startDate - ", err)
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

	if err := ensurePeriod(startT, endT); err != nil {
		return searchParams, err
	}

	searchParams = models.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	return searchParams, nil
}

func fetchDeliveryAttempts() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			e := getEventDeliveryFromContext(r.Context())

			r = r.WithContext(setDeliveryAttemptsInContext(r.Context(), &e.DeliveryAttempts))
			next.ServeHTTP(w, r)
		})
	}
}

func findMessageDeliveryAttempt(attempts *[]convoy.DeliveryAttempt, id string) (*convoy.DeliveryAttempt, error) {
	for _, a := range *attempts {
		if a.UID == id {
			return &a, nil
		}
	}
	return nil, convoy.ErrEventDeliveryAttemptNotFound
}

func matchEndpointsForDelivery(ev string, endpoints, matched []convoy.Endpoint) []convoy.Endpoint {
	if len(endpoints) == 0 {
		return matched
	}

	if matched == nil {
		matched = make([]convoy.Endpoint, 0)
	}

	e := endpoints[0]
	for _, v := range e.Events {
		if v == ev || v == "*" {
			matched = append(matched, e)
			break
		}
	}

	return matchEndpointsForDelivery(ev, endpoints[1:], matched)
}
