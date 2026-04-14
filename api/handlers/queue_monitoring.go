package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/util"
)

const (
	queueMonitoringCookieName = "convoy_queue_monitoring_session"
	// Cookie must include /queue/monitoring/session so revoke can receive and invalidate it server-side.
	queueMonitoringCookiePath = "/queue/monitoring"
	queueMonitoringCookieTTL  = 15 * time.Minute
	revokedKeyPrefix          = "convoy:queue_session:revoked:"
)

var (
	cookieSigningKey     []byte
	cookieSigningKeyOnce sync.Once
)

func getCookieSigningKey() []byte {
	cookieSigningKeyOnce.Do(func() {
		cookieSigningKey = make([]byte, 32)
		if _, err := rand.Read(cookieSigningKey); err != nil {
			panic(fmt.Sprintf("failed to generate cookie signing key: %v", err))
		}
	})
	return cookieSigningKey
}

func signCookieValue(expiry time.Time) string {
	payload := fmt.Sprintf("%d", expiry.Unix())
	mac := hmac.New(sha256.New, getCookieSigningKey())
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, sig)
}

func parseAndVerifyCookie(cookieValue string) (expiryUnix int64, ok bool) {
	dotIdx := -1
	for i, c := range cookieValue {
		if c == '.' {
			dotIdx = i
			break
		}
	}
	if dotIdx < 0 {
		return 0, false
	}
	expiryStr := cookieValue[:dotIdx]
	sig := cookieValue[dotIdx+1:]

	mac := hmac.New(sha256.New, getCookieSigningKey())
	mac.Write([]byte(expiryStr))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return 0, false
	}

	if _, err := fmt.Sscanf(expiryStr, "%d", &expiryUnix); err != nil {
		return 0, false
	}
	if time.Now().Unix() >= expiryUnix {
		return 0, false
	}
	return expiryUnix, true
}

// ValidateQueueSessionCookie checks HMAC signature, expiry, and Redis revocation list.
func ValidateQueueSessionCookie(c cache.Cache) func(string) bool {
	return func(cookieValue string) bool {
		if _, ok := parseAndVerifyCookie(cookieValue); !ok {
			return false
		}

		var revoked bool
		key := revokedKeyPrefix + cookieValue
		if err := c.Get(context.Background(), key, &revoked); err == nil && revoked {
			return false
		}

		return true
	}
}

func (h *Handler) requireInstanceAdmin(w http.ResponseWriter, r *http.Request) bool {
	user, err := h.retrieveUser(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		return false
	}

	isAdmin, err := h.A.OrgMemberRepo.HasInstanceAdminAccess(r.Context(), user.UID)
	if err != nil || !isAdmin {
		_ = render.Render(w, r, util.NewErrorResponse("instance admin access required", http.StatusForbidden))
		return false
	}
	return true
}

func (h *Handler) CreateQueueMonitoringSession(w http.ResponseWriter, r *http.Request) {
	if !h.requireInstanceAdmin(w, r) {
		return
	}

	expiry := time.Now().Add(queueMonitoringCookieTTL)
	value := signCookieValue(expiry)

	http.SetCookie(w, &http.Cookie{
		Name:     queueMonitoringCookieName,
		Value:    value,
		Path:     queueMonitoringCookiePath,
		Expires:  expiry,
		MaxAge:   int(queueMonitoringCookieTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	_ = render.Render(w, r, util.NewServerResponse("queue monitoring session created", nil, http.StatusOK))
}

func (h *Handler) RevokeQueueMonitoringSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(queueMonitoringCookieName); err == nil {
		if expiryUnix, ok := parseAndVerifyCookie(cookie.Value); ok {
			remaining := time.Until(time.Unix(expiryUnix, 0))
			if remaining > 0 {
				key := revokedKeyPrefix + cookie.Value
				_ = h.A.Cache.Set(r.Context(), key, true, remaining)
			}
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     queueMonitoringCookieName,
		Value:    "",
		Path:     queueMonitoringCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	_ = render.Render(w, r, util.NewServerResponse("queue monitoring session revoked", nil, http.StatusOK))
}
