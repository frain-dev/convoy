package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/felixge/httpsnoop"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

type contextKey string

const (
	groupCtx            contextKey = "group"
	appCtx              contextKey = "app"
	endpointCtx         contextKey = "endpoint"
	msgCtx              contextKey = "message"
	configCtx           contextKey = "configCtx"
	authConfigCtx       contextKey = "authConfig"
	authLoginCtx        contextKey = "authLogin"
	pageableCtx         contextKey = "pageable"
	pageDataCtx         contextKey = "pageData"
	dashboardCtx        contextKey = "dashboard"
	deliveryAttemptsCtx contextKey = "deliveryAttempts"
)

func instrumentPath(path string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m := httpsnoop.CaptureMetrics(next, w, r)
			requestDuration.WithLabelValues(r.Method, path,
				strconv.Itoa(m.Code)).Observe(m.Duration.Seconds())
		})
	}
}

func writeRequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", r.Context().Value(middleware.RequestIDKey).(string))
		next.ServeHTTP(w, r)
	})
}

func ensureBasicAuthFromRequest(a *config.AuthConfiguration, r *http.Request) error {
	val := r.Header.Get("Authorization")
	auth := strings.Split(val, " ")

	if len(auth) != 2 {
		return errors.New("invalid header structure")
	}
	if len(auth) != 2 {
		return errors.New("invalid auth header structure")
	}

	if strings.ToUpper(auth[0]) != "BASIC" {
		return errors.New("invalid auth header structure")
	}

	credentials, err := base64.StdEncoding.DecodeString(auth[1])
	if err != nil {
		return errors.New("invalid credentials")
	}

	if string(credentials) != fmt.Sprintf("%s:%s", a.Basic.Username, a.Basic.Password) {
		return errors.New("authorization failed")
	}

	return nil
}

// func retrieveRequestID(r *http.Request) string { return middleware.GetReqID(r.Context()) }

func jsonResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// TODO: Remove this cors filter bit
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if r.Method == "OPTIONS" {
			return
		}

		next.ServeHTTP(w, r)
	})
}

// func requireNoAuth(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

// 		// val, err := tokenFromRequest(r)
// 		// if err == nil || !val.IsZero() {
// 		// 	render.Render(w, r, models.ErrAccessDenied)
// 		// 	return
// 		// }

// 		next.ServeHTTP(w, r)
// 	})
// }

func requireApp(appRepo convoy.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			appID := chi.URLParam(r, "appID")

			app, err := appRepo.FindApplicationByID(r.Context(), appID)
			if err != nil {

				msg := "an error occurred while retrieving app details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, convoy.ErrApplicationNotFound) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			r = r.WithContext(setApplicationInContext(r.Context(), app))
			next.ServeHTTP(w, r)
		})
	}
}

func filterDeletedEndpoints(endpoints []convoy.Endpoint) []convoy.Endpoint {
	activeEndpoints := make([]convoy.Endpoint, 0)
	for _, endpoint := range endpoints {
		if endpoint.DeletedAt == 0 {
			activeEndpoints = append(activeEndpoints, endpoint)
		}
	}
	return activeEndpoints
}

func parseEndpointFromBody(body io.ReadCloser) (models.Endpoint, error) {
	var e models.Endpoint
	err := json.NewDecoder(body).Decode(&e)
	if err != nil {
		return e, errors.New("request is invalid")
	}

	description := e.Description
	if util.IsStringEmpty(description) {
		return e, errors.New("please provide a description")
	}

	e.URL, err = util.CleanEndpoint(e.URL)
	if err != nil {
		return e, err
	}

	return e, nil
}

func requireAppEndpoint() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			app := getApplicationFromContext(r.Context())
			endPointId := chi.URLParam(r, "endpointID")

			endpoint, err := findEndpoint(&app.Endpoints, endPointId)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			r = r.WithContext(setApplicationEndpointInContext(r.Context(), endpoint))
			next.ServeHTTP(w, r)
		})
	}
}

func requireMessage(msgRepo convoy.MessageRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			msgId := chi.URLParam(r, "eventID")

			msg, err := msgRepo.FindMessageByID(r.Context(), msgId)
			if err != nil {

				msg := "an error occurred while retrieving event details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, convoy.ErrMessageNotFound) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			r = r.WithContext(setMessageInContext(r.Context(), msg))
			next.ServeHTTP(w, r)
		})
	}
}

func requireMessageDeliveryAttempt() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			id := chi.URLParam(r, "deliveryAttemptID")

			attempts := getDeliveryAttemptsFromContext(r.Context())

			attempt, err := findMessageDeliveryAttempt(attempts, id)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			r = r.WithContext(setDeliveryAttemptInContext(r.Context(), attempt))
			next.ServeHTTP(w, r)
		})
	}
}

func updateEndpointIfFound(endpoints *[]convoy.Endpoint, id string, e models.Endpoint) (*[]convoy.Endpoint, *convoy.Endpoint, error) {
	for i, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			endpoint.TargetURL = e.URL
			endpoint.Description = e.Description
			endpoint.Status = convoy.ActiveEndpointStatus
			endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
			(*endpoints)[i] = endpoint
			return endpoints, &endpoint, nil
		}
	}
	return endpoints, nil, convoy.ErrEndpointNotFound
}

