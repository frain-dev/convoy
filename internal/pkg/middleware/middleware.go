package middleware

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/riandyrn/otelchi"

	oteltrace "go.opentelemetry.io/otel/trace"

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
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// untracedHTTPPaths are excluded from otelchi span creation. Health probes and
// metrics scrapes fire constantly and add no value to a trace tree; including
// them inflates exporter cost and noise without informing any debugging.
var untracedHTTPPaths = map[string]struct{}{
	"/healthz":          {},
	"/health":           {},
	"/metrics":          {},
	"/queue/monitoring": {},
}

// safeHeaders is the whitelist of header names that are safe to include
// in log output. Sensitive headers (auth tokens, cookies, signatures) are
// intentionally redacted. A whitelist is used instead of a blacklist because
// it is easier to know what headers are safe to log.
var safeHeaders = map[string]struct{}{
	"connection":                    {},
	"content-type":                  {},
	"content-length":                {},
	"user-agent":                    {},
	"accept":                        {},
	"accept-encoding":               {},
	"accept-language":               {},
	"host":                          {},
	"x-request-id":                  {},
	"cache-control":                 {},
	"pragma":                        {},
	"upgrade-insecure-requests":     {},
	"origin":                        {},
	"x-datadog-trace-id":            {},
	"x-convoy-version":              {},
	"access-control-allow-headers":  {},
	"access-control-allow-methods":  {},
	"access-control-allow-origin":   {},
	"access-control-expose-headers": {},
}

// sensitiveHeaders contains header names (lowercase) that should always be
// redacted, even if they are accidentally added to safeHeaders.
var sensitiveHeaders = map[string]struct{}{
	"referer":             {},
	"authorization":       {},
	"x-forwarded-for":     {},
	"x-webhook-secret":    {},
	"x-webhook-signature": {},
	"x-real-ip":           {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
	"x-api-key":           {},
	"x-auth-token":        {},
}

var sensitivePatterns = []string{
	"-secret",
	"-token",
	"-signature",
	"-key",
	"-credential",
	"-password",
}

var (
	ErrValidLicenseRequired = errors.New("access to this resource requires a valid license")

	// skipLoggingPaths defines paths that should not be logged by the request logger
	skipLoggingPaths = []string{
		"/billing/organisations/",
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

			val := projectIDForInstrumentation(r)
			mm.RecordIngestLatency(val, m.Duration.Seconds())
			mm.IncrementIngestTotal("http", val)
		})
	}
}

func projectIDForInstrumentation(r *http.Request) string {
	if cachedProject := r.Context().Value(convoy.ProjectCtx); cachedProject != nil {
		project, ok := cachedProject.(*datastore.Project)
		if ok && project != nil {
			return project.UID
		}
	}

	return chi.URLParam(r, "projectID")
}

func InstrumentRequests(serverName string, r chi.Router, tp oteltrace.TracerProvider) func(next http.Handler) http.Handler {
	opts := []otelchi.Option{
		otelchi.WithChiRoutes(r),
		otelchi.WithFilter(func(req *http.Request) bool {
			_, skip := untracedHTTPPaths[req.URL.Path]
			return !skip
		}),
	}
	if tp != nil {
		opts = append(opts, otelchi.WithTracerProvider(tp))
	}
	return otelchi.Middleware(serverName, opts...)
}

// EnrichSpanFromRoute attaches Convoy-domain identifiers from the matched chi
// route as attributes on the active span.
//
// Must run *after* InstrumentRequests so the span exists. We enrich after
// next.ServeHTTP returns because chi populates URLParams during route walking
// (which happens in Mux.routeHTTP, after Use-registered middleware runs). The
// otelchi span is still recording at that point because its own middleware is
// the outer wrapper and won't call End() until we return.
func EnrichSpanFromRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)

		span := oteltrace.SpanFromContext(r.Context())
		if !span.IsRecording() {
			return
		}
		if v := chi.URLParam(r, "projectID"); v != "" {
			span.SetAttributes(tracer.AttrProjectID.String(v))
		}
		if v := chi.URLParam(r, "endpointID"); v != "" {
			span.SetAttributes(tracer.AttrEndpointID.String(v))
		}
		if v := chi.URLParam(r, "eventID"); v != "" {
			span.SetAttributes(tracer.AttrEventID.String(v))
		}
		if v := chi.URLParam(r, "eventDeliveryID"); v != "" {
			span.SetAttributes(tracer.AttrEventDeliveryID.String(v))
		}
		if v := chi.URLParam(r, "subscriptionID"); v != "" {
			span.SetAttributes(tracer.AttrSubscriptionID.String(v))
		}
		if v := chi.URLParam(r, "deliveryAttemptID"); v != "" {
			span.SetAttributes(tracer.AttrDeliveryAttemptID.String(v))
		}
	})
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

