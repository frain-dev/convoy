package middleware

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/frain-dev/convoy/tracer"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/internal/pkg/apm"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/newrelic/go-agent/v3/newrelic"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/felixge/httpsnoop"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
)

type contextKey string

const (
	projectCtx          contextKey = "project"
	orgCtx              contextKey = "organisation"
	orgMemberCtx        contextKey = "organisation_member"
	endpointCtx         contextKey = "endpoint"
	endpointsCtx        contextKey = "endpoints"
	eventCtx            contextKey = "event"
	eventDeliveryCtx    contextKey = "eventDelivery"
	authLoginCtx        contextKey = "authLogin"
	authUserCtx         contextKey = "authUser"
	userCtx             contextKey = "user"
	pageableCtx         contextKey = "pageable"
	pageDataCtx         contextKey = "pageData"
	deliveryAttemptsCtx contextKey = "deliveryAttempts"
	hostCtx             contextKey = "host"
	endpointIdCtx       contextKey = "endpointId"
	endpointIdsCtx      contextKey = "endpointIds"
	portalLinkCtx       contextKey = "portal_link"
)

type Middleware struct {
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	endpointRepo      datastore.EndpointRepository
	projectRepo       datastore.ProjectRepository
	apiKeyRepo        datastore.APIKeyRepository
	subRepo           datastore.SubscriptionRepository
	sourceRepo        datastore.SourceRepository
	orgRepo           datastore.OrganisationRepository
	orgMemberRepo     datastore.OrganisationMemberRepository
	orgInviteRepo     datastore.OrganisationInviteRepository
	userRepo          datastore.UserRepository
	configRepo        datastore.ConfigurationRepository
	deviceRepo        datastore.DeviceRepository
	portalLinkRepo    datastore.PortalLinkRepository
	cache             cache.Cache
	logger            log.StdLogger
	limiter           limiter.RateLimiter
	tracer            tracer.Tracer
}

type CreateMiddleware struct {
	EventRepo         datastore.EventRepository
	EventDeliveryRepo datastore.EventDeliveryRepository
	EndpointRepo      datastore.EndpointRepository
	ProjectRepo       datastore.ProjectRepository
	ApiKeyRepo        datastore.APIKeyRepository
	SubRepo           datastore.SubscriptionRepository
	SourceRepo        datastore.SourceRepository
	OrgRepo           datastore.OrganisationRepository
	OrgMemberRepo     datastore.OrganisationMemberRepository
	OrgInviteRepo     datastore.OrganisationInviteRepository
	UserRepo          datastore.UserRepository
	ConfigRepo        datastore.ConfigurationRepository
	DeviceRepo        datastore.DeviceRepository
	PortalLinkRepo    datastore.PortalLinkRepository
	Cache             cache.Cache
	Logger            log.StdLogger
	Limiter           limiter.RateLimiter
	Tracer            tracer.Tracer
}

func NewMiddleware(cs *CreateMiddleware) *Middleware {
	return &Middleware{
		eventRepo:         cs.EventRepo,
		eventDeliveryRepo: cs.EventDeliveryRepo,
		endpointRepo:      cs.EndpointRepo,
		projectRepo:       cs.ProjectRepo,
		apiKeyRepo:        cs.ApiKeyRepo,
		subRepo:           cs.SubRepo,
		sourceRepo:        cs.SourceRepo,
		orgRepo:           cs.OrgRepo,
		orgMemberRepo:     cs.OrgMemberRepo,
		orgInviteRepo:     cs.OrgInviteRepo,
		userRepo:          cs.UserRepo,
		configRepo:        cs.ConfigRepo,
		deviceRepo:        cs.DeviceRepo,
		portalLinkRepo:    cs.PortalLinkRepo,
		cache:             cs.Cache,
		logger:            cs.Logger,
		limiter:           cs.Limiter,
		tracer:            cs.Tracer,
	}
}

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

