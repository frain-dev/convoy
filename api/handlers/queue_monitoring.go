package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/util"
)

const (
	queueMonitoringCookieName = "convoy_queue_monitoring_session"
	// Cookie must include /queue/monitoring/session so revoke can receive and invalidate it server-side.
	queueMonitoringCookiePath = "/queue/monitoring"
	queueMonitoringCookieTTL  = 15 * time.Minute
	revokedKeyPrefix          = "convoy:queue_session:revoked:"
	// Cluster-wide HMAC key for queue monitoring session cookies, stored in
	// Redis so every replica signs and validates with the same key.
	signingKeyRedisKey = "convoy:queue_monitoring:cookie_signing_key"
	signingKeyLen      = 32
)

func shouldSetSecureCookie(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}

	cfg, err := config.Get()
	if err != nil {
		return false
	}

	return !strings.EqualFold(cfg.Environment, "development")
}

// getOrCreateSigningKey returns the cluster-wide HMAC key for queue monitoring
// session cookies. It is a random 32-byte value stored in Redis so every
// replica signs and validates with the same key; a per-process key breaks
// multi-replica deployments because the pod that signs a cookie and the pod
// that later validates it differ, so HMAC verification fails and the embed
// proxy returns "Authentication required". SetNX makes the first writer win, so
// concurrent replicas converge on one key. Fail closed (ok=false) on a nil
// client, any Redis error, or a corrupt value rather than fall back to a
// per-pod key that other replicas cannot verify.
func getOrCreateSigningKey(ctx context.Context, rc redis.UniversalClient) ([]byte, bool) {
	if rc == nil {
		return nil, false
	}

	if v, err := rc.Get(ctx, signingKeyRedisKey).Result(); err == nil {
		return decodeSigningKey(v)
	} else if !errors.Is(err, redis.Nil) {
		return nil, false
	}

	newKey := make([]byte, signingKeyLen)
	if _, err := rand.Read(newKey); err != nil {
		return nil, false
	}
	created, err := rc.SetNX(ctx, signingKeyRedisKey, hex.EncodeToString(newKey), 0).Result()
	if err != nil {
		return nil, false
	}
	if created {
		return newKey, true
	}

	// Another replica created the key first; read the winner.
	v, err := rc.Get(ctx, signingKeyRedisKey).Result()
	if err != nil {
		return nil, false
	}
	return decodeSigningKey(v)
}

func decodeSigningKey(encoded string) ([]byte, bool) {
	key, err := hex.DecodeString(encoded)
	if err != nil || len(key) != signingKeyLen {
		return nil, false
	}
	return key, true
}

func signWithKey(key []byte, expiry time.Time) string {
	payload := fmt.Sprintf("%d", expiry.Unix())
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	return fmt.Sprintf("%s.%s", payload, hex.EncodeToString(mac.Sum(nil)))
}

func verifyWithKey(key []byte, cookieValue string) (expiryUnix int64, ok bool) {
	dotIdx := strings.IndexByte(cookieValue, '.')
	if dotIdx < 0 {
		return 0, false
	}
	expiryStr := cookieValue[:dotIdx]
	sig := cookieValue[dotIdx+1:]

	mac := hmac.New(sha256.New, key)
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

func signCookieValue(ctx context.Context, rc redis.UniversalClient, expiry time.Time) (string, bool) {
	key, ok := getOrCreateSigningKey(ctx, rc)
	if !ok {
		return "", false
	}
	return signWithKey(key, expiry), true
}

func parseAndVerifyCookie(ctx context.Context, rc redis.UniversalClient, cookieValue string) (int64, bool) {
	key, ok := getOrCreateSigningKey(ctx, rc)
	if !ok {
		return 0, false
	}
	return verifyWithKey(key, cookieValue)
}

// ValidateQueueSessionCookie checks HMAC signature, expiry, and Redis revocation list.
func ValidateQueueSessionCookie(rc redis.UniversalClient, c cache.Cache) func(context.Context, string) bool {
	return func(ctx context.Context, cookieValue string) bool {
		if _, ok := parseAndVerifyCookie(ctx, rc, cookieValue); !ok {
			return false
		}

		// Fail closed: if the revocation lookup errors we cannot prove the
		// cookie is still valid, so reject rather than risk accepting a revoked
		// session. A cache miss returns no error and leaves revoked=false.
		var revoked bool
		key := revokedKeyPrefix + cookieValue
		if err := c.Get(ctx, key, &revoked); err != nil {
			return false
		}

		return !revoked
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
	value, ok := signCookieValue(r.Context(), h.A.Redis, expiry)
	if !ok {
		_ = render.Render(w, r, util.NewErrorResponse("queue monitoring session unavailable", http.StatusInternalServerError))
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     queueMonitoringCookieName,
		Value:    value,
		Path:     queueMonitoringCookiePath,
		Expires:  expiry,
		MaxAge:   int(queueMonitoringCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   shouldSetSecureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})

	_ = render.Render(w, r, util.NewServerResponse("queue monitoring session created", nil, http.StatusOK))
}

func (h *Handler) RevokeQueueMonitoringSession(w http.ResponseWriter, r *http.Request) {
	// Only a caller presenting a server-signed cookie can create a revocation
	// entry, so a forged or random cookie value cannot flood the shared
	// revocation namespace. No instance-admin check here on purpose: a user
	// whose admin role was removed must still be able to revoke and clear a
	// session they previously minted, and the dashboard calls this on every
	// logout. Failure policy: if the cookie cannot be verified (invalid, or the
	// signing key is briefly unavailable) we skip the server-side entry but
	// still clear the browser cookie below; the cookie self-expires within the
	// mint TTL and the embed path fails closed while Redis is unavailable.
	if cookie, err := r.Cookie(queueMonitoringCookieName); err == nil && cookie.Value != "" {
		if expiryUnix, ok := parseAndVerifyCookie(r.Context(), h.A.Redis, cookie.Value); ok {
			if remaining := time.Until(time.Unix(expiryUnix, 0)); remaining > 0 {
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
		Secure:   shouldSetSecureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})

	_ = render.Render(w, r, util.NewServerResponse("queue monitoring session revoked", nil, http.StatusOK))
}
