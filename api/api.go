package api

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/render"

	authz "github.com/Subomi/go-authz"
	"github.com/frain-dev/convoy/api/handlers"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/subomi/requestmigrations"
)

//go:embed ui/build
var reactFS embed.FS

func reactRootHandler(rw http.ResponseWriter, req *http.Request) {
	p := req.URL.Path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
		req.URL.Path = p
	}
	p = path.Clean(p)
	f := fs.FS(reactFS)
	static, err := fs.Sub(f, "ui/build")
	if err != nil {
		return
	}
	if _, err := static.Open(strings.TrimLeft(p, "/")); err != nil { // If file not found server index/html from root
		req.URL.Path = "/"
	}
	http.FileServer(http.FS(static)).ServeHTTP(rw, req)
}

const (
	VersionHeader = "X-Convoy-Version"
	serverName    = "apiserver"
)

type ApplicationHandler struct {
	Router http.Handler
	rm     *requestmigrations.RequestMigration
	A      *types.APIOptions
	cfg    config.Configuration
}

func NewApplicationHandler(a *types.APIOptions) (*ApplicationHandler, error) {
	appHandler := &ApplicationHandler{A: a}

	cfg, err := config.Get()
	if err != nil {
		return nil, err
	}

	appHandler.cfg = cfg

	az, err := authz.NewAuthz(&authz.AuthzOpts{
		AuthCtxKey: authz.AuthCtxType(middleware.AuthUserCtx),
	})
	if err != nil {
		return nil, err
	}
	appHandler.A.Authz = az

	opts := &requestmigrations.RequestMigrationOptions{
		VersionHeader:  VersionHeader,
		CurrentVersion: config.DefaultAPIVersion,
		GetUserVersionFunc: func(req *http.Request) (string, error) {
			cfg, err := config.Get()
			if err != nil {
				return "", err
			}

			return cfg.APIVersion, nil
		},
		VersionFormat: requestmigrations.DateFormat,
	}
	rm, err := requestmigrations.NewRequestMigration(opts)
	if err != nil {
		return nil, err
	}

	err = rm.RegisterMigrations(migrations)
	if err != nil {
		return nil, err
	}

	appHandler.rm = rm

	return appHandler, nil
}

func (a *ApplicationHandler) buildRouter() *chi.Mux {
	router := chi.NewMux()

	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.Recoverer)
	router.Use(middleware.WriteRequestIDHeader)
	router.Use(middleware.WriteVersionHeader(VersionHeader, a.cfg.APIVersion))
	router.Use(middleware.InstrumentRequests(serverName, router))
	router.Use(middleware.LogHttpRequest(a.A))
	router.Use(chiMiddleware.Maybe(middleware.SetupCORS, shouldApplyCORS))

	return router
}

