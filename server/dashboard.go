package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/frain-dev/convoy/config"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

type Claims struct {
	Username string `json:"username"`
	jwt.StandardClaims
}

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

type ViewableConfiguration struct {
	Strategy  config.StrategyConfiguration  `json:"strategy"`
	Signature config.SignatureConfiguration `json:"signature"`
}

func (a *applicationHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Dashboard summary fetched successfully",
		*getDashboardSummaryFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAuthLogin(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Logged in successfully",
		getAuthLoginFromContext(r.Context()), http.StatusOK))
}

func (a *applicationHandler) GetAllConfigDetails(w http.ResponseWriter, r *http.Request) {

	_ = render.Render(w, r, newServerResponse("Config details fetched successfully",
		getConfigFromContext(r.Context()), http.StatusOK))
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

func refresh() func(next http.Handler) http.Handler {
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
				log.Errorln("error while fetching auth config - ", err)
				_ = render.Render(w, r, newErrorResponse("an error occurred while refreshing your token", http.StatusInternalServerError))
				return
			}

			token := auth[1]

			claims := &Claims{}
			_, err = jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.UIAuth.JwtKey), nil
			})
			if err != nil {
				if err == jwt.ErrSignatureInvalid {
					log.Errorln("Error validating token - ", err)
					_ = render.Render(w, r, newErrorResponse("unauthorized", http.StatusUnauthorized))
					return
				}

				if !strings.Contains(err.Error(), "expired") {
					log.Errorln("Unknown error validating token - ", err)
					_ = render.Render(w, r, newErrorResponse("invalid access token", http.StatusBadRequest))
					return
				}
			}

			if time.Since(time.Unix(claims.ExpiresAt, 0)) > 30*time.Second {
				_ = render.Render(w, r, newErrorResponse("access token is not within the refresh window of 30 seconds", http.StatusBadRequest))
				return
			}

			expiryTime := time.Now().Add(cfg.UIAuth.JwtTokenExpirySeconds * time.Second)
			claims.ExpiresAt = expiryTime.Unix()
			newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
			signedString, err := newToken.SignedString([]byte(cfg.UIAuth.JwtKey))
			if err != nil {
				log.Errorln("error while generating jwt token - ", err)
				_ = render.Render(w, r, newErrorResponse("An error has occurred", http.StatusInternalServerError))
				return
			}

			r = r.WithContext(setAuthLoginInContext(r.Context(), &AuthorizedLogin{
				Username:   claims.Username,
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

			cfg, err := config.Get()
			if err != nil {
				_ = render.Render(w, r, newErrorResponse("an error has occurred", http.StatusInternalServerError))
				return
			}

			if cfg.UIAuth.Type == config.NoAuthProvider {
				// full access
			} else if cfg.UIAuth.Type == config.BasicAuthProvider {

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
			} else {
				_ = render.Render(w, r, newErrorResponse("access denied", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func fetchAllConfigDetails() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			g := getGroupFromContext(r.Context())

			viewableConfig := ViewableConfiguration{
				Strategy:  g.Config.Strategy,
				Signature: g.Config.Signature,
			}

			r = r.WithContext(setConfigInContext(r.Context(), &viewableConfig))
			next.ServeHTTP(w, r)
		})
	}
}
