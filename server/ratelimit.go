package server

import (
	"net/http"

	"github.com/frain-dev/convoy"
	"github.com/go-chi/httprate"
)

func rateLimitByGroup() func(next http.Handler) http.Handler {
	return httprate.Limit(convoy.RATE_LIMIT, convoy.RATE_LIMIT_DURATION, httprate.WithKeyFuncs(func (req *http.Request) (string, error) {
		return getGroupFromContext(req.Context()).UID, nil
	}))
}


