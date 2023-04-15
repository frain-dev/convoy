package portalapi

import (
	"net/http"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/go-chi/chi/v5"
)

type PortalLinkHandler struct {
	M      *middleware.Middleware
	Router http.Handler
	A      types.App
}

func NewPortalLinkHandler(a types.App) *PortalLinkHandler {
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

	return &PortalLinkHandler{
		M: m,
		A: types.App{
			DB:       a.DB,
			Queue:    a.Queue,
			Cache:    a.Cache,
			Searcher: a.Searcher,
			Logger:   a.Logger,
			Tracer:   a.Tracer,
			Limiter:  a.Limiter,
		},
	}
}

func (a *PortalLinkHandler) BuildRoutes() http.Handler {
	router := chi.NewRouter()

	// App Portal API.
	router.Use(a.M.JsonResponse)
	router.Use(a.M.SetupCORS)

	router.Route("/endpoints", func(endpointRouter chi.Router) {
		endpointRouter.Get("/", a.GetPortalLinkEndpoints)

		endpointRouter.Route("/{endpointID}", func(endpointSubRouter chi.Router) {
			endpointSubRouter.Get("/", a.GetEndpoint)
			endpointSubRouter.Put("/", a.UpdateEndpoint)
		})
	})

	router.Route("/devices", func(deviceRouter chi.Router) {
		deviceRouter.With(a.M.Pagination).Get("/", a.GetPortalLinkDevices)
	})

	router.Route("/keys", func(keySubRouter chi.Router) {
		keySubRouter.With(a.M.Pagination).Get("/", a.GetPortalLinkKeys)
	})

	router.Route("/events", func(eventRouter chi.Router) {
		eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)
		eventRouter.Post("/batchreplay", a.BatchReplayEvents)
		eventRouter.Get("/countbatchreplayevents", a.CountAffectedEvents)

		eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
			eventSubRouter.Get("/", a.GetEndpointEvent)
			eventSubRouter.Put("/replay", a.ReplayEndpointEvent)
		})
	})

	router.Route("/subscriptions", func(subscriptionRouter chi.Router) {
		subscriptionRouter.Post("/", a.CreateSubscription)
		subscriptionRouter.Post("/test_filter", a.TestSubscriptionFilter)
		subscriptionRouter.With(a.M.Pagination).Get("/", a.GetSubscriptions)
		subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
		subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
		subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
	})

	router.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
		eventDeliveryRouter.With(a.M.Pagination).Get("/", a.GetEventDeliveriesPaged)
		eventDeliveryRouter.Post("/forceresend", a.ForceResendEventDeliveries)
		eventDeliveryRouter.Post("/batchretry", a.BatchRetryEventDelivery)
		eventDeliveryRouter.Get("/countbatchretryevents", a.CountAffectedEventDeliveries)

		eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
			eventDeliverySubRouter.Get("/", a.GetEventDelivery)
			eventDeliverySubRouter.Put("/resend", a.ResendEventDelivery)

			eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
				deliveryRouter.Get("/", a.GetDeliveryAttempts)
				deliveryRouter.Get("/{deliveryAttemptID}", a.GetDeliveryAttempt)
			})
		})
	})

	router.Get("/project", a.GetProject)
	router.Post("/flags", flipt.BatchEvaluate)

	return router
}

func (a *PortalLinkHandler) retrieveOrganisation(r *http.Request) (*datastore.Organisation, error) {
	return &datastore.Organisation{}, nil
}

func (a *PortalLinkHandler) retrieveProject(r *http.Request) (*datastore.Project, error) {
	return &datastore.Project{}, nil
}

func (a *PortalLinkHandler) retrieveHost() (string, error) {
	cfg, err := config.Get()
	if err != nil {
		return "", err
	}

	return cfg.Host, nil
}

func (a *PortalLinkHandler) retrievePortalLink(r *http.Request) (*datastore.PortalLink, error) {
	return &datastore.PortalLink{}, nil
}

func (a *PortalLinkHandler) retrieveEndpointIDs(r *http.Request) []string {
	return []string{""}
}