func SetupCORS(logger log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg, err := config.Get()
			if err != nil {
				logger.ErrorContext(r.Context(), "failed to load configuration", "error", err)
				return
			}

			// We only reflect the request Origin when the server is explicitly running in
			// `development` so localhost ports / CRA dev servers can talk to the API. In any
			// other environment the existing edge/proxy CORS rules apply, and we never let an
			// unverified Origin become Access-Control-Allow-Origin in staging or production.
			if env := cfg.Environment; string(env) == "development" {
				allowOrigin := strings.TrimSpace(r.Header.Get("Origin"))
				if allowOrigin == "" {
					allowOrigin = cfg.Host
				}
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				// We reflect a specific Origin (never "*"), so the response varies by
				// Origin and the queue monitoring session flow can send credentials.
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
				w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Convoy-Version, X-Organisation-Id")
			}

			if r.Method == "OPTIONS" {
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func JsonResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func RequireAuth(logger log.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds, err := GetAuthFromRequest(r)
			if err != nil {
				logger.ErrorContext(r.Context(), "failed to get auth from request", "error", err)
				_ = render.Render(w, r, util.NewErrorResponse("Authentication required", http.StatusUnauthorized))
				return
			}

			rc, err := realm_chain.Get()
			if err != nil {
				logger.ErrorContext(r.Context(), "failed to get realm chain", "error", err)
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

// OptionalAuth authenticates the request when credentials are present but never
// rejects: requests without (or with invalid) credentials pass through as guests
// with no auth user in context. Use it on guest-listed routes whose handlers serve
// richer data for signed-in callers, e.g. /ui/license/features resolving the
// caller's org count so the UI can gate the add-organisation action.
// Failure policy: fail open (guest). This middleware only enriches display data;
// hard enforcement stays with the authenticated create/update endpoints.
func OptionalAuth(logger log.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds, err := GetAuthFromRequest(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			rc, err := realm_chain.Get()
			if err != nil {
				logger.ErrorContext(r.Context(), "optional auth: failed to get realm chain", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			authUser, err := rc.Authenticate(r.Context(), creds)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			authCtx := context.WithValue(r.Context(), convoy.AuthUserCtx, authUser)
			r = r.WithContext(setAuthUserInContext(authCtx, authUser))
			next.ServeHTTP(w, r)
		})
	}
}

// RequireQueueSessionCookie allows only a valid convoy_queue_session cookie (dashboard iframe at /queue/monitoring/embed).
func RequireQueueSessionCookie(validateCookie func(context.Context, string) bool) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("convoy_queue_monitoring_session")
			if err != nil || !validateCookie(r.Context(), cookie.Value) {
				_ = render.Render(w, r, util.NewErrorResponse("Authentication required", http.StatusUnauthorized))
				return
			}
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
			Search:     strings.TrimSpace(r.URL.Query().Get("q")),
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

			defer func() {
				lvl := statusLevel(ww.Status())

				requestFields := requestLogFields(r)
				responseFields := responseLogFields(ww, start)

				if shouldSkipLogging(requestFields, responseFields) {
					return
				}

				logArgs := []any{"httpRequest", requestFields, "httpResponse", responseFields}
				if orgID := extractOrganisationID(r); orgID != "" {
					logArgs = append(logArgs, "organisation_id", orgID)
				}

				if projectId := extractProjectID(r); projectId != "" {
					logArgs = append(logArgs, "project_id", projectId)
				}

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

	span := oteltrace.SpanFromContext(r.Context())

	requestFields["traceId"] = span.SpanContext().TraceID()
	requestFields["spanId"] = span.SpanContext().SpanID()

	return requestFields
}

func responseLogFields(w middleware.WrapResponseWriter, t time.Time) map[string]interface{} {
	responseFields := map[string]interface{}{
		"status":  w.Status(),
		"bytes":   w.BytesWritten(),
		"latency": time.Since(t),
		// Do not log response body content to avoid leaking sensitive data.
		"body": "***",
	}

	if len(w.Header()) > 0 {
		responseFields["header"] = headerFields(w.Header())
	}

	return responseFields
}

func statusLevel(status int) log.Level {
	switch {
	case status <= 0:
		return log.LevelWarn
	case status < 400:
		return log.LevelInfo
	case status < 500:
		return log.LevelWarn
	default:
		return log.LevelError
	}
}

func headerFields(header http.Header) map[string]string {
	headerField := map[string]string{}

	for k, v := range header {
		k = strings.ToLower(k)
		if len(v) == 0 {
			continue
		}

		// Redact anything not on the safe allowlist (this also covers the
		// explicit sensitive set and the sensitive suffix patterns).
		if isSensitiveHeaderKey(k) {
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

// isSensitiveHeaderKey reports whether a header value must be redacted. It uses
// the same allowlist as request logging: a header is redacted unless it is on
// the explicit safe list, and any header on the sensitive list or matching a
// sensitive suffix pattern is always redacted. The key must be lowercase.
func isSensitiveHeaderKey(lowerKey string) bool {
	if _, isSensitive := sensitiveHeaders[lowerKey]; isSensitive {
		return true
	}

	for _, suffix := range sensitivePatterns {
		if strings.HasSuffix(lowerKey, suffix) {
			return true
		}
	}

	if _, safe := safeHeaders[lowerKey]; !safe {
		return true
	}

	return false
}

// isSensitiveResponseHeaderKey reports whether a header must be redacted from a
// delivery-attempt/event-delivery API response. It mirrors isSensitiveHeaderKey
// (used for logs) but keeps webhook signatures visible: a signature is an HMAC
// the receiver verifies and is already sent to the endpoint on the wire, and the
// dashboard surfaces it so users can debug signature verification. Signatures are
// not credentials, so unlike the log path they are not masked here. Auth
// credentials (Authorization, cookies, api keys, secrets, tokens) stay redacted.
// The key must be lowercase.
func isSensitiveResponseHeaderKey(lowerKey string) bool {
	if lowerKey == "x-convoy-signature" || strings.HasSuffix(lowerKey, "-signature") {
		return false
	}

	return isSensitiveHeaderKey(lowerKey)
}

// redactedHeaderValue fully masks a sensitive header value in API responses.
// The value is redacted entirely (fail-closed) so a customer-injected header
// name that holds a secret cannot leak any bytes to lower-trust portal viewers,
// the only callers this redaction path serves (API-key and authenticated
// dashboard callers receive raw headers). Webhook signatures are exempted
// upstream by isSensitiveResponseHeaderKey and stay visible.
const redactedHeaderValue = "***"

// RedactSensitiveHeaders returns a copy of a single-valued header map with
// sensitive values fully masked for API responses. Redaction is allowlist
// based: only headers on the safe allowlist (and webhook signatures) survive,
// everything else is masked, so unknown/injected header names fail closed. The
// input map is not mutated, so callers can safely redact a response view while
// the stored/dispatched headers keep their real values. A nil input returns nil.
func RedactSensitiveHeaders(header map[string]string) map[string]string {
	if header == nil {
		return nil
	}

	redacted := make(map[string]string, len(header))
	for k, v := range header {
		if isSensitiveResponseHeaderKey(strings.ToLower(k)) {
			redacted[k] = redactedHeaderValue
			continue
		}
		redacted[k] = v
	}

	return redacted
}

// RedactSensitiveMultiHeaders is the multi-valued (map[string][]string) variant
// of RedactSensitiveHeaders. Each sensitive value is fully masked while the
// value count is preserved. The input map is not mutated. A nil input returns nil.
func RedactSensitiveMultiHeaders(header map[string][]string) map[string][]string {
	if header == nil {
		return nil
	}

	redacted := make(map[string][]string, len(header))
	for k, v := range header {
		if isSensitiveResponseHeaderKey(strings.ToLower(k)) {
			masked := make([]string, len(v))
			for i := range v {
				masked[i] = redactedHeaderValue
			}
			redacted[k] = masked
			continue
		}
		redacted[k] = v
	}

	return redacted
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

func RequireValidEnterpriseSSOLicense(l license.Licenser, logger log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.EnterpriseSSO() {
				logger.WarnContext(r.Context(), "Enterprise SSO access denied - license required")
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

func extractProjectID(r *http.Request) string {
	if projectId := chi.URLParam(r, "projectID"); projectId != "" {
		return projectId
	}

	if projectId := r.URL.Query().Get("projectId"); projectId != "" {
		return projectId
	}

	if projectId := r.URL.Query().Get("project_id"); projectId != "" {
		return projectId
	}

	return ""
}

func RequireValidGoogleOAuthLicense(l license.Licenser, logger log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.GoogleOAuth() {
				logger.WarnContext(r.Context(), "Google OAuth access denied - license required")
				_ = render.Render(w, r, util.NewErrorResponse("Access denied", http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func RequireValidPortalLinksLicense(l license.Licenser, logger log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !l.PortalLinks() {
				logger.WarnContext(r.Context(), "Portal links access denied - license required")
				_ = render.Render(w, r, util.NewErrorResponse("Access denied", http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireAsynqMonitoring gates queue monitoring at request time so a runtime
// licenser refresh (e.g. after self-hosted trial start) can unlock routes without
// a process restart. The licenser is resolved per request via the getter because
// a self-hosted trial swaps the shared APIOptions.Licenser in place; capturing the
// boot-time value would keep serving the pre-trial (community) gate and return 401
// even after the license reports AsynqMonitoring=true. Fail closed when the getter
// is nil, returns nil, or the deployment license lacks the feature.
func RequireAsynqMonitoring(licenser func() license.Licenser, logger log.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var l license.Licenser
			if licenser != nil {
				l = licenser()
			}
			if l == nil || !l.AsynqMonitoring() {
				logger.WarnContext(r.Context(), "Asynq monitoring access denied - license required")
				_ = render.Render(w, r, util.NewErrorResponse("Access denied", http.StatusUnauthorized))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
