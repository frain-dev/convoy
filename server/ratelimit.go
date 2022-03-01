package server

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	limiter "github.com/frain-dev/convoy/limiter"
	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
	log "github.com/sirupsen/logrus"
)

func rateLimitByGroup() func(next http.Handler) http.Handler {
	return rateLimitByGroupWithParams(convoy.RATE_LIMIT, convoy.RATE_LIMIT_DURATION)
}

func rateLimitByGroupWithParams(requestLimit int, windowLength time.Duration) func(next http.Handler) http.Handler {
	return httprate.Limit(requestLimit, windowLength, httprate.WithKeyFuncs(func(req *http.Request) (string, error) {
		return getGroupFromContext(req.Context()).UID, nil
	}))
}

func rateLimitByGroupID(limiter limiter.RateLimiter) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			group := getGroupFromContext(r.Context())

			res, err := limiter.Allow(r.Context(), group.UID, group.RateLimit+1)
			if err != nil {
				log.WithError(err).Error("an error occured")
				_ = render.Render(w, r, newErrorResponse("an error occured", http.StatusBadRequest))
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", int(math.Max(0, float64(res.Limit.Rate-1)))))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", int(math.Max(0, float64(res.Remaining-1)))))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%v", res.ResetAfter))

			// the Retry-After header should only be set when the rate limit has been reached
			if res.RetryAfter > time.Nanosecond {
				w.Header().Set("Retry-After", fmt.Sprintf("%v", res.RetryAfter))
			}

			if res.Remaining == 0 {
				_ = render.Render(w, r, newErrorResponse("Too Many Requests", http.StatusTooManyRequests))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
