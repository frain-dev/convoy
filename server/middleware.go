package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/server/models"
	"github.com/hookcamp/hookcamp/util"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/hookcamp/hookcamp"
)

type contextKey string

const (
	orgCtx              contextKey = "org"
	appCtx              contextKey = "app"
	endpointCtx         contextKey = "endpoint"
	msgCtx              contextKey = "message"
	authConfigCtx       contextKey = "authConfig"
	pageableCtx         contextKey = "pageable"
	pageDataCtx         contextKey = "pageData"
	dashboardCtx        contextKey = "dashboard"
	deliveryAttemptsCtx contextKey = "deliveryAttempts"
)

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
		w.Header().Set("Access-Control-Allow-Origin", "*")
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

func requireApp(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			appID := chi.URLParam(r, "appID")

			app, err := appRepo.FindApplicationByID(r.Context(), appID)
			if err != nil {

				msg := "an error occurred while retrieving app details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, hookcamp.ErrApplicationNotFound) {
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

func ensureNewApp(orgRepo hookcamp.OrganisationRepository, appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var newApp models.Application
			err := json.NewDecoder(r.Body).Decode(&newApp)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
				return
			}

			appName := newApp.AppName
			if util.IsStringEmpty(appName) {
				_ = render.Render(w, r, newErrorResponse("please provide your appName", http.StatusBadRequest))
				return
			}
			orgId := newApp.OrgID
			if util.IsStringEmpty(orgId) {
				_ = render.Render(w, r, newErrorResponse("please provide your orgId", http.StatusBadRequest))
				return
			}

			org, err := orgRepo.FetchOrganisationByID(r.Context(), orgId)
			if err != nil {
				msg := "an error occurred while fetching organisation"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, hookcamp.ErrOrganisationNotFound) {
					msg = err.Error()
					statusCode = http.StatusBadRequest
				}
				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			if util.IsStringEmpty(newApp.Secret) {
				newApp.Secret, err = util.GenerateSecret()
				if err != nil {
					_ = render.Render(w, r, newErrorResponse(fmt.Sprintf("could not generate secret...%v", err.Error()), http.StatusInternalServerError))
					return
				}
			}

			uid := uuid.New().String()
			app := &hookcamp.Application{
				UID:            uid,
				OrgID:          org.UID,
				Title:          appName,
				Secret:         newApp.Secret,
				CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				Endpoints:      []hookcamp.Endpoint{},
				DocumentStatus: hookcamp.ActiveDocumentStatus,
			}

			err = appRepo.CreateApplication(r.Context(), app)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while creating app", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setApplicationInContext(r.Context(), app))
			next.ServeHTTP(w, r)
		})
	}
}

func ensureAppUpdate(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var appUpdate models.Application
			err := json.NewDecoder(r.Body).Decode(&appUpdate)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
				return
			}

			appName := appUpdate.AppName
			if util.IsStringEmpty(appName) {
				_ = render.Render(w, r, newErrorResponse("please provide your appName", http.StatusBadRequest))
				return
			}

			app := getApplicationFromContext(r.Context())

			app.Title = appName
			if !util.IsStringEmpty(appUpdate.Secret) {
				app.Secret = appUpdate.Secret
			}

			err = appRepo.UpdateApplication(r.Context(), app)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while updating app", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setApplicationInContext(r.Context(), app))
			next.ServeHTTP(w, r)
		})
	}
}

func ensureAppDeletion(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			app := getApplicationFromContext(r.Context())

			err := appRepo.DeleteApplication(r.Context(), app)
			if err != nil {
				log.Errorln("failed to delete app - ", err)
				_ = render.Render(w, r, newErrorResponse("an error occurred while deleting app", http.StatusInternalServerError))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ensureAppEndpointDeletion(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			app := getApplicationFromContext(r.Context())
			e := getApplicationEndpointFromContext(r.Context())

			for i, endpoint := range app.Endpoints {
				if endpoint.UID == e.UID && endpoint.DeletedAt == 0 {
					app.Endpoints = append(app.Endpoints[:i], app.Endpoints[i+1:]...)
					break
				}
			}

			err := appRepo.UpdateApplication(r.Context(), app)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while deleting app endpoint", http.StatusInternalServerError))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func fetchAllApps(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			orgId := r.URL.Query().Get("orgId")

			apps, err := appRepo.LoadApplications(r.Context(), orgId)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching apps", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setApplicationsInContext(r.Context(), &apps))
			next.ServeHTTP(w, r)
		})
	}
}

func ensureNewAppEndpoint(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var e models.Endpoint
			e, err := parseEndpointFromBody(r.Body)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			appID := chi.URLParam(r, "appID")
			app, err := appRepo.FindApplicationByID(r.Context(), appID)
			if err != nil {

				msg := "an error occurred while retrieving app details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, hookcamp.ErrApplicationNotFound) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			endpoint := &hookcamp.Endpoint{
				UID:         uuid.New().String(),
				TargetURL:   e.URL,
				Description: e.Description,
				CreatedAt:   primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:   primitive.NewDateTimeFromTime(time.Now()),
			}

			app.Endpoints = append(app.Endpoints, *endpoint)

			err = appRepo.UpdateApplication(r.Context(), app)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while adding app endpoint", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setApplicationEndpointInContext(r.Context(), endpoint))
			next.ServeHTTP(w, r)
		})
	}
}

