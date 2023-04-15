package dashboard

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

type DashboardHandler struct {
	M      *middleware.Middleware
	A      types.App
	Router http.Handler
}

func NewDashboardHandler(a types.App) *DashboardHandler {
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

	return &DashboardHandler{
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

func (a *DashboardHandler) BuildRoutes() http.Handler {
	router := chi.NewRouter()

	// UI API.
	router.Use(a.M.JsonResponse)
	router.Use(a.M.SetupCORS)
	router.Use(chiMiddleware.Maybe(a.M.RequireAuth(), middleware.ShouldAuthRoute))

	router.Post("/organisations/process_invite", a.ProcessOrganisationMemberInvite)
	router.Get("/users/token", a.FindUserByInviteToken)

	router.Route("/users", func(userRouter chi.Router) {
		userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
			userSubRouter.Get("/profile", a.GetUser)
			userSubRouter.Put("/profile", a.UpdateUser)
			userSubRouter.Put("/password", a.UpdatePassword)

			userSubRouter.Route("/security/personal_api_keys", func(securityRouter chi.Router) {
				securityRouter.Post("/", a.CreatePersonalAPIKey)
				securityRouter.Put("/{keyID}/revoke", a.RevokePersonalAPIKey)
				securityRouter.With(a.M.Pagination).Get("/", a.GetAPIKeys)
			})
		})
	})

	router.Post("/users/forgot-password", a.ForgotPassword)
	router.Post("/users/reset-password", a.ResetPassword)
	router.Post("/users/verify_email", a.VerifyEmail)
	router.Post("/users/resend_verification_email", a.ResendVerificationEmail)

	router.Route("/auth", func(authRouter chi.Router) {
		authRouter.Post("/login", a.LoginUser)
		authRouter.Post("/register", a.RegisterUser)
		authRouter.Post("/token/refresh", a.RefreshToken)
		authRouter.Post("/logout", a.LogoutUser)
	})

	router.Route("/organisations", func(orgRouter chi.Router) {
		orgRouter.Post("/", a.CreateOrganisation)
		orgRouter.With(a.M.Pagination).Get("/", a.GetOrganisationsPaged)

		orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {

			orgSubRouter.Get("/", a.GetOrganisation)
			orgSubRouter.Put("/", a.UpdateOrganisation)
			orgSubRouter.Delete("/", a.DeleteOrganisation)

			orgSubRouter.Route("/invites", func(orgInvitesRouter chi.Router) {
				orgInvitesRouter.Post("/", a.InviteUserToOrganisation)
				orgInvitesRouter.Post("/{inviteID}/resend", a.ResendOrganizationInvite)
				orgInvitesRouter.Post("/{inviteID}/cancel", a.CancelOrganizationInvite)
				orgInvitesRouter.With(a.M.Pagination).Get("/pending", a.GetPendingOrganisationInvites)
			})

			orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
				orgMemberRouter.With(a.M.Pagination).Get("/", a.GetOrganisationMembers)

				orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {
					orgMemberSubRouter.Get("/", a.GetOrganisationMember)
					orgMemberSubRouter.Put("/", a.UpdateOrganisationMember)
					orgMemberSubRouter.Delete("/", a.DeleteOrganisationMember)
				})
			})

			orgSubRouter.Route("/projects", func(projectRouter chi.Router) {
				projectRouter.Route("/", func(orgSubRouter chi.Router) {
					projectRouter.Post("/", a.CreateProject)
					projectRouter.Get("/", a.GetProjects)
				})

				projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
					projectSubRouter.Get("/", a.GetProject)
					projectSubRouter.Get("/stats", a.GetProjectStatistics)
					projectSubRouter.Put("/", a.UpdateProject)
					projectSubRouter.Delete("/", a.DeleteProject)

					projectSubRouter.Route("/security/keys", func(projectKeySubRouter chi.Router) {
						projectKeySubRouter.Put("/regenerate", a.RegenerateProjectAPIKey)
					})

					projectSubRouter.Route("/endpoints", func(endpointSubRouter chi.Router) {
						endpointSubRouter.Post("/", a.CreateEndpoint)
						endpointSubRouter.With(a.M.Pagination).Get("/", a.GetEndpoints)

						endpointSubRouter.Route("/{endpointID}", func(e chi.Router) {
							e.Get("/", a.GetEndpoint)
							e.Put("/", a.UpdateEndpoint)
							e.Delete("/", a.DeleteEndpoint)
							e.Put("/toggle_status", a.ToggleEndpointStatus)
							e.Put("/expire_secret", a.ExpireSecret)

							e.Route("/keys", func(keySubRouter chi.Router) {
								keySubRouter.With(fflag.CanAccessFeature(fflag.Features[fflag.CanCreateCLIAPIKey])).Post("/", a.CreateEndpointAPIKey)
								keySubRouter.With(a.M.Pagination).Get("/", a.LoadEndpointAPIKeysPaged)
								keySubRouter.Put("/{keyID}/revoke", a.RevokeEndpointAPIKey)
							})

							e.Route("/devices", func(deviceRouter chi.Router) {
								deviceRouter.With(a.M.Pagination).Get("/", a.FindDevicesByAppID)
							})
						})
					})

					projectSubRouter.Route("/events", func(eventRouter chi.Router) {

						eventRouter.Post("/", a.CreateEndpointEvent)
						eventRouter.Post("/fanout", a.CreateEndpointFanoutEvent)
						eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)
						eventRouter.Post("/batchreplay", a.BatchReplayEvents)
						eventRouter.Get("/countbatchreplayevents", a.CountAffectedEvents)

						eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
							eventSubRouter.Get("/", a.GetEndpointEvent)
							eventSubRouter.Put("/replay", a.ReplayEndpointEvent)
						})
					})

					projectSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
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

					projectSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {

						subscriptionRouter.Post("/", a.CreateSubscription)
						subscriptionRouter.Post("/test_filter", a.TestSubscriptionFilter)
						subscriptionRouter.With(a.M.Pagination).Get("/", a.GetSubscriptions)
						subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
						subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
						subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
					})

					projectSubRouter.Route("/sources", func(sourceRouter chi.Router) {
						sourceRouter.Post("/", a.CreateSource)
						sourceRouter.Get("/{sourceID}", a.GetSourceByID)
						sourceRouter.With(a.M.Pagination).Get("/", a.LoadSourcesPaged)
						sourceRouter.Put("/{sourceID}", a.UpdateSource)
						sourceRouter.Delete("/{sourceID}", a.DeleteSource)
					})

					projectSubRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
						dashboardRouter.Get("/summary", a.GetDashboardSummary)
					})

					projectSubRouter.Route("/portal-links", func(portalLinkRouter chi.Router) {
						portalLinkRouter.Post("/", a.CreatePortalLink)
						portalLinkRouter.Get("/{portalLinkID}", a.GetPortalLinkByID)
						portalLinkRouter.With(a.M.Pagination).Get("/", a.LoadPortalLinksPaged)
						portalLinkRouter.Put("/{portalLinkID}", a.UpdatePortalLink)
						portalLinkRouter.Put("/{portalLinkID}/revoke", a.RevokePortalLink)
					})
				})
			})
		})
	})

	router.Route("/configuration", func(configRouter chi.Router) {
		configRouter.Get("/", a.LoadConfiguration)
		configRouter.Post("/", a.CreateConfiguration)
		configRouter.Put("/", a.UpdateConfiguration)
	})

	router.Post("/flags", flipt.BatchEvaluate)

	return router

}

