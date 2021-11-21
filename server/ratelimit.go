package server

import (
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/go-chi/httprate"
)

func rateLimitByGroup() func(next http.Handler) http.Handler {
	return rateLimitByGroupWithParams(convoy.RATE_LIMIT, convoy.RATE_LIMIT_DURATION)
}

func rateLimitByGroupWithParams(requestLimit int, windowLength time.Duration) func(next http.Handler) http.Handler {
	return httprate.Limit(requestLimit, windowLength, httprate.WithKeyFuncs(func (req *http.Request) (string, error) {
		return getGroupFromContext(req.Context()).UID, nil
	}))
}


