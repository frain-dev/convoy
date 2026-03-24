package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/riandyrn/otelchi"

	sdktrace "go.opentelemetry.io/otel/trace"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrValidLicenseRequired = errors.New("access to this resource requires a valid license")

	// skipLoggingPaths defines paths that should not be logged by the request logger
	skipLoggingPaths = []string{
		"/billing/organisations/",
	}

	// sensitiveHeaders is the set of lowercased header names whose values must
	// never appear in logs, even transiently.
	sensitiveHeaders = map[string]struct{}{
		"authorization":                {},
		"proxy-authorization":          {},
		"cookie":                       {},
		"set-cookie":                   {},
		"x-convoy-signature":           {}, // outbound HMAC set by dispatcher
		"x-hub-signature":              {}, // GitHub legacy
		"x-hub-signature-256":          {}, // GitHub SHA-256
		"x-shopify-hmac-sha256":        {}, // Shopify
		"x-twitter-webhooks-signature": {}, // Twitter
	}
)

// shouldSkipLogging checks if the given path should be excluded from logging
func shouldSkipLogging(r, w map[string]interface{}) bool {
	// Check if this is a path we want to skip
	for _, skipPath := range skipLoggingPaths {
		if strings.Contains(r["requestURL"].(string), skipPath) {
			return true
		}
	}

	headers := w["header"].(map[string]string)

	if strings.Contains(headers["content-type"], "application/javascript") {
		return true
	}

	if strings.Contains(headers["content-type"], "image") {
		return true
	}

	if strings.Contains(headers["content-type"], "font") {
		return true
	}

	if strings.Contains(headers["content-type"], "text/html") {
		return true
	}

	if strings.Contains(headers["content-type"], "text/javascript") {
		return true
	}

	if strings.Contains(headers["content-type"], "text/css") {
		return true
	}

	return false
}

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

func RateLimiterHandler(rateLimiter limiter.RateLimiter, httpApiRateLimit int) func(next http.Handler) http.Handler {
	duration := 60
	rateLimit := httpApiRateLimit * duration
	rateLimitKey := "http-api"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := rateLimiter.AllowWithDuration(r.Context(), rateLimitKey, rateLimit, duration)
			if err == nil {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rateLimit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", 0))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%f", rlimiter.GetRetryAfter(err).Seconds()))
			w.Header().Set("Retry-After", fmt.Sprintf("%d", time.Now().Add(rlimiter.GetRetryAfter(err)).Unix()))

			_ = render.Render(w, r, util.NewErrorResponse("exceeded rate limit", http.StatusTooManyRequests))
		})
	}
}

func InstrumentPath(l license.Licenser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m := httpsnoop.CaptureMetrics(next, w, r)
			mm := metrics.GetDPInstance(l)

			val := chi.URLParam(r, "projectID")
			mm.RecordIngestLatency(val, m.Duration.Seconds())
			mm.IncrementIngestTotal("http", val)
		})
	}
}

func InstrumentRequests(serverName string, r chi.Router) func(next http.Handler) http.Handler {
	return otelchi.Middleware(serverName, otelchi.WithChiRoutes(r))
}

func WriteRequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", r.Context().Value(middleware.RequestIDKey).(string))
		next.ServeHTTP(w, r)
	})
}

func WriteVersionHeader(header, version string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(header, version)
			next.ServeHTTP(w, r)
		})
	}
}

func CanAccessFeature(fflag *fflag.FFlag, featureKey fflag.FeatureFlagKey) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !fflag.CanAccessFeature(featureKey) {
				_ = render.Render(w, r, util.NewErrorResponse("this feature is not enabled in this server", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func SetupCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.Get()
		if err != nil {
			slog.ErrorContext(r.Context(), "failed to load configuration", "error", err)
			return
		}

		if env := cfg.Environment; string(env) == "development" {
			w.Header().Set("Access-Control-Allow-Origin", cfg.Host)
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		}

		if r.Method == "OPTIONS" {
			return
		}

		next.ServeHTTP(w, r)
	})
}

func JsonResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func RequireAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds, err := GetAuthFromRequest(r)
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to get auth from request", "error", err)
				_ = render.Render(w, r, util.NewErrorResponse("Authentication required", http.StatusUnauthorized))
				return
			}

			rc, err := realm_chain.Get()
			if err != nil {
				slog.ErrorContext(r.Context(), "failed to get realm chain", "error", err)
				_ = render.Render(w, r, util.NewErrorResponse("internal server error", http.StatusInternalServerError))
				return
			}

			authUser, err := rc.Authenticate(r.Context(), creds)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("authorization failed", http.StatusUnauthorized))
				return
			}

			authCtx := context.WithValue(r.Context(), convoy.AuthUserCtx, authUser)

			r = r.WithContext(setAuthUserInContext(authCtx, authUser))
			next.ServeHTTP(w, r)
		})
	}
}

func RequirePersonalAccessToken() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := GetAuthUserFromContext(r.Context())
			_, ok := authUser.User.(*datastore.User)

			if authUser.AuthenticatedByRealm == auth.NativeRealmName && ok {
				next.ServeHTTP(w, r)
				return
			}

			_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusBadRequest))
		})
	}
}

func GetAuthFromRequest(r *http.Request) (*auth.Credential, error) {
	val := r.Header.Get("Authorization")

	// authInfo is the token and the type of token based on the header (Bearer or Basic)
	authInfo := strings.Split(val, " ")

	if len(authInfo) != 2 {
		err := errors.New("invalid header structure")
		return nil, err
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
	case auth.CredentialTypeAPIKey:
		authToken := authInfo[1]
		if util.IsStringEmpty(authToken) {
			return nil, errors.New("empty api key")
		}

		// the key is an API key or PAT
		apiKeyPrefix := fmt.Sprintf("%s%s", util.APIKeyPrefix, util.Separator)
		if strings.HasPrefix(authToken, apiKeyPrefix) {
			return &auth.Credential{
				Type:   auth.CredentialTypeAPIKey,
				APIKey: authToken,
			}, nil
		}

		portalTokenPrefix := fmt.Sprintf("%s%s", util.PortalAuthTokenPrefix, util.Separator)
		if strings.HasPrefix(authToken, portalTokenPrefix) {
			return &auth.Credential{
				Type:   auth.CredentialTypeToken,
				APIKey: authToken,
			}, nil
		}

		// the key is a jwt
		parts := strings.Split(authToken, ".")
		if len(parts) == 3 {
			return &auth.Credential{
				Type:  auth.CredentialTypeJWT,
				Token: authToken,
			}, nil
		}

		return &auth.Credential{
			Type:  auth.CredentialTypeToken,
			Token: authToken,
		}, nil
	default:
		return nil, fmt.Errorf("unknown credential type: %s", credType.String())
	}
}

func Pagination(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPerPage := r.URL.Query().Get("perPage")
		sort := r.URL.Query().Get("sort")
		rawDirection := r.URL.Query().Get("direction")
		rawNextCursor := r.URL.Query().Get("next_page_cursor")
		rawPrevCursor := r.URL.Query().Get("prev_page_cursor")

		if len(rawPerPage) == 0 {
			rawPerPage = "20"
		}

		if len(rawDirection) == 0 {
			rawDirection = "next"
		}

		perPage, err := strconv.Atoi(rawPerPage)
		if err != nil {
			perPage = 20
		}

		pageable := datastore.Pageable{
			Sort:       strings.ToUpper(sort),
			PerPage:    perPage,
			Direction:  datastore.PageDirection(rawDirection),
			NextCursor: rawNextCursor,
			PrevCursor: rawPrevCursor,
		}
		pageable.SetCursors()

		r = r.WithContext(setPageableInContext(r.Context(), pageable))
		next.ServeHTTP(w, r)
	})
}

func LogHttpRequest(a *types.APIOptions) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			wbuf := &bytes.Buffer{}
			ww.Tee(wbuf)

			defer func() {
				lvl := statusLevel(ww.Status())

				requestFields := requestLogFields(r)
				responseFields := responseLogFields(ww, wbuf, start)

				if shouldSkipLogging(requestFields, responseFields) {
					return
				}

				logArgs := []any{"httpRequest", requestFields, "httpResponse", responseFields}
				if orgID := extractOrganisationID(r); orgID != "" {
					logArgs = append(logArgs, "organisation_id", orgID)
				}

				// Use a constant log message; request details (including URL) are carried only in structured fields.
				a.Logger.Log(r.Context(), lvl, "http_request", logArgs...)
			}()

			requestID := middleware.GetReqID(r.Context())
			ctx := context.WithValue(r.Context(), convoy.RequestIDKey, requestID)
			r = r.WithContext(ctx)

			next.ServeHTTP(ww, r)
		})
	}
}