func (a *DashboardHandler) GetDashboardSummary(w http.ResponseWriter, r *http.Request) {
	format := "2006-01-02T15:04:05"
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")
	if len(startDate) == 0 {
		_ = render.Render(w, r, util.NewErrorResponse("please specify a startDate query", http.StatusBadRequest))
		return
	}

	startT, err := time.Parse(format, startDate)
	if err != nil {
		a.A.Logger.WithError(err).Error("error parsing startDate")
		_ = render.Render(w, r, util.NewErrorResponse("please specify a startDate in the format "+format, http.StatusBadRequest))
		return
	}

	period := r.URL.Query().Get("type")
	if util.IsStringEmpty(period) {
		_ = render.Render(w, r, util.NewErrorResponse("please specify a type query", http.StatusBadRequest))
		return
	}

	if !datastore.IsValidPeriod(period) {
		_ = render.Render(w, r, util.NewErrorResponse("please specify a type query in (daily, weekly, monthly, yearly)", http.StatusBadRequest))
		return
	}

	var endT time.Time
	if len(endDate) == 0 {
		endT = time.Date(startT.Year(), startT.Month(), startT.Day(), 23, 59, 59, 999999999, startT.Location())
	} else {
		endT, err = time.Parse(format, endDate)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("please specify an endDate in the format "+format+" or none at all", http.StatusBadRequest))
			return
		}
	}

	p := datastore.PeriodValues[period]
	if err := middleware.EnsurePeriod(startT, endT); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("invalid period '%s': %s", period, err.Error()), http.StatusBadRequest))
		return
	}

	searchParams := datastore.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	project, err := a.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	qs := fmt.Sprintf("%v:%v:%v:%v", project.UID, searchParams.CreatedAtStart, searchParams.CreatedAtEnd, period)

	var data *models.DashboardSummary
	err = a.A.Cache.Get(r.Context(), qs, &data)
	if err != nil {
		a.A.Logger.WithError(err)
	}

	if data != nil {
		_ = render.Render(w, r, util.NewServerResponse("Dashboard summary fetched successfully",
			data, http.StatusOK))
		return
	}

	endpointService := createEndpointService(a)
	apps, err := endpointService.CountProjectEndpoints(r.Context(), project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while searching apps", http.StatusInternalServerError))
		return
	}

	eventsSent, messages, err := a.M.ComputeDashboardMessages(r.Context(), project.UID, searchParams, p)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("an error occurred while fetching messages", http.StatusInternalServerError))
		return
	}

	dashboard := models.DashboardSummary{
		Applications: int(apps),
		EventsSent:   eventsSent,
		Period:       period,
		PeriodData:   &messages,
	}

	err = a.A.Cache.Set(r.Context(), qs, dashboard, time.Minute)

	if err != nil {
		a.A.Logger.WithError(err)
	}

	_ = render.Render(w, r, util.NewServerResponse("Dashboard summary fetched successfully",
		dashboard, http.StatusOK))
}

