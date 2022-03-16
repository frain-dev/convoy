package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
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
// @Success 200 {object} serverResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /events [post]
func (a *applicationHandler) CreateAppEvent(w http.ResponseWriter, r *http.Request) {

	var newMessage models.Event
	err := util.ReadJSON(r, &newMessage)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	eventType := newMessage.EventType
	d := newMessage.Data

	if err = util.Validate(newMessage); err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	app, err := a.appRepo.FindApplicationByID(r.Context(), newMessage.AppID)
	if err != nil {

		msg := "an error occurred while retrieving app details"
		statusCode := http.StatusInternalServerError

		if errors.Is(err, datastore.ErrApplicationNotFound) {
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

	if app.IsDisabled {
		_ = render.Render(w, r, newServerResponse("app is disabled, no events were sent", nil, http.StatusOK))
		return
	}

	matchedEndpoints := matchEndpointsForDelivery(eventType, app.Endpoints, nil)

	event := &datastore.Event{
		UID:              uuid.New().String(),
		EventType:        datastore.EventType(eventType),
		MatchedEndpoints: len(matchedEndpoints),
		Data:             d,
		CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		AppMetadata: &datastore.AppMetadata{
			Title:        app.Title,
			UID:          app.UID,
			GroupID:      app.GroupID,
			SupportEmail: app.SupportEmail,
		},
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err = a.eventRepo.CreateEvent(r.Context(), event)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while creating event", http.StatusInternalServerError))
		return
	}

	g := getGroupFromContext(r.Context())

	var intervalSeconds uint64
	var retryLimit uint64
	if string(g.Config.Strategy.Type) == string(config.DefaultStrategyProvider) {
		intervalSeconds = g.Config.Strategy.Default.IntervalSeconds
		retryLimit = g.Config.Strategy.Default.RetryLimit
	} else if string(g.Config.Strategy.Type) == string(config.ExponentialBackoffStrategyProvider) {
		intervalSeconds = 0
		retryLimit = g.Config.Strategy.ExponentialBackoff.RetryLimit
	} else {
		_ = render.Render(w, r, newErrorResponse("retry strategy not defined in configuration", http.StatusInternalServerError))
		return
	}

	for _, v := range matchedEndpoints {
		eventDelivery := &datastore.EventDelivery{
			UID: uuid.New().String(),
			EventMetadata: &datastore.EventMetadata{
				UID:       event.UID,
				EventType: event.EventType,
			},
			EndpointMetadata: &datastore.EndpointMetadata{
				UID:       v.UID,
				TargetURL: v.TargetURL,
				Status:    v.Status,
				Secret:    v.Secret,
				Sent:      false,
			},
			AppMetadata: &datastore.AppMetadata{
				UID:          app.UID,
				Title:        app.Title,
				GroupID:      app.GroupID,
				SupportEmail: app.SupportEmail,
			},
			Metadata: &datastore.Metadata{
				Data:            event.Data,
				Strategy:        g.Config.Strategy.Type,
				NumTrials:       0,
				IntervalSeconds: intervalSeconds,
				RetryLimit:      retryLimit,
				NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
			},
			Status:           getEventDeliveryStatus(v),
			DeliveryAttempts: make([]datastore.DeliveryAttempt, 0),
			DocumentStatus:   datastore.ActiveDocumentStatus,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		}
		err = a.eventDeliveryRepo.CreateEventDelivery(r.Context(), eventDelivery)
		if err != nil {
			log.WithError(err).Error("error occurred creating event delivery")
		}

		taskName := convoy.EventProcessor.SetPrefix(g.Name)

		if eventDelivery.Status != datastore.DiscardedEventStatus {
			err = a.eventQueue.Write(r.Context(), taskName, eventDelivery, 1*time.Second)
			if err != nil {
				log.Errorf("Error occurred sending new event to the queue %s", err)
			}
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
// @Success 200 {object} serverResponse{data=datastore.Event{data=Stub}}
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
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} serverResponse{data=datastore.Event{data=Stub}}
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
// @Param eventDeliveryID path string true "event delivery id"
// @Success 200 {object} serverResponse{data=datastore.Event{data=Stub}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries/{eventDeliveryID}/resend [put]
func (a *applicationHandler) ResendEventDelivery(w http.ResponseWriter, r *http.Request) {

	eventDelivery := getEventDeliveryFromContext(r.Context())

	err := a.retryEventDelivery(r.Context(), eventDelivery)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, newServerResponse("App event processed for retry successfully",
		eventDelivery, http.StatusOK))
}

// BatchRetryEventDelivery
// @Summary Batch Resend app events
// @Description This endpoint resends multiple app events
// @Tags EventDelivery
// @Accept json
// @Produce json
// @Param delivery ids body Stub{ids=[]string} true "event delivery ids"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries/batchretry [post]
func (a *applicationHandler) BatchRetryEventDelivery(w http.ResponseWriter, r *http.Request) {
	pageable := datastore.Pageable{
		Page:    0,
		PerPage: 1000000000000, // large number so we get everything in most cases
		Sort:    -1,
	}
	group := getGroupFromContext(r.Context())
	appID := r.URL.Query().Get("appId")
	eventID := r.URL.Query().Get("eventId")
	status := make([]datastore.EventDeliveryStatus, 0)

	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	deliveries, _, err := a.eventDeliveryRepo.LoadEventDeliveriesPaged(r.Context(), group.UID, appID, eventID, status, searchParams, pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch event deliveries by ids")
		_ = render.Render(w, r, newErrorResponse("failed to fetch event deliveries", http.StatusInternalServerError))
		return
	}

	ctx := r.Context()
	failures := 0

	for _, delivery := range deliveries {
		err := a.retryEventDelivery(ctx, &delivery)
		if err != nil {
			failures++
			log.WithError(err).Error("an item in the batch retry failed")
		}
	}

	_ = render.Render(w, r, newServerResponse(fmt.Sprintf("%d successful, %d failed", len(deliveries)-failures, failures), nil, http.StatusOK))
}

// CountAffectedEventDeliveries
// @Summary Count affected eventDeliveries
// @Description This endpoint counts app events that will be affected by a batch retry operation
// @Tags EventDelivery
// @Accept  json
// @Produce  json
// @Param appId query string false "application id"
// @Param groupId query string false "group id"
// @Param startDate query string false "start date"
// @Param endDate query string false "end date"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse{data=Stub{num=integer}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries/countbatchretryevents [get]
func (a *applicationHandler) CountAffectedEventDeliveries(w http.ResponseWriter, r *http.Request) {
	group := getGroupFromContext(r.Context())
	appID := r.URL.Query().Get("appId")
	eventID := r.URL.Query().Get("eventId")
	status := make([]datastore.EventDeliveryStatus, 0)

	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	count, err := a.eventDeliveryRepo.CountEventDeliveries(r.Context(), group.UID, appID, eventID, status, searchParams)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
		log.WithError(err).Error("an error occurred while fetching event deliveries")
		return
	}

	_ = render.Render(w, r, newServerResponse("event deliveries count successful", map[string]interface{}{"num": count}, http.StatusOK))
}

// ForceResendEventDeliveries
// @Summary Force Resend app events
// @Description This endpoint force resends multiple app events
// @Tags EventDelivery
// @Accept json
// @Produce json
// @Param delivery ids body Stub{ids=[]string} true "event delivery ids"
// @Success 200 {object} serverResponse{data=Stub}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries/forceresend [post]
func (a *applicationHandler) ForceResendEventDeliveries(w http.ResponseWriter, r *http.Request) {
	eventDeliveryIDs := models.IDs{}

	err := json.NewDecoder(r.Body).Decode(&eventDeliveryIDs)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
		return
	}

	var deliveries []datastore.EventDelivery

	deliveries, err = a.eventDeliveryRepo.FindEventDeliveriesByIDs(r.Context(), eventDeliveryIDs.IDs)
	if err != nil {
		log.WithError(err).Error("failed to fetch event deliveries by ids")
		_ = render.Render(w, r, newErrorResponse("failed to fetch event deliveries", http.StatusInternalServerError))
		return
	}

	ctx := r.Context()
	failures := 0

	for _, delivery := range deliveries {
		err := a.forceResendEventDelivery(ctx, &delivery)
		if err != nil {
			failures++
			log.WithError(err).Error("an item in the force resend batch failed")
		}
	}

	_ = render.Render(w, r, newServerResponse(fmt.Sprintf("%d successful, %d failed", len(deliveries)-failures, failures), nil, http.StatusOK))
}

func (a *applicationHandler) retryEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery) error {
	if eventDelivery.Status == datastore.SuccessEventStatus {
		return errors.New("event already sent")
	}

	switch eventDelivery.Status {
	case datastore.ScheduledEventStatus,
		datastore.ProcessingEventStatus,
		datastore.RetryEventStatus:
		return errors.New("cannot resend event that did not fail previously")
	}

	e := eventDelivery.EndpointMetadata
	endpoint, err := a.appRepo.FindApplicationEndpointByID(context.Background(), eventDelivery.AppMetadata.UID, e.UID)
	if err != nil {
		return errors.New("cannot find endpoint")
	}

	if endpoint.Status == datastore.PendingEndpointStatus {
		return errors.New("endpoint is being re-activated")
	}

	if endpoint.Status == datastore.InactiveEndpointStatus {
		pendingEndpoints := []string{e.UID}

		err = a.appRepo.UpdateApplicationEndpointsStatus(context.Background(), eventDelivery.AppMetadata.UID, pendingEndpoints, datastore.PendingEndpointStatus)
		if err != nil {
			return errors.New("failed to update endpoint status")
		}
	}

	return a.requeueEventDelivery(ctx, eventDelivery)
}