func (a *ApplicationHandler) BuildControlPlaneRoutes() *chi.Mux {
	router := a.buildRouter()

	handler := &handlers.Handler{A: a.A, RM: a.rm}

	// TODO(subomi): left this here temporarily till the data plane is stable.
	// Ingestion API.
	router.Route("/ingest", func(ingestRouter chi.Router) {
		ingestRouter.Use(middleware.RateLimiterHandler(a.A.Rate, a.cfg.ApiRateLimit))
		ingestRouter.Get("/{maskID}", a.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", a.IngestEvent)
	})

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {
		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(chiMiddleware.AllowContentType("application/json"))
			r.Use(middleware.JsonResponse)
			r.Use(middleware.RequireAuth())

			r.Route("/projects", func(projectRouter chi.Router) {
				projectRouter.Use(middleware.RateLimiterHandler(a.A.Rate, a.cfg.ApiRateLimit))
				projectRouter.Get("/", handler.GetProjects)
				projectRouter.Post("/", handler.CreateProject)

				projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
					projectSubRouter.Get("/", handler.GetProject)
					projectSubRouter.With(handler.RequireEnabledProject()).Put("/", handler.UpdateProject)
					projectSubRouter.Delete("/", handler.DeleteProject)

					projectSubRouter.Route("/endpoints", func(endpointSubRouter chi.Router) {
						endpointSubRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateEndpoint)
						endpointSubRouter.With(middleware.Pagination).Get("/", handler.GetEndpoints)

						endpointSubRouter.Route("/{endpointID}", func(e chi.Router) {
							e.Get("/", handler.GetEndpoint)

							e.With(handler.RequireEnabledProject()).Use(handler.RequireEnabledProject())

							e.With(handler.RequireEnabledProject()).Put("/", handler.UpdateEndpoint)
							e.With(handler.RequireEnabledProject()).Delete("/", handler.DeleteEndpoint)
							e.With(handler.RequireEnabledProject()).Put("/expire_secret", handler.ExpireSecret)
							e.With(handler.RequireEnabledProject()).Put("/pause", handler.PauseEndpoint)
						})
					})

					// TODO(subomi): left this here temporarily till the data plane is stable.
					projectSubRouter.Route("/events", func(eventRouter chi.Router) {
						eventRouter.Route("/", func(writeEventRouter chi.Router) {
							eventRouter.With(middleware.Pagination).Get("/", handler.GetEventsPaged)
							eventRouter.Get("/countbatchreplayevents", handler.CountAffectedEvents)

							// TODO(all): should the InstrumentPath change?
							eventRouter.With(handler.RequireEnabledProject(), middleware.InstrumentPath(a.A.Licenser)).Post("/", handler.CreateEndpointEvent)
							eventRouter.With(handler.RequireEnabledProject(), middleware.InstrumentPath(a.A.Licenser)).Post("/fanout", handler.CreateEndpointFanoutEvent)
							eventRouter.With(handler.RequireEnabledProject(), middleware.InstrumentPath(a.A.Licenser)).Post("/broadcast", handler.CreateBroadcastEvent)
							eventRouter.With(handler.RequireEnabledProject(), middleware.InstrumentPath(a.A.Licenser)).Post("/dynamic", handler.CreateDynamicEvent)
							eventRouter.With(handler.RequireEnabledProject()).Post("/batchreplay", handler.BatchReplayEvents)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.With(handler.RequireEnabledProject()).Put("/replay", handler.ReplayEndpointEvent)
								eventSubRouter.Get("/", handler.GetEndpointEvent)
							})
						})
					})

					projectSubRouter.Route("/event-types", func(eventTypesRouter chi.Router) {
						eventTypesRouter.Get("/", handler.GetEventTypes)
						eventTypesRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateEventType)
						eventTypesRouter.With(handler.RequireEnabledProject()).Post("/import", handler.ImportOpenApiSpec)
						eventTypesRouter.With(handler.RequireEnabledProject()).Put("/{eventTypeId}", handler.UpdateEventType)
						eventTypesRouter.With(handler.RequireEnabledProject()).Post("/{eventTypeId}/deprecate", handler.DeprecateEventType)
					})

					projectSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
						eventDeliveryRouter.With(middleware.Pagination).Get("/", handler.GetEventDeliveriesPaged)
						eventDeliveryRouter.With(handler.RequireEnabledProject()).Post("/forceresend", handler.ForceResendEventDeliveries)
						eventDeliveryRouter.With(handler.RequireEnabledProject()).Post("/batchretry", handler.BatchRetryEventDelivery)

						eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
							eventDeliverySubRouter.Get("/", handler.GetEventDelivery)
							eventDeliverySubRouter.With(handler.RequireEnabledProject()).Put("/resend", handler.ResendEventDelivery)

							eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
								deliveryRouter.Get("/", handler.GetDeliveryAttempts)
								deliveryRouter.Get("/{deliveryAttemptID}", handler.GetDeliveryAttempt)
							})
						})
					})

					projectSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
						subscriptionRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateSubscription)
						subscriptionRouter.Post("/test_filter", handler.TestSubscriptionFilter)
						subscriptionRouter.Post("/test_function", handler.TestSubscriptionFunction)
						subscriptionRouter.With(middleware.Pagination).Get("/", handler.GetSubscriptions)
						subscriptionRouter.With(handler.RequireEnabledProject()).Delete("/{subscriptionID}", handler.DeleteSubscription)
						subscriptionRouter.Get("/{subscriptionID}", handler.GetSubscription)
						subscriptionRouter.With(handler.RequireEnabledProject()).Put("/{subscriptionID}", handler.UpdateSubscription)
						subscriptionRouter.Put("/{subscriptionID}/toggle_status", handler.ToggleSubscriptionStatus)

						// Filter routes
						subscriptionRouter.Route("/{subscriptionID}/filters", func(filterRouter chi.Router) {
							filterRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateFilter)
							filterRouter.With(handler.RequireEnabledProject()).Post("/bulk", handler.BulkCreateFilters)
							filterRouter.With(handler.RequireEnabledProject()).Post("/bulk_update", handler.BulkUpdateFilters)
							filterRouter.Get("/", handler.GetFilters)
							filterRouter.Get("/{filterID}", handler.GetFilter)
							filterRouter.With(handler.RequireEnabledProject()).Put("/{filterID}", handler.UpdateFilter)
							filterRouter.With(handler.RequireEnabledProject()).Delete("/{filterID}", handler.DeleteFilter)
							filterRouter.With(handler.RequireEnabledProject()).Post("/test/{eventType}", handler.TestFilter)
						})
					})

					projectSubRouter.Route("/sources", func(sourceRouter chi.Router) {
						sourceRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateSource)
						sourceRouter.Get("/{sourceID}", handler.GetSource)
						sourceRouter.With(middleware.Pagination).Get("/", handler.LoadSourcesPaged)
						sourceRouter.Post("/test_function", handler.TestSourceFunction)
						sourceRouter.With(handler.RequireEnabledProject()).Put("/{sourceID}", handler.UpdateSource)
						sourceRouter.With(handler.RequireEnabledProject()).Delete("/{sourceID}", handler.DeleteSource)
					})

					projectSubRouter.Route("/portal-links", func(portalLinkRouter chi.Router) {
						portalLinkRouter.Use(middleware.RequireValidPortalLinksLicense(handler.A.Licenser))
						portalLinkRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreatePortalLink)
						portalLinkRouter.Get("/{portalLinkID}", handler.GetPortalLink)
						portalLinkRouter.Get("/{portalLinkID}/refresh_token", handler.RefreshPortalLinkAuthToken)
						portalLinkRouter.With(middleware.Pagination).Get("/", handler.LoadPortalLinksPaged)
						portalLinkRouter.With(handler.RequireEnabledProject()).Put("/{portalLinkID}", handler.UpdatePortalLink)
						portalLinkRouter.With(handler.RequireEnabledProject()).Put("/{portalLinkID}/revoke", handler.RevokePortalLink)
					})

					projectSubRouter.Route("/meta-events", func(metaEventRouter chi.Router) {
						metaEventRouter.With(middleware.Pagination).Get("/", handler.GetMetaEventsPaged)

						metaEventRouter.Route("/{metaEventID}", func(metaEventSubRouter chi.Router) {
							metaEventSubRouter.Get("/", handler.GetMetaEvent)
							metaEventSubRouter.With(handler.RequireEnabledProject()).Put("/resend", handler.ResendMetaEvent)
						})
					})
				})
			})
		})
	})

	// Dashboard API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(middleware.JsonResponse)
		uiRouter.Use(chiMiddleware.Maybe(middleware.RequireAuth(), shouldAuthRoute))

		uiRouter.Get("/license/features", handler.GetLicenseFeatures)

		uiRouter.Post("/users/forgot-password", handler.ForgotPassword)
		uiRouter.Post("/users/reset-password", handler.ResetPassword)
		uiRouter.Post("/users/verify_email", handler.VerifyEmail)
		uiRouter.Post("/users/resend_verification_email", handler.ResendVerificationEmail)
		uiRouter.Post("/organisations/process_invite", handler.ProcessOrganisationMemberInvite)
		uiRouter.Get("/users/token", handler.FindUserByInviteToken)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.With(middleware.RequireValidEnterpriseSSOLicense(handler.A.Licenser)).Get("/sso", handler.InitSSO)
			authRouter.Post("/login", handler.LoginUser)
			authRouter.Post("/register", handler.RegisterUser)
			authRouter.Post("/token/refresh", handler.RefreshToken)
			authRouter.Post("/logout", handler.LogoutUser)
		})

		uiRouter.Route("/saml", func(samlRouter chi.Router) {
			samlRouter.Use(middleware.RequireValidEnterpriseSSOLicense(handler.A.Licenser))
			samlRouter.Get("/login", handler.RedeemLoginSSOToken)
			samlRouter.Get("/register", handler.RedeemRegisterSSOToken)
		})

		uiRouter.Route("/users", func(userRouter chi.Router) {
			userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
				userSubRouter.Get("/profile", handler.GetUser)
				userSubRouter.Put("/profile", handler.UpdateUser)
				userSubRouter.Put("/password", handler.UpdatePassword)

				userSubRouter.Route("/security", func(securityRouter chi.Router) {
					securityRouter.Post("/personal_api_keys", handler.CreatePersonalAPIKey)
					securityRouter.With(middleware.Pagination).Get("/", handler.GetAPIKeys)
					securityRouter.Put("/{keyID}/revoke", handler.RevokePersonalAPIKey)
				})
			})
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Post("/", handler.CreateOrganisation)
			orgRouter.With(middleware.Pagination).Get("/", handler.GetOrganisationsPaged)

			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Get("/", handler.GetOrganisation)
				orgSubRouter.Put("/", handler.UpdateOrganisation)
				orgSubRouter.Delete("/", handler.DeleteOrganisation)

				orgSubRouter.Route("/invites", func(orgInvitesRouter chi.Router) {
					orgInvitesRouter.Post("/", handler.InviteUserToOrganisation)
					orgInvitesRouter.Post("/{inviteID}/resend", handler.ResendOrganizationInvite)
					orgInvitesRouter.Post("/{inviteID}/cancel", handler.CancelOrganizationInvite)
					orgInvitesRouter.With(middleware.Pagination).Get("/pending", handler.GetPendingOrganisationInvites)
				})

				orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
					orgMemberRouter.With(middleware.Pagination).Get("/", handler.GetOrganisationMembers)
					orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {
						orgMemberSubRouter.Get("/", handler.GetOrganisationMember)
						orgMemberSubRouter.Put("/", handler.UpdateOrganisationMember)
						orgMemberSubRouter.Delete("/", handler.DeleteOrganisationMember)
					})
				})

				orgSubRouter.Route("/projects", func(projectRouter chi.Router) {
					projectRouter.Get("/", handler.GetProjects)
					projectRouter.Post("/", handler.CreateProject)

					projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
						projectSubRouter.Get("/", handler.GetProject)
						projectSubRouter.With(handler.RequireEnabledProject()).Put("/", handler.UpdateProject)
						projectSubRouter.With(handler.RequireEnabledProject()).Delete("/", handler.DeleteProject)
						projectSubRouter.Get("/stats", handler.GetProjectStatistics)

						projectSubRouter.Route("/security/keys", func(projectKeySubRouter chi.Router) {
							projectKeySubRouter.With(handler.RequireEnabledProject()).Put("/regenerate", handler.RegenerateProjectAPIKey)
						})

						projectSubRouter.Route("/endpoints", func(endpointSubRouter chi.Router) {
							endpointSubRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateEndpoint)
							endpointSubRouter.With(middleware.Pagination).Get("/", handler.GetEndpoints)

							endpointSubRouter.Route("/{endpointID}", func(e chi.Router) {
								e.Get("/", handler.GetEndpoint)

								e.With(handler.RequireEnabledProject()).Use(handler.RequireEnabledProject())

								e.With(handler.RequireEnabledProject()).Put("/", handler.UpdateEndpoint)
								e.With(handler.RequireEnabledProject()).Delete("/", handler.DeleteEndpoint)
								e.With(handler.RequireEnabledProject()).Put("/expire_secret", handler.ExpireSecret)
								e.With(handler.RequireEnabledProject()).Put("/pause", handler.PauseEndpoint)
								e.With(handler.RequireEnabledProject()).Post("/activate", handler.ActivateEndpoint)
							})
						})

						// TODO(subomi): left this here temporarily till the data plane is stable.
						projectSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.With(middleware.Pagination).Get("/", handler.GetEventsPaged)
							eventRouter.Get("/countbatchreplayevents", handler.CountAffectedEvents)

							// TODO(all): should the InstrumentPath change?
							eventRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateEndpointEvent)
							eventRouter.With(handler.RequireEnabledProject()).Post("/fanout", handler.CreateEndpointFanoutEvent)
							eventRouter.With(handler.RequireEnabledProject()).Post("/broadcast", handler.CreateBroadcastEvent)
							eventRouter.With(handler.RequireEnabledProject()).Post("/dynamic", handler.CreateDynamicEvent)
							eventRouter.With(handler.RequireEnabledProject()).Post("/batchreplay", handler.BatchReplayEvents)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.With(handler.RequireEnabledProject()).Put("/replay", handler.ReplayEndpointEvent)
								eventSubRouter.Get("/", handler.GetEndpointEvent)
							})
						})

						projectSubRouter.Route("/event-types", func(eventTypesRouter chi.Router) {
							eventTypesRouter.Get("/", handler.GetEventTypes)
							eventTypesRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateEventType)
							eventTypesRouter.With(handler.RequireEnabledProject()).Post("/import", handler.ImportOpenApiSpec)
							eventTypesRouter.With(handler.RequireEnabledProject()).Put("/{eventTypeId}", handler.UpdateEventType)
							eventTypesRouter.With(handler.RequireEnabledProject()).Post("/{eventTypeId}/deprecate", handler.DeprecateEventType)
						})

						projectSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.With(middleware.Pagination).Get("/", handler.GetEventDeliveriesPaged)
							eventDeliveryRouter.With(handler.RequireEnabledProject()).Post("/forceresend", handler.ForceResendEventDeliveries)
							eventDeliveryRouter.With(handler.RequireEnabledProject()).Post("/batchretry", handler.BatchRetryEventDelivery)
							eventDeliveryRouter.Get("/countbatchretryevents", handler.CountAffectedEventDeliveries)

							eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
								eventDeliverySubRouter.Get("/", handler.GetEventDelivery)
								eventDeliverySubRouter.With(handler.RequireEnabledProject()).Put("/resend", handler.ResendEventDelivery)

								eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
									deliveryRouter.Get("/", handler.GetDeliveryAttempts)
									deliveryRouter.Get("/{deliveryAttemptID}", handler.GetDeliveryAttempt)
								})
							})
						})

						projectSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
							subscriptionRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateSubscription)
							subscriptionRouter.Post("/test_filter", handler.TestSubscriptionFilter)
							subscriptionRouter.Post("/test_function", handler.TestSubscriptionFunction)
							subscriptionRouter.With(middleware.Pagination).Get("/", handler.GetSubscriptions)
							subscriptionRouter.With(handler.RequireEnabledProject()).Delete("/{subscriptionID}", handler.DeleteSubscription)
							subscriptionRouter.Get("/{subscriptionID}", handler.GetSubscription)
							subscriptionRouter.With(handler.RequireEnabledProject()).Put("/{subscriptionID}", handler.UpdateSubscription)

							// Filter routes
							subscriptionRouter.Route("/{subscriptionID}/filters", func(filterRouter chi.Router) {
								filterRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateFilter)
								filterRouter.With(handler.RequireEnabledProject()).Post("/bulk", handler.BulkCreateFilters)
								filterRouter.With(handler.RequireEnabledProject()).Post("/bulk_update", handler.BulkUpdateFilters)
								filterRouter.Get("/", handler.GetFilters)
								filterRouter.Get("/{filterID}", handler.GetFilter)
								filterRouter.With(handler.RequireEnabledProject()).Put("/{filterID}", handler.UpdateFilter)
								filterRouter.With(handler.RequireEnabledProject()).Delete("/{filterID}", handler.DeleteFilter)
								filterRouter.With(handler.RequireEnabledProject()).Post("/test/{eventType}", handler.TestFilter)
							})
						})

						projectSubRouter.Route("/sources", func(sourceRouter chi.Router) {
							sourceRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateSource)
							sourceRouter.Get("/{sourceID}", handler.GetSource)
							sourceRouter.With(middleware.Pagination).Get("/", handler.LoadSourcesPaged)
							sourceRouter.Post("/test_function", handler.TestSourceFunction)
							sourceRouter.With(handler.RequireEnabledProject()).Put("/{sourceID}", handler.UpdateSource)
							sourceRouter.With(handler.RequireEnabledProject()).Delete("/{sourceID}", handler.DeleteSource)
						})

						projectSubRouter.Route("/meta-events", func(metaEventRouter chi.Router) {
							metaEventRouter.With(middleware.Pagination).Get("/", handler.GetMetaEventsPaged)

							metaEventRouter.Route("/{metaEventID}", func(metaEventSubRouter chi.Router) {
								metaEventSubRouter.Get("/", handler.GetMetaEvent)
								metaEventSubRouter.With(handler.RequireEnabledProject()).Put("/resend", handler.ResendMetaEvent)
							})
						})

						projectSubRouter.Route("/portal-links", func(portalLinkRouter chi.Router) {
							portalLinkRouter.Use(middleware.RequireValidPortalLinksLicense(handler.A.Licenser))
							portalLinkRouter.Post("/", handler.CreatePortalLink)
							portalLinkRouter.Get("/{portalLinkID}", handler.GetPortalLink)
							portalLinkRouter.With(middleware.Pagination).Get("/", handler.LoadPortalLinksPaged)
							portalLinkRouter.Put("/{portalLinkID}", handler.UpdatePortalLink)
							portalLinkRouter.Put("/{portalLinkID}/revoke", handler.RevokePortalLink)
						})

						projectSubRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
							dashboardRouter.Get("/summary", handler.GetDashboardSummary)
						})
					})
				})
			})
		})

		uiRouter.Route("/configuration", func(configRouter chi.Router) {
			configRouter.Get("/", handler.GetConfiguration)
			configRouter.Get("/is_signup_enabled", handler.IsSignUpEnabled)
		})
	})

	// Portal Link API.
	router.Route("/portal-api", func(portalLinkRouter chi.Router) {
		portalLinkRouter.Use(middleware.JsonResponse)
		portalLinkRouter.Use(middleware.SetupCORS)
		portalLinkRouter.Use(middleware.RequireValidPortalLinksLicense(handler.A.Licenser))
		portalLinkRouter.Use(middleware.RequireAuth())

		portalLinkRouter.Get("/portal_link", handler.GetPortalLink)

		portalLinkRouter.Get("/license/features", handler.GetLicenseFeatures)

		portalLinkRouter.Route("/endpoints", func(endpointRouter chi.Router) {
			endpointRouter.With(middleware.Pagination).Get("/", handler.GetEndpoints)
			endpointRouter.Get("/{endpointID}", handler.GetEndpoint)
			endpointRouter.With(handler.CanManageEndpoint()).Post("/", handler.CreateEndpoint)
			endpointRouter.With(handler.CanManageEndpoint()).Put("/{endpointID}", handler.UpdateEndpoint)
			endpointRouter.With(handler.CanManageEndpoint()).Delete("/{endpointID}", handler.DeleteEndpoint)
			endpointRouter.With(handler.CanManageEndpoint()).Put("/{endpointID}/pause", handler.PauseEndpoint)
			endpointRouter.With(handler.CanManageEndpoint()).Put("/{endpointID}/expire_secret", handler.ExpireSecret)
		})

		// TODO(subomi): left this here temporarily till the data plane is stable.
		portalLinkRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Post("/", handler.CreateEndpointEvent)
			eventRouter.With(middleware.Pagination).Get("/", handler.GetEventsPaged)
			eventRouter.Post("/batchreplay", handler.BatchReplayEvents)
			eventRouter.Get("/countbatchreplayevents", handler.CountAffectedEvents)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Get("/", handler.GetEndpointEvent)
				eventSubRouter.Put("/replay", handler.ReplayEndpointEvent)
			})
		})

		portalLinkRouter.Route("/event-types", func(eventTypesRouter chi.Router) {
			eventTypesRouter.Get("/", handler.GetEventTypes)
			eventTypesRouter.With(handler.RequireEnabledProject()).Post("/", handler.CreateEventType)
			eventTypesRouter.With(handler.RequireEnabledProject()).Put("/{eventTypeId}", handler.UpdateEventType)
			eventTypesRouter.With(handler.RequireEnabledProject()).Post("/{eventTypeId}/deprecate", handler.DeprecateEventType)
		})

		portalLinkRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.With(middleware.Pagination).Get("/", handler.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/forceresend", handler.ForceResendEventDeliveries)
			eventDeliveryRouter.Post("/batchretry", handler.BatchRetryEventDelivery)
			eventDeliveryRouter.Get("/countbatchretryevents", handler.CountAffectedEventDeliveries)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Get("/", handler.GetEventDelivery)
				eventDeliverySubRouter.Put("/resend", handler.ResendEventDelivery)

				eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Get("/", handler.GetDeliveryAttempts)
					deliveryRouter.Get("/{deliveryAttemptID}", handler.GetDeliveryAttempt)
				})
			})
		})

		portalLinkRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
			dashboardRouter.Get("/summary", handler.GetDashboardSummary)
		})

		portalLinkRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
			subscriptionRouter.Post("/", handler.CreateSubscription)
			subscriptionRouter.Post("/test_filter", handler.TestSubscriptionFilter)
			subscriptionRouter.Post("/test_function", handler.TestSubscriptionFunction)
			subscriptionRouter.With(middleware.Pagination).Get("/", handler.GetSubscriptions)
			subscriptionRouter.Delete("/{subscriptionID}", handler.DeleteSubscription)
			subscriptionRouter.Get("/{subscriptionID}", handler.GetSubscription)
			subscriptionRouter.Put("/{subscriptionID}", handler.UpdateSubscription)

			// Filter routes
			subscriptionRouter.Route("/{subscriptionID}/filters", func(filterRouter chi.Router) {
				filterRouter.Post("/", handler.CreateFilter)
				filterRouter.With(handler.RequireEnabledProject()).Post("/bulk", handler.BulkCreateFilters)
				filterRouter.With(handler.RequireEnabledProject()).Post("/bulk_update", handler.BulkUpdateFilters)
				filterRouter.Get("/", handler.GetFilters)
				filterRouter.Get("/{filterID}", handler.GetFilter)
				filterRouter.With(handler.RequireEnabledProject()).Put("/{filterID}", handler.UpdateFilter)
				filterRouter.With(handler.RequireEnabledProject()).Delete("/{filterID}", handler.DeleteFilter)
				filterRouter.With(handler.RequireEnabledProject()).Post("/test/{eventType}", handler.TestFilter)
			})
		})
	})

	if a.A.Licenser.AsynqMonitoring() {
		router.Route("/queue", func(asynqRouter chi.Router) {
			asynqRouter.Use(middleware.RequireAuth())
			asynqRouter.Handle("/monitoring/*", a.A.Queue.(*redisqueue.RedisQueue).Monitor())
		})
	}

	if a.A.Licenser.CanExportPrometheusMetrics() {
		router.HandleFunc("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{Registry: metrics.Reg()}).ServeHTTP)
	}

	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("Convoy %v", convoy.GetVersion()), nil, http.StatusOK))
	})

	router.HandleFunc("/*", reactRootHandler)

	a.Router = router

	return router
}

