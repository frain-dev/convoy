package api

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/api/dashboard"
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

type ApplicationHandler struct {
	M      *middleware.Middleware
	Router http.Handler
	A      types.App
}

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

func NewApplicationHandler(a types.App) *ApplicationHandler {
	m := middleware.NewMiddleware(&middleware.CreateMiddleware{
		Cache:             a.Cache,
		Logger:            a.Logger,
		Limiter:           a.Limiter,
		Tracer:            a.Tracer,
		EventRepo:         postgres.NewEventRepo(a.DB),
		EventDeliveryRepo: postgres.NewEventDeliveryRepo(a.DB),
		EndpointRepo:      postgres.NewEndpointRepo(a.DB),
		ProjectRepo:       postgres.NewProjectRepo(a.DB),
		ApiKeyRepo:        postgres.NewAPIKeyRepo(a.DB),
		SubRepo:           postgres.NewSubscriptionRepo(a.DB),
		SourceRepo:        postgres.NewSourceRepo(a.DB),
		OrgRepo:           postgres.NewOrgRepo(a.DB),
		OrgMemberRepo:     postgres.NewOrgMemberRepo(a.DB),
		OrgInviteRepo:     postgres.NewOrgInviteRepo(a.DB),
		UserRepo:          postgres.NewUserRepo(a.DB),
		ConfigRepo:        postgres.NewConfigRepo(a.DB),
		DeviceRepo:        postgres.NewDeviceRepo(a.DB),
		PortalLinkRepo:    postgres.NewPortalLinkRepo(a.DB),
	})

	az, _ := authz.NewAuthz(&authz.AuthzOpts{
		AuthCtxKey: authz.AuthCtxType(middleware.AuthUserCtx),
	})

	ah := &ApplicationHandler{
		M: m,
		A: types.App{
			DB:       a.DB,
			Queue:    a.Queue,
			Cache:    a.Cache,
			Searcher: a.Searcher,
			Logger:   a.Logger,
			Tracer:   a.Tracer,
			Limiter:  a.Limiter,
			Authz:    az,
		},
	}

	return ah
}

func (a *ApplicationHandler) BuildRoutes() http.Handler {
	router := chi.NewRouter()

	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.Recoverer)
	router.Use(a.M.WriteRequestIDHeader)
	router.Use(a.M.InstrumentRequests())
	router.Use(a.M.LogHttpRequest())

	// Ingestion API
	router.Route("/ingest", func(ingestRouter chi.Router) {
		ingestRouter.Get("/{maskID}", a.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", a.IngestEvent)
	})

	publicAPI := &public.PublicHandler{M: a.M, A: a.A}
	router.Mount("/api", publicAPI.BuildRoutes())

	dashboardAPI := &dashboard.DashboardHandler{M: a.M, A: a.A}
	router.Mount("/ui", dashboardAPI.BuildRoutes())

	portalAPI := &portalapi.PortalLinkHandler{M: a.M, A: a.A}
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
	return a.A.RegisterPolicy()
}
