package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
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

	g := getGroupFromContext(r.Context())

	event, err := a.eventService.CreateAppEvent(r.Context(), &newMessage, g)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
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

	err := a.eventService.ResendEventDelivery(r.Context(), eventDelivery, getGroupFromContext(r.Context()))
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
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

	f := &datastore.Filter{
		Group:   getGroupFromContext(r.Context()),
		AppID:   r.URL.Query().Get("appId"),
		EventID: r.URL.Query().Get("eventId"),
		Status:  status,
		Pageable: datastore.Pageable{
			Page:    0,
			PerPage: 1000000000000, // large number so we get everything in most cases
			Sort:    -1,
		},
		SearchParams: searchParams,
	}

	successes, failures, err := a.eventService.BatchRetryEventDelivery(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
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

	count, err := a.eventService.CountAffectedEventDeliveries(r.Context(), group, appID, eventID, status, searchParams)
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
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

	successes, failures, err := a.eventService.ForceResendEventDeliveries(r.Context(), eventDeliveryIDs.IDs, getGroupFromContext(r.Context()))
	if err != nil {
		_ = render.Render(w, r, newServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, newServerResponse(fmt.Sprintf("%d successful, %d failed", successes, failures), nil, http.StatusOK))
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
	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	f := &datastore.Filter{
		Group:        getGroupFromContext(r.Context()),
		AppID:        r.URL.Query().Get("appId"),
		Pageable:     getPageableFromContext(r.Context()),
		SearchParams: searchParams,
	}

	m, paginationData, err := a.eventService.GetEventsPaged(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
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

	f := &datastore.Filter{
		Group:        getGroupFromContext(r.Context()),
		AppID:        r.URL.Query().Get("appId"),
		EventID:      r.URL.Query().Get("eventId"),
		Status:       status,
		Pageable:     getPageableFromContext(r.Context()),
		SearchParams: searchParams,
	}

	ed, paginationData, err := a.eventService.GetEventDeliveriesPaged(r.Context(), f)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching event deliveries", http.StatusInternalServerError))
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
