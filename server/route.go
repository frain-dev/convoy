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

type Server struct {
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

func buildRoutes(serv *se.Server) http.Handler {
	router := chi.NewRouter()

	s := &Server{s: serv}

	router.Use(chiMiddleware.RequestID)
	router.Use(chiMiddleware.Recoverer)
	router.Use(serv.M.WriteRequestIDHeader)
	router.Use(serv.M.InstrumentRequests())
	router.Use(serv.M.LogHttpRequest())

	// Ingestion API
	router.Route("/ingest", func(ingestRouter chi.Router) {
		ingestRouter.Get("/{maskID}", s.HandleCrcCheck)
		ingestRouter.Post("/{maskID}", s.IngestEvent)
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
					appSubRouter.Post("/", s.CreateApp)
					appRouter.With(serv.M.Pagination).Get("/", s.GetApps)
				})

				appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
					appSubRouter.Use(serv.M.RequireApp())

					appSubRouter.Get("/", s.GetApp)
					appSubRouter.Put("/", s.UpdateApp)
					appSubRouter.Delete("/", s.DeleteApp)

					appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
						endpointAppSubRouter.Post("/", s.CreateAppEndpoint)
						endpointAppSubRouter.Get("/", s.GetAppEndpoints)

						endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
							e.Use(serv.M.RequireAppEndpoint())

							e.Get("/", s.GetAppEndpoint)
							e.Put("/", s.UpdateAppEndpoint)
							e.Delete("/", s.DeleteAppEndpoint)
						})
					})
				})
			})

			r.Route("/events", func(eventRouter chi.Router) {
				eventRouter.Use(serv.M.RequireGroup())
				eventRouter.Use(serv.M.RateLimitByGroupID())
				eventRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))

				eventRouter.With(serv.M.InstrumentPath("/events")).Post("/", s.CreateAppEvent)
				eventRouter.With(serv.M.Pagination).Get("/", s.GetEventsPaged)

				eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
					eventSubRouter.Use(serv.M.RequireEvent())
					eventSubRouter.Get("/", s.GetAppEvent)
					eventSubRouter.Put("/replay", s.ReplayAppEvent)
				})
			})

			r.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
				eventDeliveryRouter.Use(serv.M.RequireGroup())
				eventDeliveryRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))

				eventDeliveryRouter.With(serv.M.Pagination).Get("/", s.GetEventDeliveriesPaged)
				eventDeliveryRouter.Post("/forceresend", s.ForceResendEventDeliveries)
				eventDeliveryRouter.Post("/batchretry", s.BatchRetryEventDelivery)
				eventDeliveryRouter.Get("/countbatchretryevents", s.CountAffectedEventDeliveries)

				eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
					eventDeliverySubRouter.Use(serv.M.RequireEventDelivery())

					eventDeliverySubRouter.Get("/", s.GetEventDelivery)
					eventDeliverySubRouter.Put("/resend", s.ResendEventDelivery)

					eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
						deliveryRouter.Use(fetchDeliveryAttempts())

						deliveryRouter.Get("/", s.GetDeliveryAttempts)
						deliveryRouter.With(serv.M.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", s.GetDeliveryAttempt)
					})
				})
			})

			r.Route("/security", func(securityRouter chi.Router) {
				securityRouter.Route("/applications/{appID}/keys", func(securitySubRouter chi.Router) {
					securitySubRouter.Use(serv.M.RequireGroup())
					securitySubRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))
					securitySubRouter.Use(serv.M.RequireApp())
					securitySubRouter.Use(serv.M.RequireBaseUrl())
					securitySubRouter.Post("/", s.CreateAppPortalAPIKey)
				})
			})

			r.Route("/subscriptions", func(subscriptionRouter chi.Router) {
				subscriptionRouter.Use(serv.M.RequireGroup())
				subscriptionRouter.Use(serv.M.RateLimitByGroupID())
				subscriptionRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))

				subscriptionRouter.Post("/", s.CreateSubscription)
				subscriptionRouter.With(serv.M.Pagination).Get("/", s.GetSubscriptions)
				subscriptionRouter.Delete("/{subscriptionID}", s.DeleteSubscription)
				subscriptionRouter.Get("/{subscriptionID}", s.GetSubscription)
				subscriptionRouter.Put("/{subscriptionID}", s.UpdateSubscription)
				subscriptionRouter.Put("/{subscriptionID}/toggle_status", s.ToggleSubscriptionStatus)
			})

			r.Route("/sources", func(sourceRouter chi.Router) {
				sourceRouter.Use(serv.M.RequireGroup())
				sourceRouter.Use(serv.M.RequirePermission(auth.RoleAdmin))
				sourceRouter.Use(serv.M.RequireBaseUrl())

				sourceRouter.Post("/", s.CreateSource)
				sourceRouter.Get("/{sourceID}", s.GetSourceByID)
				sourceRouter.With(serv.M.Pagination).Get("/", s.LoadSourcesPaged)
				sourceRouter.Put("/{sourceID}", s.UpdateSource)
				sourceRouter.Delete("/{sourceID}", s.DeleteSource)
			})
		})
	})

	// UI API.
	router.Route("/ui", func(uiRouter chi.Router) {
		uiRouter.Use(serv.M.JsonResponse)
		uiRouter.Use(serv.M.SetupCORS)
		uiRouter.Use(chiMiddleware.Maybe(serv.M.RequireAuth(), middleware.ShouldAuthRoute))
		uiRouter.Use(serv.M.RequireBaseUrl())

		uiRouter.Post("/organisations/process_invite", s.ProcessOrganisationMemberInvite)
		uiRouter.Get("/users/token", s.FindUserByInviteToken)

		uiRouter.Route("/users", func(userRouter chi.Router) {
			userRouter.Use(serv.M.RequireAuthUserMetadata())
			userRouter.Route("/{userID}", func(userSubRouter chi.Router) {
				userSubRouter.Use(serv.M.RequireAuthorizedUser())
				userSubRouter.Get("/profile", s.GetUser)
				userSubRouter.Put("/profile", s.UpdateUser)
				userSubRouter.Put("/password", s.UpdatePassword)
			})
		})

		uiRouter.Post("/users/forgot-password", s.ForgotPassword)
		uiRouter.Post("/users/reset-password", s.ResetPassword)

		uiRouter.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", s.LoginUser)
			authRouter.Post("/token/refresh", s.RefreshToken)
			authRouter.Post("/logout", s.LogoutUser)
		})

		uiRouter.Route("/organisations", func(orgRouter chi.Router) {
			orgRouter.Use(serv.M.RequireAuthUserMetadata())
			orgRouter.Use(serv.M.RequireBaseUrl())

			orgRouter.Post("/", s.CreateOrganisation)
			orgRouter.With(serv.M.Pagination).Get("/", s.GetOrganisationsPaged)

			orgRouter.Route("/{orgID}", func(orgSubRouter chi.Router) {
				orgSubRouter.Use(serv.M.RequireOrganisation())
				orgSubRouter.Use(serv.M.RequireOrganisationMembership())

				orgSubRouter.Get("/", s.GetOrganisation)
				orgSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", s.UpdateOrganisation)
				orgSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", s.DeleteOrganisation)

				orgSubRouter.Route("/invites", func(orgInvitesRouter chi.Router) {
					orgInvitesRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", s.InviteUserToOrganisation)
					orgInvitesRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/resend", s.ResendOrganizationInvite)
					orgInvitesRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/{inviteID}/cancel", s.CancelOrganizationInvite)
					orgInvitesRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).With(serv.M.Pagination).Get("/pending", s.GetPendingOrganisationInvites)
				})

				orgSubRouter.Route("/members", func(orgMemberRouter chi.Router) {
					orgMemberRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

					orgMemberRouter.With(serv.M.Pagination).Get("/", s.GetOrganisationMembers)

					orgMemberRouter.Route("/{memberID}", func(orgMemberSubRouter chi.Router) {

						orgMemberSubRouter.Get("/", s.GetOrganisationMember)
						orgMemberSubRouter.Put("/", s.UpdateOrganisationMember)
						orgMemberSubRouter.Delete("/", s.DeleteOrganisationMember)

					})
				})

				orgSubRouter.Route("/security", func(securityRouter chi.Router) {
					securityRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

					securityRouter.Post("/keys", s.CreateAPIKey)
					securityRouter.With(serv.M.Pagination).Get("/keys", s.GetAPIKeys)
					securityRouter.Get("/keys/{keyID}", s.GetAPIKeyByID)
					securityRouter.Put("/keys/{keyID}", s.UpdateAPIKey)
					securityRouter.Put("/keys/{keyID}/revoke", s.RevokeAPIKey)
				})

				orgSubRouter.Route("/groups", func(groupRouter chi.Router) {
					groupRouter.Route("/", func(orgSubRouter chi.Router) {
						groupRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Post("/", s.CreateGroup)
						groupRouter.Get("/", s.GetGroups)
					})

					groupRouter.Route("/{groupID}", func(groupSubRouter chi.Router) {
						groupSubRouter.Use(serv.M.RequireGroup())
						groupSubRouter.Use(serv.M.RateLimitByGroupID())
						groupSubRouter.Use(serv.M.RequireOrganisationGroupMember())

						groupSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Get("/", s.GetGroup)
						groupSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Put("/", s.UpdateGroup)
						groupSubRouter.With(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser)).Delete("/", s.DeleteGroup)

						groupSubRouter.Route("/apps", func(appRouter chi.Router) {
							appRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

							appRouter.Route("/", func(appSubRouter chi.Router) {
								appSubRouter.Post("/", s.CreateApp)
								appRouter.With(serv.M.Pagination).Get("/", s.GetApps)
							})

							appRouter.Route("/{appID}", func(appSubRouter chi.Router) {
								appSubRouter.Use(serv.M.RequireApp())
								appSubRouter.Get("/", s.GetApp)
								appSubRouter.Put("/", s.UpdateApp)
								appSubRouter.Delete("/", s.DeleteApp)

								appSubRouter.Route("/keys", func(keySubRouter chi.Router) {
									keySubRouter.Use(serv.M.RequireBaseUrl())
									keySubRouter.Post("/", s.CreateAppPortalAPIKey)
								})

								appSubRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
									endpointAppSubRouter.Post("/", s.CreateAppEndpoint)
									endpointAppSubRouter.Get("/", s.GetAppEndpoints)

									endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
										e.Use(serv.M.RequireAppEndpoint())

										e.Get("/", s.GetAppEndpoint)
										e.Put("/", s.UpdateAppEndpoint)
										e.Delete("/", s.DeleteAppEndpoint)
									})
								})
							})
						})

						groupSubRouter.Route("/events", func(eventRouter chi.Router) {
							eventRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							eventRouter.Post("/", s.CreateAppEvent)
							eventRouter.With(serv.M.Pagination).Get("/", s.GetEventsPaged)

							eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
								eventSubRouter.Use(serv.M.RequireEvent())
								eventSubRouter.Get("/", s.GetAppEvent)
								eventSubRouter.Put("/replay", s.ReplayAppEvent)
							})
						})

						groupSubRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
							eventDeliveryRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleSuperUser))

							eventDeliveryRouter.With(serv.M.Pagination).Get("/", s.GetEventDeliveriesPaged)
							eventDeliveryRouter.Post("/forceresend", s.ForceResendEventDeliveries)
							eventDeliveryRouter.Post("/batchretry", s.BatchRetryEventDelivery)
							eventDeliveryRouter.Get("/countbatchretryevents", s.CountAffectedEventDeliveries)

							eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
								eventDeliverySubRouter.Use(serv.M.RequireEventDelivery())

								eventDeliverySubRouter.Get("/", s.GetEventDelivery)
								eventDeliverySubRouter.Put("/resend", s.ResendEventDelivery)

								eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
									deliveryRouter.Use(fetchDeliveryAttempts())

									deliveryRouter.Get("/", s.GetDeliveryAttempts)
									deliveryRouter.With(serv.M.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", s.GetDeliveryAttempt)
								})
							})
						})

						groupSubRouter.Route("/subscriptions", func(subscriptionRouter chi.Router) {
							subscriptionRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleAdmin))

							subscriptionRouter.Post("/", s.CreateSubscription)
							subscriptionRouter.With(serv.M.Pagination).Get("/", s.GetSubscriptions)
							subscriptionRouter.Delete("/{subscriptionID}", s.DeleteSubscription)
							subscriptionRouter.Get("/{subscriptionID}", s.GetSubscription)
							subscriptionRouter.Put("/{subscriptionID}", s.UpdateSubscription)
						})

						groupSubRouter.Route("/sources", func(sourceRouter chi.Router) {
							sourceRouter.Use(serv.M.RequireOrganisationMemberRole(auth.RoleAdmin))
							sourceRouter.Use(serv.M.RequireBaseUrl())

							sourceRouter.Post("/", s.CreateSource)
							sourceRouter.Get("/{sourceID}", s.GetSourceByID)
							sourceRouter.With(serv.M.Pagination).Get("/", s.LoadSourcesPaged)
							sourceRouter.Put("/{sourceID}", s.UpdateSource)
							sourceRouter.Delete("/{sourceID}", s.DeleteSource)
						})

						groupSubRouter.Route("/dashboard", func(dashboardRouter chi.Router) {
							dashboardRouter.Get("/summary", s.GetDashboardSummary)
							dashboardRouter.Get("/config", s.GetAllConfigDetails)
						})
					})

				})
			})
		})

		uiRouter.Route("/configuration", func(configRouter chi.Router) {
			configRouter.Use(serv.M.RequireAuthUserMetadata())

			configRouter.Get("/", s.LoadConfiguration)
			configRouter.Post("/", s.CreateConfiguration)
			configRouter.Put("/", s.UpdateConfiguration)

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

			appRouter.Get("/", s.GetApp)

			appRouter.Route("/endpoints", func(endpointAppSubRouter chi.Router) {
				endpointAppSubRouter.Get("/", s.GetAppEndpoints)
				endpointAppSubRouter.Post("/", s.CreateAppEndpoint)

				endpointAppSubRouter.Route("/{endpointID}", func(e chi.Router) {
					e.Use(serv.M.RequireAppEndpoint())

					e.Get("/", s.GetAppEndpoint)
					e.Put("/", s.UpdateAppEndpoint)
				})
			})
		})

		portalRouter.Route("/events", func(eventRouter chi.Router) {
			eventRouter.Use(serv.M.RequireAppPortalApplication())
			eventRouter.Use(serv.M.RequireAppPortalPermission(auth.RoleAdmin))

			eventRouter.With(serv.M.Pagination).Get("/", s.GetEventsPaged)

			eventRouter.Route("/{eventID}", func(eventSubRouter chi.Router) {
				eventSubRouter.Use(serv.M.RequireEvent())
				eventSubRouter.Get("/", s.GetAppEvent)
				eventSubRouter.Put("/replay", s.ReplayAppEvent)
			})
		})

		portalRouter.Route("/subscriptions", func(subsriptionRouter chi.Router) {
			subsriptionRouter.Use(serv.M.RequireAppPortalApplication())
			subsriptionRouter.Use(serv.M.RequireAppPortalPermission(auth.RoleAdmin))

			subsriptionRouter.Post("/", s.CreateSubscription)
			subsriptionRouter.With(serv.M.Pagination).Get("/", s.GetSubscriptions)
			subsriptionRouter.Delete("/{subscriptionID}", s.DeleteSubscription)
			subsriptionRouter.Get("/{subscriptionID}", s.GetSubscription)
			subsriptionRouter.Put("/{subscriptionID}", s.UpdateSubscription)
		})

		portalRouter.Route("/eventdeliveries", func(eventDeliveryRouter chi.Router) {
			eventDeliveryRouter.Use(serv.M.RequireAppPortalApplication())
			eventDeliveryRouter.Use(serv.M.RequireAppPortalPermission(auth.RoleAdmin))

			eventDeliveryRouter.With(serv.M.Pagination).Get("/", s.GetEventDeliveriesPaged)
			eventDeliveryRouter.Post("/forceresend", s.ForceResendEventDeliveries)
			eventDeliveryRouter.Post("/batchretry", s.BatchRetryEventDelivery)
			eventDeliveryRouter.Get("/countbatchretryevents", s.CountAffectedEventDeliveries)

			eventDeliveryRouter.Route("/{eventDeliveryID}", func(eventDeliverySubRouter chi.Router) {
				eventDeliverySubRouter.Use(serv.M.RequireEventDelivery())

				eventDeliverySubRouter.Get("/", s.GetEventDelivery)
				eventDeliverySubRouter.Put("/resend", s.ResendEventDelivery)

				eventDeliverySubRouter.Route("/deliveryattempts", func(deliveryRouter chi.Router) {
					deliveryRouter.Use(fetchDeliveryAttempts())

					deliveryRouter.Get("/", s.GetDeliveryAttempts)
					deliveryRouter.With(serv.M.RequireDeliveryAttempt()).Get("/{deliveryAttemptID}", s.GetDeliveryAttempt)
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
