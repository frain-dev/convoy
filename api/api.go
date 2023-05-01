package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/api/dashboard"
	"github.com/frain-dev/convoy/api/policies"
	portalapi "github.com/frain-dev/convoy/api/portal-api"
	"github.com/frain-dev/convoy/api/public"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed dashboard/ui/build
var reactFS embed.FS

func reactRootHandler(rw http.ResponseWriter, req *http.Request) {
	p := req.URL.Path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
		req.URL.Path = p
	}
	p = path.Clean(p)
	f := fs.FS(reactFS)
	static, err := fs.Sub(f, "dashboard/ui/build")
	if err != nil {
		return
	}
	if _, err := static.Open(strings.TrimLeft(p, "/")); err != nil { // If file not found server index/html from root
		req.URL.Path = "/"
	}
	http.FileServer(http.FS(static)).ServeHTTP(rw, req)
}

const (
	GET    = "GET"
	POST   = "POST"
	PUT    = "PUT"
	DELETE = "DELETE"
)

type ApplicationHandler struct {
	Router http.Handler
	A      *types.APIOptions
}

func NewApplicationHandler(a *types.APIOptions) (*ApplicationHandler, error) {
	az, err := authz.NewAuthz(&authz.AuthzOpts{
		AuthCtxKey: authz.AuthCtxType(middleware.AuthUserCtx),
	})
	if err != nil {
		return &ApplicationHandler{}, err
	}
	a.Authz = az

	return &ApplicationHandler{A: a}, nil
}

func (a *ApplicationHandler) BuildRoutes() *chi.Mux {
	router := chi.NewMux()

	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.Recoverer)
	router.Use(middleware.WriteRequestIDHeader)
	router.Use(middleware.InstrumentRequests())
	router.Use(middleware.LogHttpRequest(a.A))
	router.Use(chiMiddleware.Maybe(middleware.SetupCORS, shouldApplyCORS))

	// Ingestion API.
	router.Route("/ingest", func(ingestRouter chi.Router) {
		ingestRouter.Get("/{maskID}", a.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", a.IngestEvent)
	})

	// Public API.
	publicAPI := &public.PublicHandler{A: a.A}
	router.Mount("/api", publicAPI.BuildRoutes())

	// Dashboard API.
	a.RegisterDashboardRoutes(router)

	portalAPI := &portalapi.PortalLinkHandler{A: a.A}
	router.Mount("/portal-api", portalAPI.BuildRoutes())

	router.Handle("/queue/monitoring/*", a.A.Queue.(*redisqueue.RedisQueue).Monitor())
	router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
	router.HandleFunc("/*", reactRootHandler)

	metrics.RegisterQueueMetrics(a.A.Queue)
	prometheus.MustRegister(metrics.RequestDuration())
	a.Router = router

	return router
}

func (a *ApplicationHandler) RegisterPolicy() error {
	var err error

	err = a.A.Authz.RegisterPolicy(func() authz.Policy {
		po := &policies.OrganisationPolicy{
			BasePolicy:             authz.NewBasePolicy(),
			OrganisationMemberRepo: postgres.NewOrgMemberRepo(a.A.DB),
		}

		po.SetRule("manage", authz.RuleFunc(po.Manage))

		return po
	}())

	if err != nil {
		return err
	}

	err = a.A.Authz.RegisterPolicy(func() authz.Policy {
		po := &policies.ProjectPolicy{
			BasePolicy:             authz.NewBasePolicy(),
			OrganisationRepo:       postgres.NewOrgRepo(a.A.DB),
			OrganisationMemberRepo: postgres.NewOrgMemberRepo(a.A.DB),
		}

		po.SetRule("manage", authz.RuleFunc(po.Manage))

		return po
	}())

	return err
}