func (a *applicationHandler) forceResendEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery) error {
	if eventDelivery.Status != datastore.SuccessEventStatus {
		return errors.New("only successful events can be force resent")
	}

	e := eventDelivery.EndpointMetadata
	endpoint, err := a.appRepo.FindApplicationEndpointByID(context.Background(), eventDelivery.AppMetadata.UID, e.UID)
	if err != nil {
		return errors.New("cannot find endpoint")
	}

	if endpoint.Status != datastore.ActiveEndpointStatus {
		return errors.New("force resend to an inactive or pending endpoint is not allowed")
	}

	return a.requeueEventDelivery(ctx, eventDelivery)
}

func (a *applicationHandler) requeueEventDelivery(ctx context.Context, eventDelivery *datastore.EventDelivery) error {
	eventDelivery.Status = datastore.ScheduledEventStatus
	err := a.eventDeliveryRepo.UpdateStatusOfEventDelivery(ctx, *eventDelivery, datastore.ScheduledEventStatus)
	if err != nil {
		return errors.New("an error occurred while trying to resend event")
	}

	g := getGroupFromContext(ctx)
	taskName := convoy.EventProcessor.SetPrefix(g.Name)
	err = a.eventQueue.Write(ctx, taskName, eventDelivery, 1*time.Second)
	if err != nil {
		return fmt.Errorf("error occurred re-enqueing old event - %s: %v", eventDelivery.UID, err)
	}
	return nil
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
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.Event{data=Stub}}}
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

