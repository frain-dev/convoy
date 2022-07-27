package server

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	se "github.com/frain-dev/convoy/internal/pkg/server"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

type ApplicationHandler struct {
	s *se.Server
}

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
		log.WithError(err).Error("an error has occurred with the react app")
		return
	}
	if _, err := static.Open(strings.TrimLeft(p, "/")); err != nil { // If file not found server index/html from root
		req.URL.Path = "/"
	}
	http.FileServer(http.FS(static)).ServeHTTP(rw, req)
}

func BuildRoutes(serv *se.Server) http.Handler {
	router := chi.NewRouter()

	a := &ApplicationHandler{s: serv}

	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.Recoverer)
	router.Use(serv.M.WriteRequestIDHeader)
	router.Use(serv.M.InstrumentRequests())
	router.Use(serv.M.LogHttpRequest())

	// Ingestion API
	router.Route("/ingest", func(ingestRouter chi.Router) {
		ingestRouter.Get("/{maskID}", a.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", a.IngestEvent)
	})

	// Public API.
	router.Route("/api", func(v1Router chi.Router) {

		v1Router.Route("/v1", func(r chi.Router) {
			r.Use(chiMiddleware.AllowContentType("application/json"))
			r.Use(serv.M.JsonResponse)
			r.Use(serv.M.RequireAuth())

			r.Route("/applications", func(appRouter chi.Router) {
				appRouter.Use(serv.M.RequireGroup())
				appRouter.Use(serv.M.RateLimitByGroupID())
				appRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))

				appRouter.Route("/", func(appSubRouter chi.Router) {
					appSubRouter.Post("/", a.CreateApp)
					appRouter.With(serv.M.Pagination).Get("/", a.GetApps)
				})

				appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
					appSubRouter.Use(serv.M.RequireApp())

					appSubRouter.Get("/", a.GetApp)
					appSubRouter.Put("/", a.UpdateApp)
					appSubRouter.Delete("/", a.DeleteApp)

					appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
						endpointAppSubRouter.Post("/", a.CreateAppEndpoint)
						endpointAppSubRouter.Get("/", a.GetAppEndpoints)

						endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
							e.Use(serv.M.RequireAppEndpoint())

							e.Get("/", a.GetAppEndpoint)
							e.Put("/", a.UpdateAppEndpoint)
							e.Delete("/", a.DeleteAppEndpoint)
						})
					})
				})
			})

			r.Route("/events", func(eventRouter chi.Router) {
				eventRouter.Use(serv.M.RequireGroup())
				eventRouter.Use(serv.M.RateLimitByGroupID())
				eventRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))

				eventRouter.With(serv.M.InstrumentPath("/events")).Post("/", a.CreateAppEvent)
				eventRouter.With(serv.M.Pagination).Get("/", a.GetEventsPaged)

				eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
					eventSubRouter.Use(serv.M.RequireEvent())
					eventSubRouter.Get("/", a.GetAppEvent)
					eventSubRouter.Put("/replay", a.ReplayAppEvent)
				})
			})

			r.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
				eventDeliveryRouter.Use(serv.M.RequireGroup())
				eventDeliveryRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))

				eventDeliveryRouter.With(serv.M.Pagination).Get("/", a.GetEventDeliveriesPaged)
				eventDeliveryRouter.Post("/forceresend", a.ForceResendEventDeliveries)
				eventDeliveryRouter.Post("/batchretry", a.BatchRetryEventDelivery)
				eventDeliveryRouter.Get("/countbatchretryevents", a.CountAffectedEventDeliveries)

				eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
					eventDeliverySubRouter.Use(serv.M.RequireEventDelivery())

					eventDeliverySubRouter.Get("/", a.GetEventDelivery)
					eventDeliverySubRouter.Put("/resend", a.ResendEventDelivery)

					eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
						deliveryRouter.Use(fetchDeliveryAttempts())

						deliveryRouter.Get("/", a.GetDeliveryAttempts)
						deliveryRouter.With(serv.M.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", a.GetDeliveryAttempt)
					})
				})
			})

			r.Route("/security", func(securityRouter chi.Router) {
				securityRouter.Route("/applications/{appID}/keys", func(securitySubRouter chi.Router) {
					securitySubRouter.Use(serv.M.RequireGroup())
					securitySubRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))
					securitySubRouter.Use(serv.M.RequireApp())
					securitySubRouter.Use(serv.M.RequireBaseUrl())
					securitySubRouter.Post("/", a.CreateAppPortalAPIKey)
				})
			})

			r.Route("/subscriptions", func(subscriptionRouter chi.Router) {
				subscriptionRouter.Use(serv.M.RequireGroup())
				subscriptionRouter.Use(serv.M.RateLimitByGroupID())
				subscriptionRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))

				subscriptionRouter.Post("/", a.CreateSubscription)
				subscriptionRouter.With(serv.M.Pagination).Get("/", a.GetSubscriptions)
				subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
				subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
				subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
				subscriptionRouter.Put("/{subscriptionID}/toggle_status", a.ToggleSubscriptionStatus)
			})

			r.Route("/sources", func(sourceRouter chi.Router) {
				sourceRouter.Use(serv.M.RequireGroup())
				sourceRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))
				sourceRouter.Use(serv.M.RequireBaseUrl())

				sourceRouter.Post("/", a.CreateSource)
				sourceRouter.Get("/{sourceID}", a.GetSourceByID)
				sourceRouter.With(serv.M.Pagination).Get("/", a.LoadSourcesPaged)
				sourceRouter.Put("/{sourceID}", a.UpdateSource)
				sourceRouter.Delete("/{sourceID}", a.DeleteSource)
			})
		})
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(serv.M.JsonResponse)
		uiRouter.Use(serv.M.SetupCORS)
		uiRouter.Use(chiMiddleware.Maybe(serv.M.RequireAuth(), middleware.ShouldAuthRoute))
		uiRouter.Use(serv.M.RequireBaseUrl())

		uiRouter.Post("/organisations/process_invite", a.ProcessOrganisationMemberInvite)
		uiRouter.Get("/users/token", a.FindUserByInviteToken)

		uiRouter.Route("/users", func(userRouter chi.Router) {
			userRouter.Use(serv.M.RequireAuthUserMetadata())
			userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
				userSubRouter.Use(serv.M.RequireAuthorizedUser())
				userSubRouter.Get("/profile", a.GetUser)
				userSubRouter.Put("/profile", a.UpdateUser)
				userSubRouter.Put("/password", a.UpdatePassword)
			})
		})

		uiRouter.Post("/users/forgot-password", a.ForgotPassword)
		uiRouter.Post("/users/reset-password", a.ResetPassword)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", a.LoginUser)
			authRouter.Post("/token/refresh", a.RefreshToken)
			authRouter.Post("/logout", a.LogoutUser)
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(serv.M.RequireAuthUserMetadata())
			orgRouter.Use(serv.M.RequireBaseUrl())

			orgRouter.Post("/", a.CreateOrganisation)
			orgRouter.With(serv.M.Pagination).Get("/", a.GetOrganisationsPaged)

			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Use(serv.M.RequireOrganisation())
				orgSubRouter.Use(serv.M.RequireOrganisationMembership())

				orgSubRouter.Get("/", a.GetOrganisation)
				orgSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", a.UpdateOrganisation)
				orgSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", a.DeleteOrganisation)

				orgSubRouter.Route("/invites", func(orgInvitesRouter chi.Router) {
					orgInvitesRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", a.InviteUserToOrganisation)
					orgInvitesRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/resend", a.ResendOrganizationInvite)
					orgInvitesRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/cancel", a.CancelOrganizationInvite)
					orgInvitesRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).With(serv.M.Pagination).Get("/pending", a.GetPendingOrganisationInvites)
				})

				orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
					orgMemberRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

					orgMemberRouter.With(serv.M.Pagination).Get("/", a.GetOrganisationMembers)

					orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {

						orgMemberSubRouter.Get("/", a.GetOrganisationMember)
						orgMemberSubRouter.Put("/", a.UpdateOrganisationMember)
						orgMemberSubRouter.Delete("/", a.DeleteOrganisationMember)

					})
				})

				orgSubRouter.Route("/security", func(securityRouter chi.Router) {
					securityRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

					securityRouter.Post("/keys", a.CreateAPIKey)
					securityRouter.With(serv.M.Pagination).Get("/keys", a.GetAPIKeys)
					securityRouter.Get("/keys/{keyID}", a.GetAPIKeyByID)
					securityRouter.Put("/keys/{keyID}", a.UpdateAPIKey)
					securityRouter.Put("/keys/{keyID}/revoke", a.RevokeAPIKey)
				})

				orgSubRouter.Route("/groups", func(groupRouter chi.Router) {
					groupRouter.Route("/", func(orgSubRouter chi.Router) {
						groupRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", a.CreateGroup)
						groupRouter.Get("/", a.GetGroups)
					})

					groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
						groupSubRouter.Use(serv.M.RequireGroup())
						groupSubRouter.Use(serv.M.RateLimitByGroupID())
						groupSubRouter.Use(serv.M.RequireOrganisationGroupMember())

						groupSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Get("/", a.GetGroup)
						groupSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", a.UpdateGroup)
						groupSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", a.DeleteGroup)

						groupSubRouter.Route("/apps", func(appRouter chi.Router) {
							appRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

							appRouter.Route("/", func(appSubRouter chi.Router) {
								appSubRouter.Post("/", a.CreateApp)
								appRouter.With(serv.M.Pagination).Get("/", a.GetApps)
							})

							appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
								appSubRouter.Use(serv.M.RequireApp())
								appSubRouter.Get("/", a.GetApp)
								appSubRouter.Put("/", a.UpdateApp)
								appSubRouter.Delete("/", a.DeleteApp)

								appSubRouter.Route("/keys", func(keySubRouter chi.Router) {
									keySubRouter.Use(serv.M.RequireBaseUrl())
									keySubRouter.Post("/", a.CreateAppPortalAPIKey)
								})

								appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
									endpointAppSubRouter.Post("/", a.CreateAppEndpoint)
									endpointAppSubRouter.Get("/", a.GetAppEndpoints)

									endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
										e.Use(serv.M.RequireAppEndpoint())

										e.Get("/", a.GetAppEndpoint)
										e.Put("/", a.UpdateAppEndpoint)
										e.Delete("/", a.DeleteAppEndpoint)
									})
								})
							})
						})

						groupSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							eventRouter.Post("/", a.CreateAppEvent)
							eventRouter.With(serv.M.Pagination).Get("/", a.GetEventsPaged)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Use(serv.M.RequireEvent())
								eventSubRouter.Get("/", a.GetAppEvent)
								eventSubRouter.Put("/replay", a.ReplayAppEvent)
							})
						})

						groupSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

							eventDeliveryRouter.With(serv.M.Pagination).Get("/", a.GetEventDeliveriesPaged)
							eventDeliveryRouter.Post("/forceresend", a.ForceResendEventDeliveries)
							eventDeliveryRouter.Post("/batchretry", a.BatchRetryEventDelivery)
							eventDeliveryRouter.Get("/countbatchretryevents", a.CountAffectedEventDeliveries)

							eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
								eventDeliverySubRouter.Use(serv.M.RequireEventDelivery())

								eventDeliverySubRouter.Get("/", a.GetEventDelivery)
								eventDeliverySubRouter.Put("/resend", a.ResendEventDelivery)

								eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
									deliveryRouter.Use(fetchDeliveryAttempts())

									deliveryRouter.Get("/", a.GetDeliveryAttempts)
									deliveryRouter.With(serv.M.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", a.GetDeliveryAttempt)
								})
							})
						})

						groupSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
							subscriptionRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							subscriptionRouter.Post("/", a.CreateSubscription)
							subscriptionRouter.With(serv.M.Pagination).Get("/", a.GetSubscriptions)
							subscriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
							subscriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
							subscriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
						})

						groupSubRouter.Route("/sources", func(sourceRouter chi.Router) {
							sourceRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleAdmin))
							sourceRouter.Use(serv.M.RequireBaseUrl())

							sourceRouter.Post("/", a.CreateSource)
							sourceRouter.Get("/{sourceID}", a.GetSourceByID)
							sourceRouter.With(serv.M.Pagination).Get("/", a.LoadSourcesPaged)
							sourceRouter.Put("/{sourceID}", a.UpdateSource)
							sourceRouter.Delete("/{sourceID}", a.DeleteSource)
						})

						groupSubRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
							dashboardRouter.Get("/summary", a.GetDashboardSummary)
							dashboardRouter.Get("/config", a.GetAllConfigDetails)
						})
					})

				})
			})
		})

		uiRouter.Route("/configuration", func(configRouter chi.Router) {
			configRouter.Use(serv.M.RequireAuthUserMetadata())

			configRouter.Get("/", a.LoadConfiguration)
			configRouter.Post("/", a.CreateConfiguration)
			configRouter.Put("/", a.UpdateConfiguration)

		})
	})

	//App Portal API.
	router.Route("/portal", func(portalRouter chi.Router) {
		portalRouter.Use(serv.M.JsonResponse)
		portalRouter.Use(serv.M.SetupCORS)
		portalRouter.Use(serv.M.RequireAuth())
		portalRouter.Use(serv.M.RequireGroup())
		portalRouter.Use(serv.M.RequireAppID())

		portalRouter.Route("/apps", func(appRouter chi.Router) {
			appRouter.Use(serv.M.RequireAppPortalApplication())
			appRouter.Use(serv.M.RequireAppPortalPermission(auth.RoleAdmin))

			appRouter.Get("/", a.GetApp)

			appRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
				endpointAppSubRouter.Get("/", a.GetAppEndpoints)
				endpointAppSubRouter.Post("/", a.CreateAppEndpoint)

				endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
					e.Use(serv.M.RequireAppEndpoint())

					e.Get("/", a.GetAppEndpoint)
					e.Put("/", a.UpdateAppEndpoint)
				})
			})
		})

		portalRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Use(serv.M.RequireAppPortalApplication())
			eventRouter.Use(serv.M.RequireAppPortalPermission(auth.RoleAdmin))

			eventRouter.With(serv.M.Pagination).Get("/", a.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(serv.M.RequireEvent())
				eventSubRouter.Get("/", a.GetAppEvent)
				eventSubRouter.Put("/replay", a.ReplayAppEvent)
			})
		})

		portalRouter.Route("/subscriptions", func(subsriptionRouter chi.Router) {
			subsriptionRouter.Use(serv.M.RequireAppPortalApplication())
			subsriptionRouter.Use(serv.M.RequireAppPortalPermission(auth.RoleAdmin))

			subsriptionRouter.Post("/", a.CreateSubscription)
			subsriptionRouter.With(serv.M.Pagination).Get("/", a.GetSubscriptions)
			subsriptionRouter.Delete("/{subscriptionID}", a.DeleteSubscription)
			subsriptionRouter.Get("/{subscriptionID}", a.GetSubscription)
			subsriptionRouter.Put("/{subscriptionID}", a.UpdateSubscription)
		})

		portalRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(serv.M.RequireAppPortalApplication())
			eventDeliveryRouter.Use(serv.M.RequireAppPortalPermission(auth.RoleAdmin))

			eventDeliveryRouter.With(serv.M.Pagination).Get("/", a.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/forceresend", a.ForceResendEventDeliveries)
			eventDeliveryRouter.Post("/batchretry", a.BatchRetryEventDelivery)
			eventDeliveryRouter.Get("/countbatchretryevents", a.CountAffectedEventDeliveries)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Use(serv.M.RequireEventDelivery())

				eventDeliverySubRouter.Get("/", a.GetEventDelivery)
				eventDeliverySubRouter.Put("/resend", a.ResendEventDelivery)

				eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Use(fetchDeliveryAttempts())

					deliveryRouter.Get("/", a.GetDeliveryAttempts)
					deliveryRouter.With(serv.M.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", a.GetDeliveryAttempt)
				})
			})
		})
	})

	router.Handle("/queue/monitoring/*", serv.Queue.(*redisqueue.RedisQueue).Monitor())
	router.Handle("/metrics", promhttp.HandlerFor(metrics.Reg(), promhttp.HandlerOpts{}))
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = render.Render(w, r, util.NewServerResponse(fmt.Sprintf("Convoy %v", convoy.GetVersion()), nil, http.StatusOK))
	})
	router.HandleFunc("/*", reactRootHandler)
	return router
}