func findEndpoint(endpoints *[]convoy.Endpoint, id string) (*convoy.Endpoint, error) {
	for _, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			return &endpoint, nil
		}
	}
	return nil, convoy.ErrEndpointNotFound
}

func requireDefaultGroup(groupRepo convoy.GroupRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			groups, err := groupRepo.LoadGroups(r.Context())
			if err != nil {

				msg := "an error occurred while loading default group"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, mongo.ErrNoDocuments) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			r = r.WithContext(setGroupInContext(r.Context(), groups[0]))
			next.ServeHTTP(w, r)
		})
	}
}

func pagination(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPerPage := r.URL.Query().Get("perPage")
		rawPage := r.URL.Query().Get("page")
		rawSort := r.URL.Query().Get("sort")

		if len(rawPerPage) == 0 {
			rawPerPage = "20"
		}
		if len(rawPage) == 0 {
			rawPage = "0"
		}
		if len(rawSort) == 0 {
			rawSort = "-1"
		}

		var err error
		var sort = -1 // desc by default
		order := strings.ToLower(rawSort)
		if order == "asc" {
			sort = 1
		}

		var perPage int
		if perPage, err = strconv.Atoi(rawPerPage); err != nil {
			perPage = 20
		}

		var page int
		if page, err = strconv.Atoi(rawPage); err != nil {
			page = 0
		}
		pageable := models.Pageable{
			Page:    page,
			PerPage: perPage,
			Sort:    sort,
		}
		r = r.WithContext(setPageableInContext(r.Context(), pageable))
		next.ServeHTTP(w, r)
	})
}

func fetchGroupApps(appRepo convoy.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			pageable := getPageableFromContext(r.Context())

			group := getGroupFromContext(r.Context())

			apps, paginationData, err := appRepo.LoadApplicationsPagedByGroupId(r.Context(), group.UID, pageable)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching apps", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setApplicationsInContext(r.Context(), &apps))
			r = r.WithContext(setPaginationDataInContext(r.Context(), &paginationData))
			next.ServeHTTP(w, r)
		})
	}
}

func fetchDashboardSummary(appRepo convoy.ApplicationRepository, msgRepo convoy.MessageRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			format := "2006-01-02T15:04:05"
			startDate := r.URL.Query().Get("startDate")
			endDate := r.URL.Query().Get("endDate")
			if len(startDate) == 0 {
				_ = render.Render(w, r, newErrorResponse("please specify a startDate query", http.StatusBadRequest))
				return
			}

			startT, err := time.Parse(format, startDate)
			if err != nil {
				log.Errorln("error parsing startDate - ", err)
				_ = render.Render(w, r, newErrorResponse("please specify a startDate in the format "+format, http.StatusBadRequest))
				return
			}

			period := r.URL.Query().Get("type")
			if util.IsStringEmpty(period) {
				_ = render.Render(w, r, newErrorResponse("please specify a type query", http.StatusBadRequest))
				return
			}

			if !convoy.IsValidPeriod(period) {
				_ = render.Render(w, r, newErrorResponse("please specify a type query in (daily, weekly, monthly, yearly)", http.StatusBadRequest))
				return
			}

			var endT time.Time
			if len(endDate) == 0 {
				endT = time.Date(startT.Year(), startT.Month(), startT.Day(), 23, 59, 59, 999999999, startT.Location())
			} else {
				endT, err = time.Parse(format, endDate)
				if err != nil {
					_ = render.Render(w, r, newErrorResponse("please specify an endDate in the format "+format+" or none at all", http.StatusBadRequest))
					return
				}
			}

			p := convoy.PeriodValues[period]
			if err := ensurePeriod(startT, endT); err != nil {
				_ = render.Render(w, r, newErrorResponse(fmt.Sprintf("invalid period '%s': %s", period, err.Error()), http.StatusBadRequest))
				return
			}

			searchParams := models.SearchParams{
				CreatedAtStart: startT.Unix(),
				CreatedAtEnd:   endT.Unix(),
			}

			group := getGroupFromContext(r.Context())

			apps, err := appRepo.SearchApplicationsByGroupId(r.Context(), group.UID, searchParams)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while searching apps", http.StatusInternalServerError))
				return
			}

			messagesSent, messages, err := computeDashboardMessages(r.Context(), group.UID, msgRepo, searchParams, p)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching messages", http.StatusInternalServerError))
				return
			}

			dashboard := models.DashboardSummary{
				Applications: len(apps),
				MessagesSent: messagesSent,
				Period:       period,
				PeriodData:   &messages,
			}

			r = r.WithContext(setDashboardSummaryInContext(r.Context(), &dashboard))
			next.ServeHTTP(w, r)
		})
	}
}

func ensurePeriod(start time.Time, end time.Time) error {
	if start.Unix() > end.Unix() {
		return errors.New("startDate cannot be greater than endDate")
	}

	return nil
}

