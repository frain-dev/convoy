package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/searcher"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/tracer"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

type Server struct {
	s                         *http.Server
	M                         *middleware.Middleware
	Cache                     cache.Cache
	Queue                     queue.Queuer
	AppService                *services.AppService
	EventService              *services.EventService
	GroupService              *services.GroupService
	SecurityService           *services.SecurityService
	SourceService             *services.SourceService
	ConfigService             *services.ConfigService
	UserService               *services.UserService
	SubService                *services.SubcriptionService
	OrganisationService       *services.OrganisationService
	OrganisationMemberService *services.OrganisationMemberService
	OrganisationInviteService *services.OrganisationInviteService

	// for crc check only
	SourceRepo datastore.SourceRepository
}

type CreateServer struct {
	Cfg               config.Configuration
	EventRepo         datastore.EventRepository
	EventDeliveryRepo datastore.EventDeliveryRepository
	AppRepo           datastore.ApplicationRepository
	GroupRepo         datastore.GroupRepository
	ApiKeyRepo        datastore.APIKeyRepository
	SubRepo           datastore.SubscriptionRepository
	SourceRepo        datastore.SourceRepository
	OrgRepo           datastore.OrganisationRepository
	OrgMemberRepo     datastore.OrganisationMemberRepository
	OrgInviteRepo     datastore.OrganisationInviteRepository
	UserRepo          datastore.UserRepository
	ConfigRepo        datastore.ConfigurationRepository
	Queue             queue.Queuer
	Logger            logger.Logger
	Tracer            tracer.Tracer
	Cache             cache.Cache
	Limiter           limiter.RateLimiter
	Searcher          searcher.Searcher
}

func NewServer(c *CreateServer) *Server {

	as := services.NewAppService(c.AppRepo, c.EventRepo, c.EventDeliveryRepo, c.Cache)
	es := services.NewEventService(c.AppRepo, c.EventRepo, c.EventDeliveryRepo, c.Queue, c.Cache, c.Searcher, c.SubRepo)
	gs := services.NewGroupService(c.ApiKeyRepo, c.AppRepo, c.GroupRepo, c.EventRepo, c.EventDeliveryRepo, c.Limiter, c.Cache)
	ss := services.NewSecurityService(c.GroupRepo, c.ApiKeyRepo)
	os := services.NewOrganisationService(c.OrgRepo, c.OrgMemberRepo)
	rs := services.NewSubscriptionService(c.SubRepo, c.AppRepo, c.SourceRepo)
	sos := services.NewSourceService(c.SourceRepo, c.Cache)
	us := services.NewUserService(c.UserRepo, c.Cache, c.Queue)
	ois := services.NewOrganisationInviteService(c.OrgRepo, c.UserRepo, c.OrgMemberRepo, c.OrgInviteRepo, c.Queue)
	om := services.NewOrganisationMemberService(c.OrgMemberRepo)
	cs := services.NewConfigService(c.ConfigRepo)

	m := middleware.NewMiddleware(&middleware.CreateMiddleware{
		EventRepo:         c.EventRepo,
		EventDeliveryRepo: c.EventDeliveryRepo,
		AppRepo:           c.AppRepo,
		GroupRepo:         c.GroupRepo,
		ApiKeyRepo:        c.ApiKeyRepo,
		SubRepo:           c.SubRepo,
		SourceRepo:        c.SourceRepo,
		OrgRepo:           c.OrgRepo,
		OrgMemberRepo:     c.OrgMemberRepo,
		OrgInviteRepo:     c.OrgInviteRepo,
		UserRepo:          c.UserRepo,
		ConfigRepo:        c.ConfigRepo,
		Cache:             c.Cache,
		Logger:            c.Logger,
		Limiter:           c.Limiter,
		Tracer:            c.Tracer,
	})

	srv := &Server{
		s: &http.Server{
			ReadTimeout:  time.Second * 30,
			WriteTimeout: time.Second * 30,
			Addr:         fmt.Sprintf(":%d", c.Cfg.Server.HTTP.Port),
		},
		M:                         m,
		Queue:                     c.Queue,
		Cache:                     c.Cache,
		AppService:                as,
		EventService:              es,
		GroupService:              gs,
		SecurityService:           ss,
		SourceService:             sos,
		ConfigService:             cs,
		UserService:               us,
		SubService:                rs,
		OrganisationService:       os,
		OrganisationMemberService: om,
		OrganisationInviteService: ois,

		SourceRepo: c.SourceRepo,
	}

	metrics.RegisterQueueMetrics(c.Queue, c.Cfg)
	metrics.RegisterDBMetrics(c.EventDeliveryRepo)
	prometheus.MustRegister(metrics.RequestDuration())
	return srv
}

func (s *Server) SetupRoutes(handler http.Handler) http.Handler {
	s.s.Handler = handler
	return handler
	// s.s.Handler = router
	// return router
}

func (s *Server) Listen() {

	go func() {
		//service connections
		if err := s.s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("failed to listen")
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) ListenAndServeTLS(certFile, keyFile string) {
	go func() {
		//service connections
		if err := s.s.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("failed to listen")
		}
	}()

	s.gracefulShutdown()
}

func (s *Server) gracefulShutdown() {
	//Wait for interrupt signal to gracefully shutdown the server with a timeout of 10 seconds
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	log.Info("Stopping server")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.s.Shutdown(ctx); err != nil {
		log.WithError(err).Fatal("Server Shutdown")
	}

	log.Info("Server exiting")

	time.Sleep(2 * time.Second) // allow all pending connections close themselves
}
