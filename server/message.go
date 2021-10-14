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

// CreateAppMessage
// @Summary Create app message
// @Description This endpoint creates an app message
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param message body models.Message true "Message Details"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events [post]
func (a *applicationHandler) CreateAppMessage(w http.ResponseWriter, r *http.Request) {

	var newMessage models.Message
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

	messageStatus := convoy.ScheduledMessageStatus
	activeEndpoints := util.ParseMetadataFromActiveEndpoints(app.Endpoints)
	if len(activeEndpoints) == 0 {
		messageStatus = convoy.DiscardedMessageStatus
		activeEndpoints = util.GetMetadataFromEndpoints(app.Endpoints)
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

	msg := &convoy.Message{
		UID:       uuid.New().String(),
		AppID:     app.UID,
		EventType: convoy.EventType(eventType),
		Data:      d,
		Metadata: &convoy.MessageMetadata{
			Strategy:        cfg.Strategy.Type,
			NumTrials:       0,
			IntervalSeconds: intervalSeconds,
			RetryLimit:      retryLimit,
			NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
		},
		MessageAttempts: make([]convoy.MessageAttempt, 0),
		CreatedAt:       primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:       primitive.NewDateTimeFromTime(time.Now()),
		AppMetadata: &convoy.AppMetadata{
			GroupID:      app.GroupID,
			Secret:       app.Secret,
			SupportEmail: app.SupportEmail,
			Endpoints:    activeEndpoints,
		},
		Status:         messageStatus,
		DocumentStatus: convoy.ActiveDocumentStatus,
	}

	err = a.msgRepo.CreateMessage(r.Context(), msg)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while creating event", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App event created successfully", msg, http.StatusCreated))
}

// GetAppMessage
// @Summary Get app message
// @Description This endpoint fetches an app message
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events/{eventID} [get]
func (a *applicationHandler) GetAppMessage(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("App event fetched successfully",
		*getMessageFromContext(r.Context()), http.StatusOK))
}

// ResendAppMessage
// @Summary Resend an app message
// @Description This endpoint resends an app message
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param eventID path string true "event id"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events/{eventID}/resend [put]
func (a *applicationHandler) ResendAppMessage(w http.ResponseWriter, r *http.Request) {

	msg := getMessageFromContext(r.Context())

	if msg.Status == convoy.SuccessMessageStatus {
		_ = render.Render(w, r, newErrorResponse("event already sent", http.StatusBadRequest))
		return
	}

	switch msg.Status {
	case convoy.ScheduledMessageStatus,
		convoy.ProcessingMessageStatus,
		convoy.SuccessMessageStatus,
		convoy.RetryMessageStatus:
		_ = render.Render(w, r, newErrorResponse("cannot resend event that did not fail previously", http.StatusBadRequest))
		return
	}

	// Retry to Inactive endpoints.
	// System cannot handle more than one endpoint per url at this point.
	e := msg.AppMetadata.Endpoints[0]
	endpoint, err := a.appRepo.FindApplicationEndpointByID(context.Background(), msg.AppID, e.UID)
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

		err = a.appRepo.UpdateApplicationEndpointsStatus(context.Background(), msg.AppID, pendingEndpoints, convoy.PendingEndpointStatus)
		if err != nil {
			_ = render.Render(w, r, newErrorResponse("failed to update endpoint status", http.StatusInternalServerError))
			return
		}
	}

	msg.Status = convoy.ScheduledMessageStatus
	err = a.msgRepo.UpdateStatusOfMessages(r.Context(), []convoy.Message{*msg}, convoy.ScheduledMessageStatus)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while trying to resend event", http.StatusInternalServerError))
		return
	}

	_ = render.Render(w, r, newServerResponse("App event processed for retry successfully",
		msg, http.StatusOK))
}

// GetMessagesPaged
// @Summary Get app messages with pagination
// @Description This endpoint fetches app messages with pagination
// @Tags Messages
// @Accept  json
// @Produce  json
// @Param appId query string false "application id"
// @Param groupId query string false "group id"
// @Param startDate query string false "start date"
// @Param endDate query string false "end date"
// @Param perPage query string false "results per page"
// @Param page query string false "page number"
// @Param sort query string false "sort order"
// @Success 200 {object} serverResponse
// @Failure 400 {object} serverResponse
// @Failure 401 {object} serverResponse
// @Failure 500 {object} serverResponse
// @Security ApiKeyAuth
// @Router /events [get]
func (a *applicationHandler) GetMessagesPaged(w http.ResponseWriter, r *http.Request) {

	pageable := getPageableFromContext(r.Context())
	groupID := r.URL.Query().Get("groupId")
	appID := r.URL.Query().Get("appId")

	searchParams, err := getSearchParams(r)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	m, paginationData, err := a.msgRepo.LoadMessagesPaged(r.Context(), groupID, appID, searchParams, pageable)
	if err != nil {
		_ = render.Render(w, r, newErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
		log.Errorln("error while fetching events - ", err)
		return
	}

	_ = render.Render(w, r, newServerResponse("App events fetched successfully",
		pagedResponse{Content: &m, Pagination: &paginationData}, http.StatusOK))
}

func fetchAllMessages(msgRepo convoy.MessageRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			pageable := getPageableFromContext(r.Context())

			groupID := r.URL.Query().Get("groupId")
			appId := r.URL.Query().Get("appId")

			searchParams, err := getSearchParams(r)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			m, paginationData, err := msgRepo.LoadMessagesPaged(r.Context(), groupID, appId, searchParams, pageable)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching app events", http.StatusInternalServerError))
				log.Errorln("error while fetching events - ", err)
				return
			}

			r = r.WithContext(setMessagesInContext(r.Context(), &m))
			r = r.WithContext(setPaginationDataInContext(r.Context(), &paginationData))
			next.ServeHTTP(w, r)
		})
	}
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

func fetchMessageDeliveryAttempts() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			msg := getMessageFromContext(r.Context())

			r = r.WithContext(setDeliveryAttemptsInContext(r.Context(), &msg.MessageAttempts))
			next.ServeHTTP(w, r)
		})
	}
}

func findMessageDeliveryAttempt(attempts *[]convoy.MessageAttempt, id string) (*convoy.MessageAttempt, error) {
	for _, a := range *attempts {
		if a.UID == id {
			return &a, nil
		}
	}
	return nil, convoy.ErrMessageDeliveryAttemptNotFound
}
