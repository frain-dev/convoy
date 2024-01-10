package middleware

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/riandyrn/otelchi"

	"github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/felixge/httpsnoop"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
)

const (
	AuthUserCtx types.ContextKey = "authUser"
	pageableCtx types.ContextKey = "pageable"
)

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

func InstrumentPath(path string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m := httpsnoop.CaptureMetrics(next, w, r)
			metrics.RequestDuration().WithLabelValues(r.Method, path,
				strconv.Itoa(m.Code)).Observe(m.Duration.Seconds())
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

func CanAccessFeature(fflag *fflag.FFlag, featureKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg, err := config.Get()
			if err != nil {
				log.FromContext(r.Context()).WithError(err).Error("failed to load configuration")
				_ = render.Render(w, r, util.NewErrorResponse("something went wrong", http.StatusInternalServerError))
				return
			}

			if !fflag.CanAccessFeature(featureKey, &cfg) {
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
			log.FromContext(r.Context()).WithError(err).Error("failed to load configuration")
			return
		}

		if env := cfg.Environment; string(env) == "development" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
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
				log.FromContext(r.Context()).WithError(err).Error("failed to get auth from request")
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
				return
			}

			rc, err := realm_chain.Get()
			if err != nil {
				log.FromContext(r.Context()).WithError(err).Error("failed to get realm chain")
				_ = render.Render(w, r, util.NewErrorResponse("internal server error", http.StatusInternalServerError))
				return
			}

			authUser, err := rc.Authenticate(r.Context(), creds)
			if err != nil {
				log.FromContext(r.Context()).WithError(err).Error("failed to authenticate")
				_ = render.Render(w, r, util.NewErrorResponse("authorization failed", http.StatusUnauthorized))
				return
			}

			authCtx := context.WithValue(r.Context(), AuthUserCtx, authUser)

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
			return nil, errors.New("empty api key or token")
		}

		prefix := fmt.Sprintf("%s%s", util.Prefix, util.Seperator)
		if strings.HasPrefix(authToken, prefix) {
			return &auth.Credential{
				Type:   auth.CredentialTypeAPIKey,
				APIKey: authToken,
			}, nil
		}

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
				lvl, err := statusLevel(ww.Status()).ToLogrusLevel()
				if err != nil {
					log.FromContext(r.Context()).WithError(err).Error("Failed to generate status level")
				}

				requestFields := requestLogFields(r)
				responseFields := responseLogFields(ww, wbuf, start, lvl)

				logFields := map[string]interface{}{
					"httpRequest":  requestFields,
					"httpResponse": responseFields,
				}

				log.FromContext(r.Context()).WithFields(logFields).Log(lvl, requestFields["requestURL"])
			}()

			requestID := middleware.GetReqID(r.Context())
			ctx := log.NewContext(r.Context(), a.Logger, log.Fields{"request_id": requestID})
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

	//cfg, err := config.Get()
	//if err != nil {
	//	return nil
	//}

	//if cfg.Tracer.Type == config.NewRelicTracerProvider {
	//	txn := newrelic.FromContext(r.Context()).GetLinkingMetadata()

	//	if cfg.Tracer.NewRelic.DistributedTracerEnabled {
	//		requestFields["traceId"] = txn.TraceID
	//      requestFields["spanId"] = txn.SpanID
	//	}

	//	requestFields["entity.guid"] = txn.EntityGUID
	//	requestFields["entity.name"] = txn.EntityName
	//}

	return requestFields
}

func responseLogFields(w middleware.WrapResponseWriter, wbuf *bytes.Buffer, t time.Time, lvl logrus.Level) map[string]interface{} {
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

func statusLevel(status int) log.Level {
	switch {
	case status <= 0:
		return log.WarnLevel
	case status < 400:
		return log.InfoLevel
	case status >= 400 && status < 500:
		return log.WarnLevel
	case status >= 500:
		return log.ErrorLevel
	default:
		return log.InfoLevel
	}
}

func headerFields(header http.Header) map[string]string {
	headerField := map[string]string{}

	for k, v := range header {
		k = strings.ToLower(k)
		switch {
		case len(v) == 0:
			continue
		case len(v) == 1:
			headerField[k] = v[0]
		default:
			headerField[k] = fmt.Sprintf("[%s]", strings.Join(v, "], ["))
		}
		if k == "authorization" || k == "cookie" || k == "set-cookie" {
			headerField[k] = "***"
		}
	}

	return headerField
}

func EnsurePeriod(start time.Time, end time.Time) error {
	if start.Unix() > end.Unix() {
		return errors.New("startDate cannot be greater than endDate")
	}

	return nil
}

func setPageableInContext(ctx context.Context, pageable datastore.Pageable) context.Context {
	return context.WithValue(ctx, pageableCtx, pageable)
}

func GetPageableFromContext(ctx context.Context) datastore.Pageable {
	return ctx.Value(pageableCtx).(datastore.Pageable)
}

func setAuthUserInContext(ctx context.Context, a *auth.AuthenticatedUser) context.Context {
	return context.WithValue(ctx, AuthUserCtx, a)
}

func GetAuthUserFromContext(ctx context.Context) *auth.AuthenticatedUser {
	return ctx.Value(AuthUserCtx).(*auth.AuthenticatedUser)
}
