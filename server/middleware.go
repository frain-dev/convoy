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

	"github.com/frain-dev/convoy/auth/realm_chain"

	"github.com/frain-dev/convoy/auth"

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
	groupCtx         contextKey = "group"
	appCtx           contextKey = "app"
	endpointCtx      contextKey = "endpoint"
	eventCtx         contextKey = "event"
	eventDeliveryCtx contextKey = "eventDelivery"
	configCtx        contextKey = "configCtx"
	//authConfigCtx       contextKey = "authConfig"
	authLoginCtx        contextKey = "authLogin"
	authUserCtx         contextKey = "authUser"
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

				event := "an error occurred while retrieving app details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, convoy.ErrApplicationNotFound) {
					event = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(event, statusCode))
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

func requireEvent(eventRepo convoy.EventRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			eventId := chi.URLParam(r, "eventID")

			event, err := eventRepo.FindEventByID(r.Context(), eventId)
			if err != nil {

				event := "an error occurred while retrieving event details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, convoy.ErrEventNotFound) {
					event = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(event, statusCode))
				return
			}

			r = r.WithContext(setEventInContext(r.Context(), event))
			next.ServeHTTP(w, r)
		})
	}
}

func requireEventDelivery(eventRepo convoy.EventDeliveryRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			eventDeliveryID := chi.URLParam(r, "eventDeliveryID")

			eventDelivery, err := eventRepo.FindEventDeliveryByID(r.Context(), eventDeliveryID)
			if err != nil {

				eventDelivery := "an error occurred while retrieving event delivery details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, convoy.ErrEventDeliveryNotFound) {
					eventDelivery = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(eventDelivery, statusCode))
				return
			}

			r = r.WithContext(setEventDeliveryInContext(r.Context(), eventDelivery))
			next.ServeHTTP(w, r)
		})
	}
}

func requireDeliveryAttempt() func(next http.Handler) http.Handler {
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

func getDefaultGroup(r *http.Request, groupRepo convoy.GroupRepository) (*convoy.Group, error) {
	// TODO(daniel,subomi): i think this isn't safe
	name := r.URL.Query().Get("name")

	groups, err := groupRepo.LoadGroups(r.Context(), &convoy.GroupFilter{Name: name})
	if err != nil {
		return nil, err
	}

	return groups[0], err
}

func requireGroup(groupRepo convoy.GroupRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var group *convoy.Group
			var err error

			groupID := r.URL.Query().Get("groupID")
			if groupID != "" {
				group, err = groupRepo.FetchGroupByID(r.Context(), groupID)
				if err != nil {
					_ = render.Render(w, r, newErrorResponse("failed to fetch group by id", http.StatusInternalServerError))
					return
				}
			} else {
				group, err = getDefaultGroup(r, groupRepo)
				if err != nil {
					event := "an error occurred while loading default group"
					statusCode := http.StatusInternalServerError

					if errors.Is(err, mongo.ErrNoDocuments) {
						event = err.Error()
						statusCode = http.StatusNotFound
					}

					_ = render.Render(w, r, newErrorResponse(event, statusCode))
					return
				}
			}

			authUser := getAuthUserFromContext(r.Context())
			if authUser.Role.Type.Is(auth.RoleSuperUser) {
				// superuser has access to everything
				return
			}
			ctx := setGroupInContext(r.Context(), group)

			r = r.WithContext(ctx)
			for _, v := range authUser.Role.Groups {
				if group.Name == v {
					next.ServeHTTP(w, r)
					return
				}
			}

			_ = render.Render(w, r, newErrorResponse("unauthorized to access group", http.StatusUnauthorized))
		})
	}
}

func requireAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds, err := getAuthFromRequest(r)
			if err != nil {
				log.WithError(err).Error("failed to get auth from request")
				_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusUnauthorized))
				return
			}

			rc, err := realm_chain.Get()
			if err != nil {
				log.WithError(err).Error("failed to get realm chain")
				_ = render.Render(w, r, newErrorResponse("internal server error", http.StatusInternalServerError))
				return
			}

			authUser, err := rc.Authenticate(creds)
			if err != nil {
				log.WithError(err).Error("failed to authenticate")
				_ = render.Render(w, r, newErrorResponse("authorization failed", http.StatusUnauthorized))
				return
			}

			r = r.WithContext(setAuthUserInContext(r.Context(), authUser))
			next.ServeHTTP(w, r)
		})
	}
}