func requestLogFields(r *http.Request) map[string]interface{} {
	scheme := "http"

	if r.TLS != nil {
		scheme = "https"
	}

	requestURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	requestFields := map[string]interface{}{
		"requestURL":    requestURL,
		"requestMethod": r.Method,
		"requestPath":   r.URL.Path,
		"remoteIP":      r.RemoteAddr,
		"proto":         r.Proto,
		"scheme":        scheme,
	}

	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		requestFields["x-request-id"] = reqID
	}

	if len(r.Header) > 0 {
		requestFields["header"] = headerFields(r.Header)
	}

	span := sdktrace.SpanFromContext(r.Context())

	requestFields["traceId"] = span.SpanContext().SpanID()
	requestFields["spanId"] = span.SpanContext().TraceID()

	return requestFields
}

func responseLogFields(w middleware.WrapResponseWriter, wbuf *bytes.Buffer, t time.Time) map[string]interface{} {
	responseFields := map[string]interface{}{
		"status":  w.Status(),
		"byes":    w.BytesWritten(),
		"latency": time.Since(t),
		"body":    wbuf.String(),
	}

	if len(w.Header()) > 0 {
		responseFields["header"] = headerFields(w.Header())
	}

	return responseFields
}

func statusLevel(status int) slog.Level {
	switch {
	case status <= 0:
		return slog.LevelWarn
	case status < 400:
		return slog.LevelInfo
	case status < 500:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}

func headerFields(header http.Header) map[string]string {
	headerField := map[string]string{}

	for k, v := range header {
		k = strings.ToLower(k)
		if len(v) == 0 {
			continue
		}
		// Redact before any raw value is stored — severs CodeQL taint flow.
		if _, sensitive := sensitiveHeaders[k]; sensitive {
			headerField[k] = "***"
			continue
		}
		if len(v) == 1 {
			headerField[k] = v[0]
		} else {
			headerField[k] = fmt.Sprintf("[%s]", strings.Join(v, "], ["))
		}
	}

	return headerField
}

func EnsurePeriod(start, end time.Time) error {
	if start.Unix() > end.Unix() {
		return errors.New("startDate cannot be greater than endDate")
	}

	return nil
}

func setPageableInContext(ctx context.Context, pageable datastore.Pageable) context.Context {
	return context.WithValue(ctx, convoy.PageableCtx, pageable)
}

func GetPageableFromContext(ctx context.Context) datastore.Pageable {
	v := ctx.Value(convoy.PageableCtx)
	if v != nil {
		return v.(datastore.Pageable)
	}
	return datastore.Pageable{}
}

func setAuthUserInContext(ctx context.Context, a *auth.AuthenticatedUser) context.Context {
	return context.WithValue(ctx, convoy.AuthUserCtx, a)
}

func GetAuthUserFromContext(ctx context.Context) *auth.AuthenticatedUser {
	return ctx.Value(convoy.AuthUserCtx).(*auth.AuthenticatedUser)
}

func RequireValidEnterpriseSSOLicense(l license.Licenser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.EnterpriseSSO() {
				slog.Warn("Enterprise SSO access denied - license required")
				_ = render.Render(w, r, util.NewErrorResponse("Access denied", http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractOrganisationID(r *http.Request) string {
	if orgID := chi.URLParam(r, "orgID"); orgID != "" {
		return orgID
	}

	if orgID := r.URL.Query().Get("orgID"); orgID != "" {
		return orgID
	}

	if orgID := r.URL.Query().Get("organisation_id"); orgID != "" {
		return orgID
	}

	return ""
}

func RequireValidGoogleOAuthLicense(l license.Licenser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.GoogleOAuth() {
				slog.Warn("Google OAuth access denied - license required")
				_ = render.Render(w, r, util.NewErrorResponse("Access denied", http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireValidPortalLinksLicense(l license.Licenser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.PortalLinks() {
				slog.Warn("Portal links access denied - license required")
				_ = render.Render(w, r, util.NewErrorResponse("Access denied", http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