func (a *ApplicationHandler) BuildDataPlaneRoutes() *chi.Mux {
	router := a.buildRouter()

	if a.A.Licenser.CanExportPrometheusMetrics() {
		router.HandleFunc("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{Registry: metrics.Reg()}).ServeHTTP)
	}

	router.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("Convoy %v", convoy.GetVersion()), nil, http.StatusOK))
	})

	// Ingestion API.
	router.Route("/ingest", func(ingestRouter chi.Router) {
		ingestRouter.Use(middleware.RateLimiterHandler(a.A.Rate, a.cfg.ApiRateLimit))
		ingestRouter.Get("/{maskID}", a.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", a.IngestEvent)
	})

	handler := &handlers.Handler{A: a.A, RM: a.rm}

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {
		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(chiMiddleware.AllowContentType("application/json"))
			r.Use(middleware.JsonResponse)
			r.Use(middleware.RequireAuth())

			r.Route("/projects", func(projectRouter chi.Router) {
				projectRouter.Use(middleware.RateLimiterHandler(a.A.Rate, a.cfg.ApiRateLimit))
				projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
					projectSubRouter.Route("/events", func(eventRouter chi.Router) {
						eventRouter.With(middleware.InstrumentPath(a.A.Licenser)).Post("/", handler.CreateEndpointEvent)
						eventRouter.With(middleware.InstrumentPath(a.A.Licenser)).Post("/fanout", handler.CreateEndpointFanoutEvent)
						eventRouter.With(middleware.InstrumentPath(a.A.Licenser)).Post("/broadcast", handler.CreateBroadcastEvent)
						eventRouter.With(middleware.InstrumentPath(a.A.Licenser)).Post("/dynamic", handler.CreateDynamicEvent)
						eventRouter.With(middleware.Pagination).Get("/", handler.GetEventsPaged)
						eventRouter.Post("/batchreplay", handler.BatchReplayEvents)

						eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
							eventSubRouter.Get("/", handler.GetEndpointEvent)
							eventSubRouter.Put("/replay", handler.ReplayEndpointEvent)
						})
					})

					projectSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
						eventDeliveryRouter.With(middleware.Pagination).Get("/", handler.GetEventDeliveriesPaged)
						eventDeliveryRouter.Post("/forceresend", handler.ForceResendEventDeliveries)
						eventDeliveryRouter.Post("/batchretry", handler.BatchRetryEventDelivery)

						eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
							eventDeliverySubRouter.Get("/", handler.GetEventDelivery)
							eventDeliverySubRouter.Put("/resend", handler.ResendEventDelivery)

							eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
								deliveryRouter.Get("/", handler.GetDeliveryAttempts)
								deliveryRouter.Get("/{deliveryAttemptID}", handler.GetDeliveryAttempt)
							})
						})
					})
				})
			})
		})
	})

	// Dashboard API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(middleware.JsonResponse)
		uiRouter.Use(chiMiddleware.Maybe(middleware.RequireAuth(), shouldAuthRoute))

		// TODO(subomi): added these back for the tests to pass.
		// What should we do in the future?
		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.With(middleware.RequireValidEnterpriseSSOLicense(handler.A.Licenser)).Get("/sso", handler.InitSSO)
			authRouter.Post("/login", handler.LoginUser)
			authRouter.Post("/register", handler.RegisterUser)
			authRouter.Post("/token/refresh", handler.RefreshToken)
			authRouter.Post("/logout", handler.LogoutUser)
		})

		uiRouter.Route("/saml", func(samlRouter chi.Router) {
			samlRouter.Use(middleware.RequireValidEnterpriseSSOLicense(handler.A.Licenser))
			samlRouter.Get("/login", handler.RedeemLoginSSOToken)
			samlRouter.Get("/register", handler.RedeemRegisterSSOToken)
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Route("/projects", func(projectRouter chi.Router) {
					projectRouter.Route("/{projectID}", func(projectSubRouter chi.Router) {
						projectSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.Post("/", handler.CreateEndpointEvent)
							eventRouter.Post("/fanout", handler.CreateEndpointFanoutEvent)
							eventRouter.With(middleware.Pagination).Get("/", handler.GetEventsPaged)
							eventRouter.Post("/batchreplay", handler.BatchReplayEvents)
							eventRouter.Get("/countbatchreplayevents", handler.CountAffectedEvents)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Get("/", handler.GetEndpointEvent)
								eventSubRouter.Put("/replay", handler.ReplayEndpointEvent)
							})
						})

						projectSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.With(middleware.Pagination).Get("/", handler.GetEventDeliveriesPaged)
							eventDeliveryRouter.Post("/forceresend", handler.ForceResendEventDeliveries)
							eventDeliveryRouter.Post("/batchretry", handler.BatchRetryEventDelivery)
							eventDeliveryRouter.Get("/countbatchretryevents", handler.CountAffectedEventDeliveries)

							eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
								eventDeliverySubRouter.Get("/", handler.GetEventDelivery)
								eventDeliverySubRouter.Put("/resend", handler.ResendEventDelivery)

								eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
									deliveryRouter.Get("/", handler.GetDeliveryAttempts)
									deliveryRouter.Get("/{deliveryAttemptID}", handler.GetDeliveryAttempt)
								})
							})
						})
					})
				})
			})
		})
	})

	// Portal Link API.
	router.Route("/portal-api", func(portalLinkRouter chi.Router) {
		portalLinkRouter.Use(middleware.JsonResponse)
		portalLinkRouter.Use(middleware.SetupCORS)
		portalLinkRouter.Use(middleware.RequireValidPortalLinksLicense(handler.A.Licenser))
		portalLinkRouter.Use(middleware.RequireAuth())

		portalLinkRouter.Get("/license/features", handler.GetLicenseFeatures)

		portalLinkRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Post("/", handler.CreateEndpointEvent)
			eventRouter.With(middleware.Pagination).Get("/", handler.GetEventsPaged)
			eventRouter.Post("/batchreplay", handler.BatchReplayEvents)
			eventRouter.Get("/countbatchreplayevents", handler.CountAffectedEvents)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Get("/", handler.GetEndpointEvent)
				eventSubRouter.Put("/replay", handler.ReplayEndpointEvent)
			})
		})

		portalLinkRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.With(middleware.Pagination).Get("/", handler.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/forceresend", handler.ForceResendEventDeliveries)
			eventDeliveryRouter.Post("/batchretry", handler.BatchRetryEventDelivery)
			eventDeliveryRouter.Get("/countbatchretryevents", handler.CountAffectedEventDeliveries)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Get("/", handler.GetEventDelivery)
				eventDeliverySubRouter.Put("/resend", handler.ResendEventDelivery)

				eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Get("/", handler.GetDeliveryAttempts)
					deliveryRouter.Get("/{deliveryAttemptID}", handler.GetDeliveryAttempt)
				})
			})
		})
	})

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

		po.SetRule(string(policies.PermissionManageAll), authz.RuleFunc(po.ManageAll))
		po.SetRule(string(policies.PermissionManage), authz.RuleFunc(po.Manage))
		po.SetRule(string(policies.PermissionAdd), authz.RuleFunc(po.Add))

		return po
	}())

	if err != nil {
		return err
	}

	err = a.A.Authz.RegisterPolicy(func() authz.Policy {
		po := &policies.ProjectPolicy{
			BasePolicy:             authz.NewBasePolicy(),
			Licenser:               a.A.Licenser,
			OrganisationRepo:       postgres.NewOrgRepo(a.A.DB),
			OrganisationMemberRepo: postgres.NewOrgMemberRepo(a.A.DB),
		}

		po.SetRule(string(policies.PermissionManage), authz.RuleFunc(po.Manage))
		po.SetRule(string(policies.PermissionView), authz.RuleFunc(po.View))

		return po
	}())

	return err
}

var guestRoutes = []string{
	"/auth/sso",
	"/saml/login",
	"/saml/register",
	"/auth/login",
	"/auth/register",
	"/auth/token/refresh",
	"/users/token",
	"/users/forgot-password",
	"/users/reset-password",
	"/users/verify_email",
	"/organisations/process_invite",
	"/ui/configuration/is_signup_enabled",
	"/ui/license/features",
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