func computeDashboardMessages(ctx context.Context, orgId string, msgRepo convoy.MessageRepository, searchParams models.SearchParams, period convoy.Period) (uint64, []models.MessageInterval, error) {

	var messagesSent uint64

	messages, err := msgRepo.LoadMessageIntervals(ctx, orgId, searchParams, period, 1)
	if err != nil {
		log.Errorln("failed to load message intervals - ", err)
		return 0, nil, err
	}

	for _, m := range messages {
		messagesSent += m.Count
	}

	return messagesSent, messages, nil
}

func requireAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			cfg, err := config.Get()
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error has occurred", http.StatusInternalServerError))
				return
			}

			if cfg.Auth.Type == config.NoAuthProvider {
				// full access
			} else if cfg.Auth.Type == config.BasicAuthProvider {
				err := ensureBasicAuthFromRequest(&cfg.Auth, r)
				if err != nil {
					_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusUnauthorized))
					return
				}
			} else {
				_ = render.Render(w, r, newErrorResponse("access denied", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setApplicationInContext(ctx context.Context,
	app *convoy.Application) context.Context {
	return context.WithValue(ctx, appCtx, app)
}

func getApplicationFromContext(ctx context.Context) *convoy.Application {
	return ctx.Value(appCtx).(*convoy.Application)
}

func setMessageInContext(ctx context.Context,
	msg *convoy.Message) context.Context {
	return context.WithValue(ctx, msgCtx, msg)
}

func getMessageFromContext(ctx context.Context) *convoy.Message {
	return ctx.Value(msgCtx).(*convoy.Message)
}

func setMessagesInContext(ctx context.Context,
	msg *[]convoy.Message) context.Context {
	return context.WithValue(ctx, msgCtx, msg)
}
func setApplicationsInContext(ctx context.Context,
	apps *[]convoy.Application) context.Context {
	return context.WithValue(ctx, appCtx, apps)
}

func getApplicationsFromContext(ctx context.Context) *[]convoy.Application {
	return ctx.Value(appCtx).(*[]convoy.Application)
}

func setApplicationEndpointInContext(ctx context.Context,
	endpoint *convoy.Endpoint) context.Context {
	return context.WithValue(ctx, endpointCtx, endpoint)
}

func getApplicationEndpointFromContext(ctx context.Context) *convoy.Endpoint {
	return ctx.Value(endpointCtx).(*convoy.Endpoint)
}

func setGroupInContext(ctx context.Context, group *convoy.Group) context.Context {
	return context.WithValue(ctx, groupCtx, group)
}

func getGroupFromContext(ctx context.Context) *convoy.Group {
	return ctx.Value(groupCtx).(*convoy.Group)
}

func setPageableInContext(ctx context.Context, pageable models.Pageable) context.Context {
	return context.WithValue(ctx, pageableCtx, pageable)
}

func getPageableFromContext(ctx context.Context) models.Pageable {
	return ctx.Value(pageableCtx).(models.Pageable)
}

func setPaginationDataInContext(ctx context.Context, p *pager.PaginationData) context.Context {
	return context.WithValue(ctx, pageDataCtx, p)
}

func getPaginationDataFromContext(ctx context.Context) *pager.PaginationData {
	return ctx.Value(pageDataCtx).(*pager.PaginationData)
}

func setDashboardSummaryInContext(ctx context.Context, d *models.DashboardSummary) context.Context {
	return context.WithValue(ctx, dashboardCtx, d)
}

func getDashboardSummaryFromContext(ctx context.Context) *models.DashboardSummary {
	return ctx.Value(dashboardCtx).(*models.DashboardSummary)
}

func setDeliveryAttemptInContext(ctx context.Context,
	attempt *convoy.MessageAttempt) context.Context {
	return context.WithValue(ctx, deliveryAttemptsCtx, attempt)
}

func getDeliveryAttemptFromContext(ctx context.Context) *convoy.MessageAttempt {
	return ctx.Value(deliveryAttemptsCtx).(*convoy.MessageAttempt)
}

func setDeliveryAttemptsInContext(ctx context.Context,
	attempts *[]convoy.MessageAttempt) context.Context {
	return context.WithValue(ctx, deliveryAttemptsCtx, attempts)
}

func getDeliveryAttemptsFromContext(ctx context.Context) *[]convoy.MessageAttempt {
	return ctx.Value(deliveryAttemptsCtx).(*[]convoy.MessageAttempt)
}

func setAuthLoginInContext(ctx context.Context, a *AuthorizedLogin) context.Context {
	return context.WithValue(ctx, authLoginCtx, a)
}

func getAuthLoginFromContext(ctx context.Context) *AuthorizedLogin {
	return ctx.Value(authLoginCtx).(*AuthorizedLogin)
}

func setConfigInContext(ctx context.Context, c *ViewableConfiguration) context.Context {
	return context.WithValue(ctx, configCtx, c)
}

func getConfigFromContext(ctx context.Context) *ViewableConfiguration {
	return ctx.Value(configCtx).(*ViewableConfiguration)
}
