package portalapi

import (
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

type PortalLinkHandler struct {
	Router http.Handler
	A      *types.APIOptions
}

func NewPortalLinkHandler(a *types.APIOptions) *PortalLinkHandler {
	return &PortalLinkHandler{A: a}
}

func (a *PortalLinkHandler) BuildRoutes() http.Handler {
	router := chi.NewRouter()

	// App Portal API.
	router.Use(middleware.JsonResponse)
	router.Use(middleware.SetupCORS)

	router.Route("/endpoints", func(endpointRouter chi.Router) {
		endpointRouter.Get("/", a.GetPortalLinkEndpoints)

		endpointRouter.Route("/{endpointID}", func(endpointSubRouter chi.Router) {
			endpointSubRouter.Get("/", a.GetEndpoint)
		})
	})

	router.Route("/events", func(eventRouter chi.Router) {
		eventRouter.With(middleware.Pagination).Get("/", a.GetEventsPaged)
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
		subscriptionRouter.With(middleware.Pagination).Get("/", a.GetSubscriptions)
		subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
		subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
		subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
	})

	router.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
		eventDeliveryRouter.With(middleware.Pagination).Get("/", a.GetEventDeliveriesPaged)
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
	var org *datastore.Organisation

	project, err := a.retrieveProject(r)
	if err != nil {
		return org, err
	}

	orgRepo := postgres.NewOrgRepo(a.A.DB)
	org, err = orgRepo.FetchOrganisationByID(r.Context(), project.OrganisationID)
	if err != nil {
		return org, err
	}

	return org, nil
}

func (a *PortalLinkHandler) retrieveProject(r *http.Request) (*datastore.Project, error) {
	var project *datastore.Project
	pLink, err := a.retrievePortalLink(r)
	if err != nil {
		return project, err
	}

	projectRepo := postgres.NewProjectRepo(a.A.DB)
	project, err = projectRepo.FetchProjectByID(r.Context(), pLink.ProjectID)
	if err != nil {
		return project, err
	}

	return project, nil
}

func (a *PortalLinkHandler) retrieveHost() (string, error) {
	cfg, err := config.Get()
	if err != nil {
		return "", err
	}

	return cfg.Host, nil
}

func (a *PortalLinkHandler) retrievePortalLink(r *http.Request) (*datastore.PortalLink, error) {
	token := r.URL.Query().Get("token")

	if util.IsStringEmpty(token) {
		cred, _ := middleware.GetAuthFromRequest(r)
		token = cred.Token
	}

	portalLinkRepo := postgres.NewPortalLinkRepo(a.A.DB)
	pLink, err := portalLinkRepo.FindPortalLinkByToken(r.Context(), token)
	if err != nil {
		message := "an error occurred while retrieving portal link"

		if errors.Is(err, datastore.ErrPortalLinkNotFound) {
			message = "invalid token"
		}

		return &datastore.PortalLink{}, errors.New(message)
	}

	return pLink, nil
}

func RequirePortalAccess(a *PortalLinkHandler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			project, err := a.retrieveProject(r)
			if err != nil {
				_ = render.Render(w, r, util.NewServiceErrResponse(err))
				return
			}

			err = a.A.Authz.Authorize(r.Context(), "project.get", project)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
