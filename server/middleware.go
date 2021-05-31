package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/hookcamp/hookcamp"
)

type contextKey string

const (
	orgCtx contextKey = "org"
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

func retrieveRequestID(r *http.Request) string { return middleware.GetReqID(r.Context()) }

func jsonResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func requireNoAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// val, err := tokenFromRequest(r)
		// if err == nil || !val.IsZero() {
		// 	render.Render(w, r, models.ErrAccessDenied)
		// 	return
		// }

		next.ServeHTTP(w, r)
	})
}

func requireAuth() func(next http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
