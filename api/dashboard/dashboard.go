package dashboard

import (
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
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
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(a.M.JsonResponse)
		uiRouter.Use(a.M.SetupCORS)
		uiRouter.Use(chiMiddleware.Maybe(a.M.RequireAuth(), middleware.ShouldAuthRoute))
		uiRouter.Use(a.M.RequireBaseUrl())

		uiRouter.Post("/organisations/process_invite", a.ProcessOrganisationMemberInvite)
		uiRouter.Get("/users/token", a.FindUserByInviteToken)

		uiRouter.Route("/users", func(userRouter chi.Router) {
			userRouter.Use(a.M.RequireAuthUserMetadata())
			userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
				userSubRouter.Use(a.M.RequireAuthorizedUser())
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

		uiRouter.Post("/users/forgot-password", a.ForgotPassword)
		uiRouter.Post("/users/reset-password", a.ResetPassword)
		uiRouter.Post("/users/verify_email", a.VerifyEmail)
		uiRouter.Post("/users/resend_verification_email", a.ResendVerificationEmail)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", a.LoginUser)
			authRouter.Post("/register", a.RegisterUser)
			authRouter.Post("/token/refresh", a.RefreshToken)
			authRouter.Post("/logout", a.LogoutUser)
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(a.M.RequireAuthUserMetadata())
			orgRouter.Use(a.M.RequireBaseUrl())

			orgRouter.Post("/", a.CreateOrganisation)
			orgRouter.With(a.M.Pagination).Get("/", a.GetOrganisationsPaged)

			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Use(a.M.RequireOrganisation())
				orgSubRouter.Use(a.M.RequireOrganisationMembership())

				orgSubRouter.Get("/", a.GetOrganisation)
				orgSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", a.UpdateOrganisation)
				orgSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", a.DeleteOrganisation)

				orgSubRouter.Route("/invites", func(orgInvitesRouter chi.Router) {
					orgInvitesRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", a.InviteUserToOrganisation)
					orgInvitesRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/resend", a.ResendOrganizationInvite)
					orgInvitesRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/cancel", a.CancelOrganizationInvite)
					orgInvitesRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).With(a.M.Pagination).Get("/pending", a.GetPendingOrganisationInvites)
				})

				orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
					orgMemberRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

					orgMemberRouter.With(a.M.Pagination).Get("/", a.GetOrganisationMembers)

					orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {
						orgMemberSubRouter.Get("/", a.GetOrganisationMember)
						orgMemberSubRouter.Put("/", a.UpdateOrganisationMember)
						orgMemberSubRouter.Delete("/", a.DeleteOrganisationMember)
					})
				})

				orgSubRouter.Route("/projects", func(projectRouter chi.Router) {
					projectRouter.Route("/", func(orgSubRouter chi.Router) {
						projectRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", a.CreateProject)
						projectRouter.Get("/", a.GetProjects)
					})

					projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
						projectSubRouter.Use(a.M.RequireProject())
						projectSubRouter.Use(a.M.RateLimitByProjectID())
						projectSubRouter.Use(a.M.RequireOrganisationProjectMember())

						projectSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Get("/", a.GetProject)
						projectSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", a.UpdateProject)
						projectSubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", a.DeleteProject)

						projectSubRouter.Get("/stats", a.GetProjectStatistics)

						projectSubRouter.Route("/security/keys", func(projectKeySubRouter chi.Router) {
							projectKeySubRouter.With(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/regenerate", a.RegenerateProjectAPIKey)
						})

						projectSubRouter.Route("/endpoints", func(endpointSubRouter chi.Router) {
							endpointSubRouter.Post("/", a.CreateEndpoint)
							endpointSubRouter.With(a.M.Pagination).Get("/", a.GetEndpoints)

							endpointSubRouter.Route("/{endpointID}", func(e chi.Router) {
								e.Use(a.M.RequireEndpoint())

								e.Get("/", a.GetEndpoint)
								e.Put("/", a.UpdateEndpoint)
								e.Delete("/", a.DeleteEndpoint)
								e.Put("/toggle_status", a.ToggleEndpointStatus)
								e.Put("/expire_secret", a.ExpireSecret)

								e.Route("/keys", func(keySubRouter chi.Router) {
									keySubRouter.Use(a.M.RequireBaseUrl())
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
							eventRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							eventRouter.Post("/", a.CreateEndpointEvent)
							eventRouter.Post("/fanout", a.CreateEndpointFanoutEvent)
							eventRouter.With(a.M.Pagination).Get("/", a.GetEventsPaged)
							eventRouter.Post("/batchreplay", a.BatchReplayEvents)
							eventRouter.Get("/countbatchreplayevents", a.CountAffectedEvents)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Use(a.M.RequireEvent())
								eventSubRouter.Get("/", a.GetEndpointEvent)
								eventSubRouter.Put("/replay", a.ReplayEndpointEvent)
							})
						})

						projectSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

							eventDeliveryRouter.With(a.M.Pagination).Get("/", a.GetEventDeliveriesPaged)
							eventDeliveryRouter.Post("/forceresend", a.ForceResendEventDeliveries)
							eventDeliveryRouter.Post("/batchretry", a.BatchRetryEventDelivery)
							eventDeliveryRouter.Get("/countbatchretryevents", a.CountAffectedEventDeliveries)

							eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
								eventDeliverySubRouter.Use(a.M.RequireEventDelivery())

								eventDeliverySubRouter.Get("/", a.GetEventDelivery)
								eventDeliverySubRouter.Put("/resend", a.ResendEventDelivery)

								eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
									deliveryRouter.Use(fetchDeliveryAttempts())

									deliveryRouter.Get("/", a.GetDeliveryAttempts)
									deliveryRouter.With(a.M.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", a.GetDeliveryAttempt)
								})
							})
						})

						projectSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
							subscriptionRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							subscriptionRouter.Post("/", a.CreateSubscription)
							subscriptionRouter.Post("/test_filter", a.TestSubscriptionFilter)
							subscriptionRouter.With(a.M.Pagination, a.M.RequireBaseUrl()).Get("/", a.GetSubscriptions)
							subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
							subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
							subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
						})

						projectSubRouter.Route("/sources", func(sourceRouter chi.Router) {
							sourceRouter.Use(a.M.RequireOrganisationMemberRole(auth.RoleAdmin))
							sourceRouter.Use(a.M.RequireBaseUrl())

							sourceRouter.Post("/", a.CreateSource)
							sourceRouter.Get("/{sourceID}", a.GetSourceByID)
							sourceRouter.With(a.M.Pagination).Get("/", a.LoadSourcesPaged)
							sourceRouter.Put("/{sourceID}", a.UpdateSource)
							sourceRouter.Delete("/{sourceID}", a.DeleteSource)
						})

						projectSubRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
							dashboardRouter.Get("/summary", a.GetDashboardSummary)
							dashboardRouter.Get("/config", a.GetAllConfigDetails)
						})

						projectSubRouter.Route("/portal-links", func(portalLinkRouter chi.Router) {
							portalLinkRouter.Use(a.M.RequireBaseUrl())

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

		uiRouter.Route("/configuration", func(configRouter chi.Router) {
			configRouter.Use(a.M.RequireAuthUserMetadata())

			configRouter.Get("/", a.LoadConfiguration)
			configRouter.Post("/", a.CreateConfiguration)
			configRouter.Put("/", a.UpdateConfiguration)
		})

		uiRouter.Post("/flags", flipt.BatchEvaluate)
	})

	return router

}

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

type ViewableConfiguration struct {
	Strategy  datastore.StrategyConfiguration  `json:"strategy"`
	Signature datastore.SignatureConfiguration `json:"signature"`
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
	if err := m.EnsurePeriod(startT, endT); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("invalid period '%s': %s", period, err.Error()), http.StatusBadRequest))
		return
	}

	searchParams := datastore.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	project := m.GetProjectFromContext(r.Context())

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

func (a *DashboardHandler) GetAuthLogin(w http.ResponseWriter, r *http.Request) {
	_ = render.Render(w, r, util.NewServerResponse("Logged in successfully",
		m.GetAuthLoginFromContext(r.Context()), http.StatusOK))
}

func (a *DashboardHandler) GetAllConfigDetails(w http.ResponseWriter, r *http.Request) {
	g := m.GetProjectFromContext(r.Context())

	viewableConfig := ViewableConfiguration{
		Strategy:  *g.Config.Strategy,
		Signature: *g.Config.Signature,
	}

	_ = render.Render(w, r, util.NewServerResponse("Config details fetched successfully",
		viewableConfig, http.StatusOK))
}