func fetchAppEndpoints() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			app := getApplicationFromContext(r.Context())

			app.Endpoints = filterDeletedEndpoints(app.Endpoints)

			r = r.WithContext(setApplicationEndpointsInContext(r.Context(), &app.Endpoints))
			next.ServeHTTP(w, r)
		})
	}
}

func filterDeletedEndpoints(endpoints []hookcamp.Endpoint) []hookcamp.Endpoint {
	activeEndpoints := make([]hookcamp.Endpoint, 0)
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

	if util.IsStringEmpty(e.URL) {
		return e, errors.New("please provide your url")
	}

	u, err := url.Parse(e.URL)
	if err != nil {
		return e, errors.New("please provide a valid url")
	}

	e.URL = u.String()

	return e, nil
}

func ensureAppEndpointUpdate(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var e models.Endpoint
			e, err := parseEndpointFromBody(r.Body)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			app := getApplicationFromContext(r.Context())
			endPointId := chi.URLParam(r, "endpointID")

			endpoints, endpoint, err := updateEndpointIfFound(&app.Endpoints, endPointId, e)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			app.Endpoints = *endpoints
			err = appRepo.UpdateApplication(r.Context(), app)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while updating app endpoints", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setApplicationEndpointInContext(r.Context(), endpoint))
			next.ServeHTTP(w, r)
		})
	}
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

func updateEndpointIfFound(endpoints *[]hookcamp.Endpoint, id string, e models.Endpoint) (*[]hookcamp.Endpoint, *hookcamp.Endpoint, error) {
	for i, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			endpoint.TargetURL = e.URL
			endpoint.Description = e.Description
			endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
			(*endpoints)[i] = endpoint
			return endpoints, &endpoint, nil
		}
	}
	return endpoints, nil, hookcamp.ErrEndpointNotFound
}

func findEndpoint(endpoints *[]hookcamp.Endpoint, id string) (*hookcamp.Endpoint, error) {
	for _, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			return &endpoint, nil
		}
	}
	return nil, hookcamp.ErrEndpointNotFound
}

func ensureNewOrganisation(orgRepo hookcamp.OrganisationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var newOrg models.Organisation
			err := json.NewDecoder(r.Body).Decode(&newOrg)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
				return
			}

			orgName := newOrg.Name
			if util.IsStringEmpty(orgName) {
				_ = render.Render(w, r, newErrorResponse("please provide a valid name", http.StatusBadRequest))
				return
			}
			org := &hookcamp.Organisation{
				UID:            uuid.New().String(),
				OrgName:        orgName,
				CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				DocumentStatus: hookcamp.ActiveDocumentStatus,
			}

			err = orgRepo.CreateOrganisation(r.Context(), org)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while creating organisation", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setOrganisationInContext(r.Context(), org))
			next.ServeHTTP(w, r)
		})
	}
}

func fetchAllOrganisations(orgRepo hookcamp.OrganisationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			orgs, err := orgRepo.LoadOrganisations(r.Context())
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching organisations", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setOrganisationsInContext(r.Context(), orgs))
			next.ServeHTTP(w, r)
		})
	}
}

func requireOrganisation(orgRepo hookcamp.OrganisationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			orgId := chi.URLParam(r, "orgID")

			org, err := orgRepo.FetchOrganisationByID(r.Context(), orgId)
			if err != nil {

				msg := "an error occurred while retrieving organisation details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, hookcamp.ErrOrganisationNotFound) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			r = r.WithContext(setOrganisationInContext(r.Context(), org))
			next.ServeHTTP(w, r)
		})
	}
}

func ensureOrganisationUpdate(orgRepo hookcamp.OrganisationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var update models.Organisation
			err := json.NewDecoder(r.Body).Decode(&update)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
				return
			}

			orgName := update.Name
			if util.IsStringEmpty(orgName) {
				_ = render.Render(w, r, newErrorResponse("please provide a valid name", http.StatusBadRequest))
				return
			}

			orgId := chi.URLParam(r, "orgID")

			org, err := orgRepo.FetchOrganisationByID(r.Context(), orgId)
			if err != nil {

				msg := "an error occurred while retrieving organisation details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, hookcamp.ErrOrganisationNotFound) {
					msg = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, newErrorResponse(msg, statusCode))
				return
			}

			org.OrgName = orgName
			err = orgRepo.UpdateOrganisation(r.Context(), org)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while updating organisation", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setOrganisationInContext(r.Context(), org))
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