// GetEventDeliveriesPaged
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
// @Param status query []string false "status"
// @Success 200 {object} serverResponse{data=pagedResponse{content=[]datastore.EventDelivery{data=Stub}}}
// @Failure 400,401,500 {object} serverResponse{data=Stub}
// @Security ApiKeyAuth
// @Router /eventdeliveries [get]
func (a *applicationHandler) GetEventDeliveriesPaged(w http.ResponseWriter, r *http.Request) {

	pageable := getPageableFromContext(r.Context())
	group := getGroupFromContext(r.Context())
	appID := r.URL.Query().Get("appId")
	eventID := r.URL.Query().Get("eventId")
	status := make([]datastore.EventDeliveryStatus, 0)

	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	ed, paginationData, err := a.eventDeliveryRepo.LoadEventDeliveriesPaged(r.Context(), group.UID, appID, eventID, status, searchParams, pageable)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
		log.WithError(err)
		return
	}

	_ = render.Render(w, r, newServerResponse("Event deliveries fetched successfully",
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

	searchParams = datastore.SearchParams{
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

func findMessageDeliveryAttempt(attempts *[]datastore.DeliveryAttempt, id string) (*datastore.DeliveryAttempt, error) {
	for _, a := range *attempts {
		if a.UID == id {
			return &a, nil
		}
	}
	return nil, datastore.ErrEventDeliveryAttemptNotFound
}

func matchEndpointsForDelivery(ev string, endpoints, matched []datastore.Endpoint) []datastore.Endpoint {
	if len(endpoints) == 0 {
		return matched
	}

	if matched == nil {
		matched = make([]datastore.Endpoint, 0)
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

func getEventDeliveryStatus(endpoint datastore.Endpoint) datastore.EventDeliveryStatus {
	if endpoint.Status != datastore.ActiveEndpointStatus {
		return datastore.DiscardedEventStatus
	}

	return datastore.ScheduledEventStatus
}
