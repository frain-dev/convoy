package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/limiter"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/util"
)

const (
	workspaceSlugProbeLimit    = 30
	workspaceSlugProbeDuration = 60
)

// WorkspaceSlugProbeRateLimit throttles unauthenticated workspace slug lookups on guest routes.
// Failure policy: fail closed when over limit (429). Applies only when slug query param is set.
func WorkspaceSlugProbeRateLimit(rateLimiter limiter.RateLimiter) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := strings.TrimSpace(r.URL.Query().Get("slug"))
			if slug == "" {
				next.ServeHTTP(w, r)
				return
			}

			clientIP := clientIPFromRequest(r)
			key := fmt.Sprintf("workspace-slug-probe:%s", clientIP)
			err := rateLimiter.AllowWithDuration(r.Context(), key, workspaceSlugProbeLimit, workspaceSlugProbeDuration)
			if err == nil {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", workspaceSlugProbeLimit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", time.Now().Add(rlimiter.GetRetryAfter(err)).Unix()))
			_ = render.Render(w, r, util.NewErrorResponse("exceeded rate limit", http.StatusTooManyRequests))
		})
	}
}

func clientIPFromRequest(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