func (a *DashboardHandler) retrieveOrganisation(r *http.Request) (*datastore.Organisation, error) {
	orgID := chi.URLParam(r, "orgID")

	if util.IsStringEmpty(orgID) {
		orgID = r.URL.Query().Get("orgID")
	}

	orgRepo := postgres.NewOrgRepo(a.A.DB)
	return orgRepo.FetchOrganisationByID(r.Context(), orgID)
}

func (a *DashboardHandler) retrieveUser(r *http.Request) (*datastore.User, error) {
	authUser := middleware.GetAuthUserFromContext(r.Context())
	user, ok := authUser.Metadata.(*datastore.User)
	if !ok {
		return &datastore.User{}, errors.New("User not found")
	}

	return user, nil
}

func (a *DashboardHandler) retrieveMembership(r *http.Request) (*datastore.OrganisationMember, error) {
	org, err := a.retrieveOrganisation(r)
	if err != nil {
		return &datastore.OrganisationMember{}, err
	}

	user, err := a.retrieveUser(r)
	if err != nil {
		return &datastore.OrganisationMember{}, err
	}

	orgMemberRepo := postgres.NewOrgMemberRepo(a.A.DB)
	return orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, org.UID)
}

func (a *DashboardHandler) retrieveProject(r *http.Request) (*datastore.Project, error) {
	projectID := chi.URLParam(r, "projectID")

	if util.IsStringEmpty(projectID) {
		return &datastore.Project{}, errors.New("Project ID not present in request")
	}

	projectRepo := postgres.NewProjectRepo(a.A.DB)
	return projectRepo.FetchProjectByID(r.Context(), projectID)
}

func (a *DashboardHandler) retrieveHost() (string, error) {
	cfg, err := config.Get()
	if err != nil {
		return "", err
	}

	return cfg.Host, nil
}
