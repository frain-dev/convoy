package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/hookcamp/hookcamp"
)

type contextKey string

const (
	orgCtx contextKey = "org"
	appCtx contextKey = "app"
)

func writeRequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", r.Context().Value(middleware.RequestIDKey).(string))
		next.ServeHTTP(w, r)
	})
}

func tokenFromRequest(r *http.Request) (hookcamp.Token, error) {
	val := r.Header.Get("Authorization")
	splitted := strings.Split(val, " ")

	var t hookcamp.Token

	if len(splitted) != 2 {
		return t, errors.New("invalid header structure")
	}

	if strings.ToUpper(splitted[0]) != "BEARER" {
		return t, errors.New("invalid header structure")
	}

	return hookcamp.Token(splitted[1]), nil
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

func requireAppOwnership(appRepo hookcamp.ApplicationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			org := getOrgFromContext(r.Context())

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

			if !org.IsOwner(app) {
				_ = render.Render(w, r, newErrorResponse("cannot access resource", http.StatusUnauthorized))
				return
			}

			r = r.WithContext(setApplicationInContext(r.Context(), app))
			next.ServeHTTP(w, r)
		})
	}
}

func requireAuth(orgRepo hookcamp.OrganisationRepository) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			token, err := tokenFromRequest(r)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("please provide your API key", http.StatusUnauthorized))
				return
			}

			org, err := orgRepo.FetchOrganisationByAPIKey(r.Context(), token)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error occurred", http.StatusNotFound))
				return
			}

			if org.IsDeleted() {
				_ = render.Render(w, r, newErrorResponse("cannot access deleted organisation", http.StatusForbidden))
				return
			}

			r = r.WithContext(setOrgInContext(r.Context(), org))
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

func setOrgInContext(ctx context.Context,
	org *hookcamp.Organisation) context.Context {
	return context.WithValue(ctx, orgCtx, org)
}

func getOrgFromContext(ctx context.Context) *hookcamp.Organisation {
	return ctx.Value(orgCtx).(*hookcamp.Organisation)
}
