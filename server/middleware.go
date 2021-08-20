package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/server/models"
	"github.com/hookcamp/hookcamp/util"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/hookcamp/hookcamp"
)

type contextKey string

const (
	appCtx      contextKey = "app"
	endpointCtx contextKey = "endpoint"
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

			uid := uuid.New().String()
			app := &hookcamp.Application{
				UID:       uid,
				OrgID:     org.UID,
				Title:     appName,
				CreatedAt: time.Now().Unix(),
				UpdatedAt: time.Now().Unix(),
				Endpoints: []hookcamp.Endpoint{},
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

			app.Title = appName
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

func fetchAllApps(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			apps, err := appRepo.LoadApplications(r.Context())
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
				Secret:      e.Secret,
				Description: e.Description,
				CreatedAt:   time.Now().Unix(),
				UpdatedAt:   time.Now().Unix(),
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

	if util.IsStringEmpty(e.Secret) {
		e.Secret, err = util.GenerateRandomString(25)
		if err != nil {
			return e, fmt.Errorf("could not generate secret...%v", err)
		}
	}

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

func updateEndpointIfFound(endpoints *[]hookcamp.Endpoint, id string, e models.Endpoint) (*[]hookcamp.Endpoint, *hookcamp.Endpoint, error) {
	for i, endpoint := range *endpoints {
		if endpoint.UID == id {
			endpoint.TargetURL = e.URL
			endpoint.Description = e.Description
			endpoint.UpdatedAt = time.Now().Unix()
			(*endpoints)[i] = endpoint
			return endpoints, &endpoint, nil
		}
	}
	return endpoints, nil, hookcamp.ErrEndpointNotFound
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
