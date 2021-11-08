package server

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/frain-dev/convoy/config"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm_chain"

	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

// http middleware
func requirePermission() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds, err := getAuthFromRequest(r)
			if err != nil {
				log.WithError(err).Error("failed to get auth from request")
				_ = render.Render(w, r, newErrorResponse(err.Error(), http.StatusUnauthorized))
				return
			}

			rc := realm_chain.Get()
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

func requireRole(role auth.RoleType) func(next http.Handler) http.Handler {
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
