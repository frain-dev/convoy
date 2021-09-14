package server

import (
	"encoding/json"
	"github.com/dgrijalva/jwt-go"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

func fetchAuthConfig() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			cfg, err := config.Get()
			if err != nil {
				log.Errorln("error while fetching auth config - ", err)
				_ = render.Render(w, r, newErrorResponse("an error occurred while fetching authorization details", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setAuthConfigInContext(r.Context(), &cfg.Auth))
			next.ServeHTTP(w, r)
		})
	}
}

func login() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var b config.Basic

			err := json.NewDecoder(r.Body).Decode(&b)
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("Request is invalid", http.StatusBadRequest))
				return
			}

			cfg, err := config.Get()
			if err != nil {
				log.Errorln("error while fetching auth config - ", err)
				_ = render.Render(w, r, newErrorResponse("an error occurred while logging you in", http.StatusInternalServerError))
				return
			}

			authorizedUsers := cfg.UIAuthorizedUsers
			expectedPassword, ok := authorizedUsers[b.Username]
			if !ok || expectedPassword != b.Password {
				_ = render.Render(w, r, newErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}

			expiryTime := time.Now().Add(cfg.UIAuth.JwtTokenExpirySeconds * time.Second)
			claims := &Claims{
				Username: b.Username,
				StandardClaims: jwt.StandardClaims{
					ExpiresAt: expiryTime.Unix(),
				},
			}

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signedString, err := token.SignedString([]byte(cfg.UIAuth.JwtKey))
			if err != nil {
				log.Errorln("error while generating jwt token - ", err)
				_ = render.Render(w, r, newErrorResponse("An error has occurred", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setAuthLoginInContext(r.Context(), &AuthorizedLogin{
				Username:   b.Username,
				Token:      signedString,
				ExpiryTime: expiryTime,
			}))
			next.ServeHTTP(w, r)
		})
	}
}

func requireUIAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			val := r.Header.Get("Authorization")
			auth := strings.Split(val, " ")

			if len(auth) != 2 {
				_ = render.Render(w, r, newErrorResponse("invalid bearer header structure", http.StatusBadRequest))
				return
			}

			if strings.ToUpper(auth[0]) != "BEARER" {
				_ = render.Render(w, r, newErrorResponse("invalid bearer header structure", http.StatusBadRequest))
				return
			}

			cfg, err := config.Get()
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error has occurred", http.StatusInternalServerError))
				return
			}

			token := auth[1]

			claims := &Claims{}

			t, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.UIAuth.JwtKey), nil
			})
			if err != nil {
				if err == jwt.ErrSignatureInvalid {
					log.Errorln("Error validating token - ", err)
					_ = render.Render(w, r, newErrorResponse("unauthorized", http.StatusUnauthorized))
					return
				}

				if strings.Contains(err.Error(), "expired") {
					_ = render.Render(w, r, newErrorResponse("access token has expired", http.StatusUnauthorized))
					return
				}

				log.Errorln("Unknown error validating token - ", err)
				_ = render.Render(w, r, newErrorResponse("invalid request", http.StatusUnauthorized))
				return
			}
			if !t.Valid {
				_ = render.Render(w, r, newErrorResponse("invalid access token", http.StatusUnauthorized))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