func requirePermission(role auth.RoleType) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := getAuthUserFromContext(r.Context())
			if authUser.Role.Type.Is(auth.RoleSuperUser) {
				// superuser has access to everything
				return
			}

			if !authUser.Role.Type.Is(role) {
				_ = render.Render(w, r, newErrorResponse("unauthorized role", http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func getAuthFromRequest(r *http.Request) (*auth.Credential, error) {
	cfg, err := config.Get()
	if err != nil {
		log.WithError(err)
		return nil, err
	}

	if !cfg.Auth.RequireAuth {
		return nil, nil
	}

	val := r.Header.Get("Authorization")
	authInfo := strings.Split(val, " ")

	if len(authInfo) != 2 {
		return nil, errors.New("invalid header structure")
	}

	credType := auth.CredentialType(strings.ToUpper(authInfo[0]))
	switch credType {
	case auth.CredentialTypeBasic:

		credentials, err := base64.StdEncoding.DecodeString(authInfo[1])
		if err != nil {
			return nil, errors.New("invalid credentials")
		}

		creds := strings.Split(string(credentials), ":")

		if len(creds) != 2 {
			return nil, errors.New("invalid basic credentials")
		}

		return &auth.Credential{
			Type:     auth.CredentialTypeBasic,
			Username: creds[0],
			Password: creds[1],
		}, nil

	default:
		return nil, fmt.Errorf("unknown credential type: %s", credType.String())
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

func fetchDashboardSummary(appRepo convoy.ApplicationRepository, eventRepo convoy.EventRepository) func(next http.Handler) http.Handler {
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

			eventsSent, messages, err := computeDashboardMessages(r.Context(), group.UID, eventRepo, searchParams, p)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching messages", http.StatusInternalServerError))
				return
			}

			dashboard := models.DashboardSummary{
				Applications: len(apps),
				EventsSent:   eventsSent,
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

func computeDashboardMessages(ctx context.Context, orgId string, eventRepo convoy.EventRepository, searchParams models.SearchParams, period convoy.Period) (uint64, []models.EventInterval, error) {

	var messagesSent uint64

	messages, err := eventRepo.LoadEventIntervals(ctx, orgId, searchParams, period, 1)
	if err != nil {
		log.Errorln("failed to load message intervals - ", err)
		return 0, nil, err
	}

	for _, m := range messages {
		messagesSent += m.Count
	}

	return messagesSent, messages, nil
}

func setApplicationInContext(ctx context.Context,
	app *convoy.Application) context.Context {
	return context.WithValue(ctx, appCtx, app)
}

func getApplicationFromContext(ctx context.Context) *convoy.Application {
	return ctx.Value(appCtx).(*convoy.Application)
}

func setEventInContext(ctx context.Context,
	event *convoy.Event) context.Context {
	return context.WithValue(ctx, eventCtx, event)
}

func getEventFromContext(ctx context.Context) *convoy.Event {
	return ctx.Value(eventCtx).(*convoy.Event)
}

func setEventDeliveryInContext(ctx context.Context,
	eventDelivery *convoy.EventDelivery) context.Context {
	return context.WithValue(ctx, eventDeliveryCtx, eventDelivery)
}

func getEventDeliveryFromContext(ctx context.Context) *convoy.EventDelivery {
	return ctx.Value(eventDeliveryCtx).(*convoy.EventDelivery)
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
	attempt *convoy.DeliveryAttempt) context.Context {
	return context.WithValue(ctx, deliveryAttemptsCtx, attempt)
}

func getDeliveryAttemptFromContext(ctx context.Context) *convoy.DeliveryAttempt {
	return ctx.Value(deliveryAttemptsCtx).(*convoy.DeliveryAttempt)
}

func setDeliveryAttemptsInContext(ctx context.Context,
	attempts *[]convoy.DeliveryAttempt) context.Context {
	return context.WithValue(ctx, deliveryAttemptsCtx, attempts)
}

func getDeliveryAttemptsFromContext(ctx context.Context) *[]convoy.DeliveryAttempt {
	return ctx.Value(deliveryAttemptsCtx).(*[]convoy.DeliveryAttempt)
}

func setAuthUserInContext(ctx context.Context, a *auth.AuthenticatedUser) context.Context {
	return context.WithValue(ctx, authUserCtx, a)
}

func getAuthUserFromContext(ctx context.Context) *auth.AuthenticatedUser {
	return ctx.Value(authUserCtx).(*auth.AuthenticatedUser)
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