func (m *Middleware) InstrumentPath(path string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m := httpsnoop.CaptureMetrics(next, w, r)
			metrics.RequestDuration().WithLabelValues(r.Method, path,
				strconv.Itoa(m.Code)).Observe(m.Duration.Seconds())
		})
	}
}

func (m *Middleware) InstrumentRequests() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			txn, r, w := apm.StartWebTransaction(r.URL.Path, r, w)
			defer txn.End()

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) WriteRequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", r.Context().Value(middleware.RequestIDKey).(string))
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) SetupCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.Get()
		if err != nil {
			m.logger.WithError(err).Error("failed to load configuration")
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

func (m *Middleware) JsonResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func FilterDeletedEndpoints(endpoints []datastore.Endpoint) []datastore.Endpoint {
	activeEndpoints := make([]datastore.Endpoint, 0)
	for _, endpoint := range endpoints {
		if endpoint.DeletedAt.IsZero() {
			activeEndpoints = append(activeEndpoints, endpoint)
		}
	}
	return activeEndpoints
}

func (m *Middleware) RequireAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds, err := GetAuthFromRequest(r)
			if err != nil {
				m.logger.WithError(err).Error("failed to get auth from request")
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
				return
			}

			rc, err := realm_chain.Get()
			if err != nil {
				m.logger.WithError(err).Error("failed to get realm chain")
				_ = render.Render(w, r, util.NewErrorResponse("internal server error", http.StatusInternalServerError))
				return
			}

			authUser, err := rc.Authenticate(r.Context(), creds)
			if err != nil {
				m.logger.WithError(err).Error("failed to authenticate")
				_ = render.Render(w, r, util.NewErrorResponse("authorization failed", http.StatusUnauthorized))
				return
			}

			authCtx := context.WithValue(r.Context(), policies.AuthCtxKey, authUser)

			r = r.WithContext(setAuthUserInContext(authCtx, authUser))
			next.ServeHTTP(w, r)
		})
	}
}

func GetAuthFromRequest(r *http.Request) (*auth.Credential, error) {
	val := r.Header.Get("Authorization")
	authInfo := strings.Split(val, " ")

	if len(authInfo) != 2 {
		err := errors.New("invalid header structure")
		apm.NoticeError(r.Context(), err)
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

		return &auth.Credential{
			Type:  auth.CredentialTypeJWT,
			Token: authToken,
		}, nil

	default:
		return nil, fmt.Errorf("unknown credential type: %s", credType.String())
	}
}

func (m *Middleware) Pagination(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPerPage := r.URL.Query().Get("perPage")
		rawDirection := r.URL.Query().Get("direction")
		rawNextCursor := r.URL.Query().Get("next_page_cursor")
		rawPrevCursor := r.URL.Query().Get("prev_page_cursor")

		if len(rawPerPage) == 0 {
			rawPerPage = "20"
		}

		if len(rawDirection) == 0 {
			rawDirection = "next"
		}

		if len(rawNextCursor) == 0 {
			const jsMaxInt = ^uint64(0) >> 1
			rawNextCursor = fmt.Sprintf("%d", jsMaxInt)
		}

		if len(rawPrevCursor) == 0 {
			rawPrevCursor = ""
		}

		perPage, err := strconv.Atoi(rawPerPage)
		if err != nil {
			perPage = 20
		}

		pageable := datastore.Pageable{
			PerPage:    perPage,
			Direction:  datastore.PageDirection(rawDirection),
			NextCursor: rawNextCursor,
			PrevCursor: rawPrevCursor,
		}

		// fmt.Printf("middleware %+v\n", pageable)

		r = r.WithContext(setPageableInContext(r.Context(), pageable))
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) LogHttpRequest() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				requestFields := requestLogFields(r)
				responseFields := responseLogFields(ww, start)

				logFields := map[string]interface{}{
					"httpRequest":  requestFields,
					"httpResponse": responseFields,
				}

				lvl, err := m.statusLevel(ww.Status()).ToLogrusLevel()
				if err != nil {
					m.logger.WithError(err).Error("Failed to generate status level")
				}

				m.logger.WithFields(logFields).Log(lvl, requestFields["requestURL"])
			}()

			requestID := middleware.GetReqID(r.Context())
			ctx := log.NewContext(r.Context(), m.logger, log.Fields{"request_id": requestID})
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

	cfg, err := config.Get()
	if err != nil {
		return nil
	}

	if cfg.Tracer.Type == config.NewRelicTracerProvider {
		txn := newrelic.FromContext(r.Context()).GetLinkingMetadata()

		if cfg.Tracer.NewRelic.DistributedTracerEnabled {
			requestFields["traceID"] = txn.TraceID
			requestFields["spanID"] = txn.SpanID
		}

		requestFields["entityGUID"] = txn.EntityGUID
		requestFields["entityType"] = txn.EntityType
	}

	return requestFields
}