func (a *ApplicationHandler) RegisterDashboardRoutes(r *chi.Mux) {
	dh := &dashboard.DashboardHandler{A: a.A}
	uiMiddlewares := chi.Middlewares{
		middleware.JsonResponse,
		chiMiddleware.Maybe(middleware.RequireAuth(), shouldAuthRoute),
	}
	uiMiddlewaresWithPagination := chi.Chain(append(
		uiMiddlewares,
		chi.Middlewares{middleware.Pagination}...)...)

	r.Method(POST, "/ui/organisations/process_invite", uiMiddlewares.HandlerFunc(dh.ProcessOrganisationMemberInvite))
	r.Method(GET, "/ui/users/token", uiMiddlewares.HandlerFunc(dh.FindUserByInviteToken))
	r.Method(POST, "/ui/users/forgot-password", uiMiddlewares.HandlerFunc(dh.ForgotPassword))
	r.Method(POST, "/ui/users/reset-password", uiMiddlewares.HandlerFunc(dh.ResetPassword))
	r.Method(POST, "/ui/users/verify_email", uiMiddlewares.HandlerFunc(dh.VerifyEmail))
	r.Method(POST, "/ui/users/resend_verification_email", uiMiddlewares.HandlerFunc(dh.ResendVerificationEmail))

	r.Method(POST, "/ui/auth/login", uiMiddlewares.HandlerFunc(dh.LoginUser))
	r.Method(POST, "/ui/auth/register", uiMiddlewares.HandlerFunc(dh.RegisterUser))
	r.Method(POST, "/ui/auth/token/refresh", uiMiddlewares.HandlerFunc(dh.RefreshToken))
	r.Method(POST, "/ui/auth/logout", uiMiddlewares.HandlerFunc(dh.LogoutUser))

	r.Method(GET, "/ui/users/{userID}/profile", uiMiddlewares.HandlerFunc(dh.GetUser))
	r.Method(PUT, "/ui/users/{userID}/profile", uiMiddlewares.HandlerFunc(dh.UpdateUser))
	r.Method(PUT, "/ui/users/{userID}/password", uiMiddlewares.HandlerFunc(dh.UpdatePassword))
	r.Method(POST, "/ui/users/{userID}/security/personal_api_keys", uiMiddlewares.HandlerFunc(dh.CreatePersonalAPIKey))
	r.Method(PUT, "/ui/users/{userID}/security/{keyID}/revoke", uiMiddlewares.HandlerFunc(dh.RevokePersonalAPIKey))
	r.Method(GET, "/ui/users/{userID}/security/", uiMiddlewaresWithPagination.HandlerFunc(dh.GetAPIKeys))

	r.Method(GET, "/ui/organisations", uiMiddlewaresWithPagination.HandlerFunc(dh.GetOrganisationsPaged))
	r.Method(POST, "/ui/organisations", uiMiddlewares.HandlerFunc(dh.CreateOrganisation))
	r.Method(GET, "/ui/organisations/{orgID}", uiMiddlewares.HandlerFunc(dh.GetOrganisation))
	r.Method(GET, "/ui/organisations/{orgID}", uiMiddlewares.HandlerFunc(dh.GetOrganisation))
	r.Method(PUT, "/ui/organisations/{orgID}", uiMiddlewares.HandlerFunc(dh.UpdateOrganisation))
	r.Method(DELETE, "/ui/organisations/{orgID}", uiMiddlewares.HandlerFunc(dh.DeleteOrganisation))
	r.Method(POST, "/ui/organisations/{orgID}/invites", uiMiddlewares.HandlerFunc(dh.InviteUserToOrganisation))
	r.Method(GET, "/ui/organisations/{orgID}/invites/pending", uiMiddlewaresWithPagination.HandlerFunc(dh.GetPendingOrganisationInvites))
	r.Method(POST, "/ui/organisations/{orgID}/invites/{inviteID}/resend", uiMiddlewares.HandlerFunc(dh.ResendOrganizationInvite))
	r.Method(POST, "/ui/organisations/{orgID}/invites/{inviteID}/cancel", uiMiddlewares.HandlerFunc(dh.CancelOrganizationInvite))

	r.Method(GET, "/ui/organisations/{orgID}/members", uiMiddlewaresWithPagination.HandlerFunc(dh.GetOrganisationMembers))
	r.Method(GET, "/ui/organisations/{orgID}/members/{memberID}", uiMiddlewares.HandlerFunc(dh.GetOrganisationMember))
	r.Method(PUT, "/ui/organisations/{orgID}/members/{memberID}", uiMiddlewares.HandlerFunc(dh.UpdateOrganisationMember))
	r.Method(DELETE, "/ui/organisations/{orgID}/members/{memberID}", uiMiddlewares.HandlerFunc(dh.DeleteOrganisationMember))

	r.Method(POST, "/ui/organisations/{orgID}/projects", uiMiddlewares.HandlerFunc(dh.CreateProject))
	r.Method(GET, "/ui/organisations/{orgID}/projects", uiMiddlewaresWithPagination.HandlerFunc(dh.GetProjects))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}", uiMiddlewares.HandlerFunc(dh.GetProject))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/stats", uiMiddlewares.HandlerFunc(dh.GetProjectStatistics))
	r.Method(PUT, "/ui/organisations/{orgID}/projects/{projectID}", uiMiddlewares.HandlerFunc(dh.UpdateProject))
	r.Method(DELETE, "/ui/organisations/{orgID}/projects/{projectID}", uiMiddlewares.HandlerFunc(dh.DeleteProject))
	r.Method(PUT, "/ui/organisations/{orgID}/projects/{projectID}/security/keys/regenerate", uiMiddlewares.HandlerFunc(dh.RegenerateProjectAPIKey))

	r.Method(POST, "/ui/organisations/{orgID}/projects/{projectID}/endpoints", uiMiddlewares.HandlerFunc(dh.CreateEndpoint))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/endpoints", uiMiddlewaresWithPagination.HandlerFunc(dh.GetEndpoints))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/endpoints/{endpointID}", uiMiddlewares.HandlerFunc(dh.GetEndpoint))
	r.Method(PUT, "/ui/organisations/{orgID}/projects/{projectID}/endpoints/{endpointID}", uiMiddlewares.HandlerFunc(dh.UpdateEndpoint))
	r.Method(DELETE, "/ui/organisations/{orgID}/projects/{projectID}/endpoints/{endpointID}", uiMiddlewares.HandlerFunc(dh.DeleteEndpoint))
	r.Method(PUT, "/ui/organisations/{orgID}/projects/{projectID}/endpoints/{endpointID}/toggle_status", uiMiddlewares.HandlerFunc(dh.ToggleEndpointStatus))
	r.Method(PUT, "/ui/organisations/{orgID}/projects/{projectID}/endpoints/{endpointID}/expire_secret", uiMiddlewares.HandlerFunc(dh.ExpireSecret))
	r.Method(PUT, "/ui/organisations/{orgID}/projects/{projectID}/endpoints/{endpointID}/pause", uiMiddlewares.HandlerFunc(dh.PauseEndpoint))

	r.Method(POST, "/ui/organisations/{orgID}/projects/{projectID}/events", uiMiddlewares.HandlerFunc(dh.CreateEndpointEvent))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/events", uiMiddlewaresWithPagination.HandlerFunc(dh.GetEventsPaged))
	r.Method(POST, "/ui/organisations/{orgID}/projects/{projectID}/events/batchreplay", uiMiddlewares.HandlerFunc(dh.BatchReplayEvents))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/events/countbatchreplayevents", uiMiddlewares.HandlerFunc(dh.CountAffectedEvents))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/events/{eventID}", uiMiddlewares.HandlerFunc(dh.GetEndpointEvent))
	r.Method(PUT, "/ui/organisations/{orgID}/projects/{projectID}/events/{eventID}/replay", uiMiddlewares.HandlerFunc(dh.ReplayEndpointEvent))

	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/eventdeliveries", uiMiddlewaresWithPagination.HandlerFunc(dh.GetEventDeliveriesPaged))
	r.Method(POST, "/ui/organisations/{orgID}/projects/{projectID}/eventdeliveries/forceresend", uiMiddlewares.HandlerFunc(dh.ForceResendEventDeliveries))
	r.Method(POST, "/ui/organisations/{orgID}/projects/{projectID}/eventdeliveries/batchretry", uiMiddlewares.HandlerFunc(dh.BatchRetryEventDelivery))
	r.Method(GET,
		"/ui/organisations/{orgID}/projects/{projectID}/eventdeliveries/countbatchretryevents",
		uiMiddlewares.HandlerFunc(dh.CountAffectedEventDeliveries))

	r.Method(GET,
		"/ui/organisations/{orgID}/projects/{projectID}/eventdeliveries/{eventDeliveryID}",
		uiMiddlewares.HandlerFunc(dh.GetEventDelivery))

	r.Method(PUT,
		"/ui/organisations/{orgID}/projects/{projectID}/eventdeliveries/{eventDeliveryID}/resend",
		uiMiddlewares.HandlerFunc(dh.ResendEventDelivery))

	r.Method(GET,
		"/ui/organisations/{orgID}/projects/{projectID}/eventdeliveries/{eventDeliveryID}/deliveryattempts",
		uiMiddlewaresWithPagination.HandlerFunc(dh.GetDeliveryAttempts))

	r.Method(GET,
		"/ui/organisations/{orgID}/projects/{projectID}/eventdeliveries/{eventDeliveryID}/deliveryattempts/{deliveryAttemptID}",
		uiMiddlewaresWithPagination.HandlerFunc(dh.GetDeliveryAttempt))

	r.Method(POST,
		"/ui/organisations/{orgID}/projects/{projectID}/subscriptions",
		uiMiddlewares.HandlerFunc(dh.CreateSubscription))

	r.Method(POST,
		"/ui/organisations/{orgID}/projects/{projectID}/subscriptions/test_filter",
		uiMiddlewares.HandlerFunc(dh.TestSubscriptionFilter))

	r.Method(GET,
		"/ui/organisations/{orgID}/projects/{projectID}/subscriptions",
		uiMiddlewaresWithPagination.HandlerFunc(dh.GetSubscriptions))

	r.Method(DELETE,
		"/ui/organisations/{orgID}/projects/{projectID}/subscriptions/{subscriptionID}",
		uiMiddlewares.HandlerFunc(dh.DeleteSubscription))

	r.Method(GET,
		"/ui/organisations/{orgID}/projects/{projectID}/subscriptions/{subscriptionID}",
		uiMiddlewares.HandlerFunc(dh.GetSubscription))

	r.Method(PUT,
		"/ui/organisations/{orgID}/projects/{projectID}/subscriptions/{subscriptionID}",
		uiMiddlewares.HandlerFunc(dh.UpdateSubscription))

	r.Method(POST, "/ui/organisations/{orgID}/projects/{projectID}/sources", uiMiddlewares.HandlerFunc(dh.CreateSource))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/sources/{sourceID}", uiMiddlewares.HandlerFunc(dh.GetSourceByID))
	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/sources", uiMiddlewaresWithPagination.HandlerFunc(dh.LoadSourcesPaged))
	r.Method(PUT, "/ui/organisations/{orgID}/projects/{projectID}/sources/{sourceID}", uiMiddlewares.HandlerFunc(dh.UpdateSource))
	r.Method(DELETE, "/ui/organisations/{orgID}/projects/{projectID}/sources/{sourceID}", uiMiddlewares.HandlerFunc(dh.DeleteSource))

	r.Method(GET, "/ui/organisations/{orgID}/projects/{projectID}/dashboard/summary", uiMiddlewares.HandlerFunc(dh.GetDashboardSummary))

	r.Method(POST,
		"/ui/organisations/{orgID}/projects/{projectID}/portal-links",
		uiMiddlewares.HandlerFunc(dh.CreatePortalLink))

	r.Method(GET,
		"/ui/organisations/{orgID}/projects/{projectID}/portal-links/{portalLinkID}",
		uiMiddlewares.HandlerFunc(dh.GetPortalLinkByID))

	r.Method(GET,
		"/ui/organisations/{orgID}/projects/{projectID}/portal-links",
		uiMiddlewaresWithPagination.HandlerFunc(dh.LoadPortalLinksPaged))

	r.Method(PUT,
		"/ui/organisations/{orgID}/projects/{projectID}/portal-links/{portalLinkID}",
		uiMiddlewares.HandlerFunc(dh.UpdatePortalLink))

	r.Method(PUT,
		"/ui/organisations/{orgID}/projects/{projectID}/portal-links/{portalLinkID}/revoke",
		uiMiddlewares.HandlerFunc(dh.RevokePortalLink))

	r.Method(GET,
		"/ui/configuration", uiMiddlewares.HandlerFunc(dh.LoadConfiguration))

	r.Method(POST,
		"/ui/configuration", uiMiddlewares.HandlerFunc(dh.CreateConfiguration))

	r.Method(PUT,
		"/ui/configuration", uiMiddlewares.HandlerFunc(dh.UpdateConfiguration))
}

var guestRoutes = []string{
	"/auth/login",
	"/auth/register",
	"/auth/token/refresh",
	"/users/token",
	"/users/forgot-password",
	"/users/reset-password",
	"/users/verify_email",
	"/organisations/process_invite",
}

func shouldAuthRoute(r *http.Request) bool {
	for _, route := range guestRoutes {
		if strings.HasSuffix(r.URL.Path, route) {
			return false
		}
	}

	return true
}

func shouldApplyCORS(r *http.Request) bool {
	corsRoutes := []string{"/ui", "/portal-api"}

	for _, route := range corsRoutes {
		if strings.HasPrefix(r.URL.Path, route) {
			return true
		}
	}

	return false
}