func fetchOrganisationApps(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			pageable := getPageableFromContext(r.Context())

			org := getOrganisationFromContext(r.Context())

			apps, paginationData, err := appRepo.LoadApplicationsPagedByOrgId(r.Context(), org.UID, pageable)
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

func fetchDashboardSummary(appRepo hookcamp.ApplicationRepository, msgRepo hookcamp.MessageRepository) func(next http.Handler) http.Handler {
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

			if !hookcamp.IsValidPeriod(period) {
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

			p := hookcamp.PeriodValues[period]
			if err := ensurePeriod(startT, endT); err != nil {
				_ = render.Render(w, r, newErrorResponse(fmt.Sprintf("invalid period '%s': %s", period, err.Error()), http.StatusBadRequest))
				return
			}

			searchParams := models.SearchParams{
				CreatedAtStart: startT.Unix(),
				CreatedAtEnd:   endT.Unix(),
			}

			org := getOrganisationFromContext(r.Context())

			apps, err := appRepo.SearchApplicationsByOrgId(r.Context(), org.UID, searchParams)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred while searching apps", http.StatusInternalServerError))
				return
			}

			messagesSent, messages, err := computeDashboardMessages(r.Context(), org.UID, msgRepo, searchParams, p)
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

func computeDashboardMessages(ctx context.Context, orgId string, msgRepo hookcamp.MessageRepository, searchParams models.SearchParams, period hookcamp.Period) (uint64, []models.MessageInterval, error) {

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
	app *hookcamp.Application) context.Context {
	return context.WithValue(ctx, appCtx, app)
}

func getApplicationFromContext(ctx context.Context) *hookcamp.Application {
	return ctx.Value(appCtx).(*hookcamp.Application)
}

func setMessageInContext(ctx context.Context,
	msg *hookcamp.Message) context.Context {
	return context.WithValue(ctx, msgCtx, msg)
}

func getMessageFromContext(ctx context.Context) *hookcamp.Message {
	return ctx.Value(msgCtx).(*hookcamp.Message)
}

func setMessagesInContext(ctx context.Context,
	msg *[]hookcamp.Message) context.Context {
	return context.WithValue(ctx, msgCtx, msg)
}

func getMessagesFromContext(ctx context.Context) *[]hookcamp.Message {
	return ctx.Value(msgCtx).(*[]hookcamp.Message)
}

func setApplicationsInContext(ctx context.Context,
	apps *[]hookcamp.Application) context.Context {
	return context.WithValue(ctx, appCtx, apps)
}

func getApplicationsFromContext(ctx context.Context) *[]hookcamp.Application {
	return ctx.Value(appCtx).(*[]hookcamp.Application)
}

func setApplicationEndpointInContext(ctx context.Context,
	endpoint *hookcamp.Endpoint) context.Context {
	return context.WithValue(ctx, endpointCtx, endpoint)
}

func getApplicationEndpointFromContext(ctx context.Context) *hookcamp.Endpoint {
	return ctx.Value(endpointCtx).(*hookcamp.Endpoint)
}

func setApplicationEndpointsInContext(ctx context.Context,
	endpoints *[]hookcamp.Endpoint) context.Context {
	return context.WithValue(ctx, endpointCtx, endpoints)
}

func getApplicationEndpointsFromContext(ctx context.Context) *[]hookcamp.Endpoint {
	return ctx.Value(endpointCtx).(*[]hookcamp.Endpoint)
}

func setOrganisationInContext(ctx context.Context, organisation *hookcamp.Organisation) context.Context {
	return context.WithValue(ctx, orgCtx, organisation)
}

func getOrganisationFromContext(ctx context.Context) *hookcamp.Organisation {
	return ctx.Value(orgCtx).(*hookcamp.Organisation)
}

func setOrganisationsInContext(ctx context.Context, organisations []*hookcamp.Organisation) context.Context {
	return context.WithValue(ctx, orgCtx, organisations)
}

func getOrganisationsFromContext(ctx context.Context) []*hookcamp.Organisation {
	return ctx.Value(orgCtx).([]*hookcamp.Organisation)
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
	attempt *hookcamp.MessageAttempt) context.Context {
	return context.WithValue(ctx, deliveryAttemptsCtx, attempt)
}

func getDeliveryAttemptFromContext(ctx context.Context) *hookcamp.MessageAttempt {
	return ctx.Value(deliveryAttemptsCtx).(*hookcamp.MessageAttempt)
}

func setDeliveryAttemptsInContext(ctx context.Context,
	attempts *[]hookcamp.MessageAttempt) context.Context {
	return context.WithValue(ctx, deliveryAttemptsCtx, attempts)
}

func getDeliveryAttemptsFromContext(ctx context.Context) *[]hookcamp.MessageAttempt {
	return ctx.Value(deliveryAttemptsCtx).(*[]hookcamp.MessageAttempt)
}

func setAuthConfigInContext(ctx context.Context, a *config.AuthConfiguration) context.Context {
	return context.WithValue(ctx, authConfigCtx, a)
}

func getAuthConfigFromContext(ctx context.Context) *config.AuthConfiguration {
	return ctx.Value(authConfigCtx).(*config.AuthConfiguration)
}