func responseLogFields(w middleware.WrapResponseWriter, t time.Time) map[string]interface{} {
	responseFields := map[string]interface{}{
		"status":  w.Status(),
		"byes":    w.BytesWritten(),
		"latency": time.Since(t),
	}

	if len(w.Header()) > 0 {
		responseFields["header"] = headerFields(w.Header())
	}

	return responseFields
}

func (m *Middleware) statusLevel(status int) log.Level {
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

var guestRoutes = []string{
	"/ui/auth/login",
	"/ui/auth/token/refresh",
	"/ui/organisations/process_invite",
	"/ui/users/token",
	"/ui/users/forgot-password",
	"/ui/users/reset-password",
	"/ui/users/verify_email",
	"/ui/auth/register",
}

func ShouldAuthRoute(r *http.Request) bool {
	for _, route := range guestRoutes {
		if r.URL.Path == route {
			return false
		}
	}

	return true
}

func EnsurePeriod(start time.Time, end time.Time) error {
	if start.Unix() > end.Unix() {
		return errors.New("startDate cannot be greater than endDate")
	}

	return nil
}

func (m *Middleware) ComputeDashboardMessages(ctx context.Context, projectID string, searchParams datastore.SearchParams, period datastore.Period) (uint64, []datastore.EventInterval, error) {
	var messagesSent uint64

	messages, err := m.eventDeliveryRepo.LoadEventDeliveriesIntervals(ctx, projectID, searchParams, period, 1)
	if err != nil {
		m.logger.WithError(err).Error("failed to load message intervals - ")
		return 0, nil, err
	}

	for _, m := range messages {
		messagesSent += m.Count
	}

	return messagesSent, messages, nil
}

func setPageableInContext(ctx context.Context, pageable datastore.Pageable) context.Context {
	return context.WithValue(ctx, pageableCtx, pageable)
}

func GetPageableFromContext(ctx context.Context) datastore.Pageable {
	return ctx.Value(pageableCtx).(datastore.Pageable)
}

func GetPaginationDataFromContext(ctx context.Context) *datastore.PaginationData {
	return ctx.Value(pageDataCtx).(*datastore.PaginationData)
}

func setAuthUserInContext(ctx context.Context, a *auth.AuthenticatedUser) context.Context {
	return context.WithValue(ctx, authUserCtx, a)
}

func GetAuthUserFromContext(ctx context.Context) *auth.AuthenticatedUser {
	return ctx.Value(authUserCtx).(*auth.AuthenticatedUser)
}

func GetAuthLoginFromContext(ctx context.Context) *AuthorizedLogin {
	return ctx.Value(authLoginCtx).(*AuthorizedLogin)
}

func findMessageDeliveryAttempt(attempts *[]datastore.DeliveryAttempt, id string) (*datastore.DeliveryAttempt, error) {
	for _, a := range *attempts {
		if a.UID == id {
			return &a, nil
		}
	}
	return nil, datastore.ErrEventDeliveryAttemptNotFound
}
