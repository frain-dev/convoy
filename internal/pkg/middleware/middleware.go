package middleware

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/frain-dev/convoy/tracer"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/internal/pkg/apm"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/newrelic/go-agent/v3/newrelic"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/felixge/httpsnoop"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/policies"
	"github.com/frain-dev/convoy/util"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/go-chi/render"
)

type contextKey string

const (
	groupCtx            contextKey = "group"
	orgCtx              contextKey = "organisation"
	orgMemberCtx        contextKey = "organisation_member"
	endpointCtx         contextKey = "endpoint"
	endpointsCtx        contextKey = "endpoints"
	eventCtx            contextKey = "event"
	eventDeliveryCtx    contextKey = "eventDelivery"
	authLoginCtx        contextKey = "authLogin"
	authUserCtx         contextKey = "authUser"
	userCtx             contextKey = "user"
	pageableCtx         contextKey = "pageable"
	pageDataCtx         contextKey = "pageData"
	deliveryAttemptsCtx contextKey = "deliveryAttempts"
	hostCtx             contextKey = "host"
	endpointIdCtx       contextKey = "endpointId"
	endpointIdsCtx      contextKey = "endpointIds"
	portalLinkCtx       contextKey = "portal_link"
)

type Middleware struct {
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	endpointRepo      datastore.EndpointRepository
	groupRepo         datastore.GroupRepository
	apiKeyRepo        datastore.APIKeyRepository
	subRepo           datastore.SubscriptionRepository
	sourceRepo        datastore.SourceRepository
	orgRepo           datastore.OrganisationRepository
	orgMemberRepo     datastore.OrganisationMemberRepository
	orgInviteRepo     datastore.OrganisationInviteRepository
	userRepo          datastore.UserRepository
	configRepo        datastore.ConfigurationRepository
	deviceRepo        datastore.DeviceRepository
	portalLinkRepo    datastore.PortalLinkRepository
	cache             cache.Cache
	logger            log.StdLogger
	limiter           limiter.RateLimiter
	tracer            tracer.Tracer
}

type CreateMiddleware struct {
	EventRepo         datastore.EventRepository
	EventDeliveryRepo datastore.EventDeliveryRepository
	EndpointRepo      datastore.EndpointRepository
	GroupRepo         datastore.GroupRepository
	ApiKeyRepo        datastore.APIKeyRepository
	SubRepo           datastore.SubscriptionRepository
	SourceRepo        datastore.SourceRepository
	OrgRepo           datastore.OrganisationRepository
	OrgMemberRepo     datastore.OrganisationMemberRepository
	OrgInviteRepo     datastore.OrganisationInviteRepository
	UserRepo          datastore.UserRepository
	ConfigRepo        datastore.ConfigurationRepository
	DeviceRepo        datastore.DeviceRepository
	PortalLinkRepo    datastore.PortalLinkRepository
	Cache             cache.Cache
	Logger            log.StdLogger
	Limiter           limiter.RateLimiter
	Tracer            tracer.Tracer
}

func NewMiddleware(cs *CreateMiddleware) *Middleware {
	return &Middleware{
		eventRepo:         cs.EventRepo,
		eventDeliveryRepo: cs.EventDeliveryRepo,
		endpointRepo:      cs.EndpointRepo,
		groupRepo:         cs.GroupRepo,
		apiKeyRepo:        cs.ApiKeyRepo,
		subRepo:           cs.SubRepo,
		sourceRepo:        cs.SourceRepo,
		orgRepo:           cs.OrgRepo,
		orgMemberRepo:     cs.OrgMemberRepo,
		orgInviteRepo:     cs.OrgInviteRepo,
		userRepo:          cs.UserRepo,
		configRepo:        cs.ConfigRepo,
		deviceRepo:        cs.DeviceRepo,
		portalLinkRepo:    cs.PortalLinkRepo,
		cache:             cs.Cache,
		logger:            cs.Logger,
		limiter:           cs.Limiter,
		tracer:            cs.Tracer,
	}
}

type AuthorizedLogin struct {
	Username   string    `json:"username,omitempty"`
	Token      string    `json:"token"`
	ExpiryTime time.Time `json:"expiry_time"`
}

func (m *Middleware) InstrumentPath(path string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			m := httpsnoop.CaptureMetrics(next, w, r)
			metrics.RequestDuration().WithLabelValues(r.Method, path,
				strconv.Itoa(m.Code)).Observe(m.Duration.Seconds())
		})
	}
}

func (m *Middleware) InstrumentRequests() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			txn, r, w := apm.StartWebTransaction(r.URL.Path, r, w)
			defer txn.End()

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) WriteRequestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", r.Context().Value(middleware.RequestIDKey).(string))
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) SetupCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.Get()
		if err != nil {
			m.logger.WithError(err).Error("failed to load configuration")
			return
		}

		if env := cfg.Environment; string(env) == "development" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		}

		if r.Method == "OPTIONS" {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) JsonResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) RequireEndpoint() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpointID := chi.URLParam(r, "endpointID")

			var endpoint *datastore.Endpoint
			endpointCacheKey := convoy.EndpointsCacheKey.Get(endpointID).String()

			event := "an error occurred while retrieving endpoint details"
			statusCode := http.StatusBadRequest

			err := m.cache.Get(r.Context(), endpointCacheKey, &endpoint)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), statusCode))
				return
			}

			if endpoint == nil {
				endpoint, err = m.endpointRepo.FindEndpointByID(r.Context(), endpointID)
				if err != nil {
					if errors.Is(err, datastore.ErrEndpointNotFound) {
						event = err.Error()
						statusCode = http.StatusNotFound
					}
					_ = render.Render(w, r, util.NewErrorResponse(event, statusCode))
					return
				}

				err = m.cache.Set(r.Context(), endpointCacheKey, &endpoint, time.Second*1)
				if err != nil {
					_ = render.Render(w, r, util.NewErrorResponse(err.Error(), statusCode))
					return
				}
			}

			r = r.WithContext(setEndpointInContext(r.Context(), endpoint))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireAppID() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := GetAuthUserFromContext(r.Context())

			if !util.IsStringEmpty(authUser.Role.Endpoint) {
				endpointID := authUser.Role.Endpoint
				r = r.WithContext(setEndpointIDInContext(r.Context(), endpointID))
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequirePortalLink() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")

			pLink, err := m.portalLinkRepo.FindPortalLinkByToken(r.Context(), token)
			if err != nil {
				message := "an error occurred while retrieving portal link"
				statusCode := http.StatusBadRequest

				if errors.Is(err, datastore.ErrPortalLinkNotFound) {
					message = "invalid token"
					statusCode = http.StatusUnauthorized
				}

				_ = render.Render(w, r, util.NewErrorResponse(message, statusCode))
				return
			}

			var group *datastore.Group
			groupID := pLink.GroupID

			groupCacheKey := convoy.GroupsCacheKey.Get(groupID).String()
			err = m.cache.Get(r.Context(), groupCacheKey, &group)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			if group == nil {
				group, err = m.groupRepo.FetchGroupByID(r.Context(), groupID)
				if err != nil {
					_ = render.Render(w, r, util.NewErrorResponse("failed to fetch group by id", http.StatusNotFound))
					return
				}

				err = m.cache.Set(r.Context(), groupCacheKey, &group, time.Minute*5)
				if err != nil {
					_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
					return
				}
			}

			r = r.WithContext(setPortalLinkInContext(r.Context(), pLink))
			r = r.WithContext(setEndpointIDsInContext(r.Context(), pLink.Endpoints))
			r = r.WithContext(setGroupInContext(r.Context(), group))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequirePortalLinkEndpoint() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			portalLink := GetPortalLinkFromContext(r.Context())
			endpoint := GetEndpointFromContext(r.Context())

			for _, e := range portalLink.Endpoints {
				if endpoint.UID == e {
					r = r.WithContext(setEndpointIDInContext(r.Context(), endpoint.UID))
					next.ServeHTTP(w, r)
					return
				}
			}

			_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusForbidden))
		})
	}
}

func FilterDeletedEndpoints(endpoints []datastore.Endpoint) []datastore.Endpoint {
	activeEndpoints := make([]datastore.Endpoint, 0)
	for _, endpoint := range endpoints {
		if endpoint.DeletedAt == nil {
			activeEndpoints = append(activeEndpoints, endpoint)
		}
	}
	return activeEndpoints
}

func (m *Middleware) RequireEvent() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			eventId := chi.URLParam(r, "eventID")

			event, err := m.eventRepo.FindEventByID(r.Context(), eventId)
			if err != nil {

				event := "an error occurred while retrieving event details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, datastore.ErrEventNotFound) {
					event = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, util.NewErrorResponse(event, statusCode))
				return
			}

			r = r.WithContext(setEventInContext(r.Context(), event))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireOrganisation() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orgID := chi.URLParam(r, "orgID")

			if util.IsStringEmpty(orgID) {
				orgID = r.URL.Query().Get("orgID")
			}

			org, err := m.orgRepo.FetchOrganisationByID(r.Context(), orgID)
			if err != nil {
				m.logger.WithError(err).Error("failed to fetch organisation")
				_ = render.Render(w, r, util.NewErrorResponse("failed to fetch organisation", http.StatusBadRequest))
				return
			}

			r = r.WithContext(setOrganisationInContext(r.Context(), org))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireAuthUserMetadata() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := GetAuthUserFromContext(r.Context())
			user, ok := authUser.Metadata.(*datastore.User)
			if !ok {
				m.logger.Error("metadata missing in auth user object")
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}

			r = r.WithContext(setUserInContext(r.Context(), user))
			next.ServeHTTP(w, r)
		})
	}
}

// RequireGroupAccess checks if the given authentication creds can access the group. It handles PATs as well
func (m *Middleware) RequireGroupAccess() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := GetAuthUserFromContext(r.Context())
			group := GetGroupFromContext(r.Context())

			if authUser.Metadata != nil { // this signals that a personal api key was used for authentication
				user, _ := authUser.Metadata.(*datastore.User)
				if user != nil {
					member, err := m.orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, group.OrganisationID)
					if err != nil {
						m.logger.WithError(err).Error("failed to fetch organisation member")
						_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
						return
					}

					if member.Role.Type.Is(auth.RoleSuperUser) || member.Role.Group == group.UID {
						r = r.WithContext(setOrganisationMemberInContext(r.Context(), member))
						next.ServeHTTP(w, r)
						return
					}

					_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
					return
				}
			}

			// it's a project api key at this point
			if authUser.Role.Type.Is(auth.RoleAdmin) && authUser.Role.Group == group.UID {
				next.ServeHTTP(w, r)
				return
			}

			_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		})
	}
}

// RejectAppPortalKey ensures that an app portal api key was not used for authentication
func (m *Middleware) RejectAppPortalKey() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := GetAuthUserFromContext(r.Context())
			if authUser.Role.Endpoint != "" {
				// if authUser.Role.App is not empty, an app portal api key was used to authenticate
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireEndpointBelongsToGroup() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint := GetEndpointFromContext(r.Context())
			group := GetGroupFromContext(r.Context())

			if endpoint.GroupID != group.UID {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireOrganisationMembership() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := GetUserFromContext(r.Context())
			org := GetOrganisationFromContext(r.Context())

			member, err := m.orgMemberRepo.FetchOrganisationMemberByUserID(r.Context(), user.UID, org.UID)
			if err != nil {
				m.logger.WithError(err).Error("failed to find organisation member by user id")
				_ = render.Render(w, r, util.NewErrorResponse("failed to fetch organisation member", http.StatusBadRequest))
				return
			}

			r = r.WithContext(setOrganisationMemberInContext(r.Context(), member))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireOrganisationGroupMember() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			member := GetOrganisationMemberFromContext(r.Context())
			if member.Role.Type.Is(auth.RoleSuperUser) {
				// superuser has access to everything
				next.ServeHTTP(w, r)
				return
			}

			group := GetGroupFromContext(r.Context())
			if member.Role.Group == group.UID {
				next.ServeHTTP(w, r)
				return
			}

			_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
		})
	}
}

func (m *Middleware) RequireOrganisationMemberRole(roleType auth.RoleType) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			member := GetOrganisationMemberFromContext(r.Context())
			if member.Role.Type.Is(auth.RoleSuperUser) {
				// superuser has access to everything
				next.ServeHTTP(w, r)
				return
			}

			if member.Role.Type != roleType {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireEventDelivery() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			eventDeliveryID := chi.URLParam(r, "eventDeliveryID")

			eventDelivery, err := m.eventDeliveryRepo.FindEventDeliveryByID(r.Context(), eventDeliveryID)
			if err != nil {

				eventDelivery := "an error occurred while retrieving event delivery details"
				statusCode := http.StatusInternalServerError

				if errors.Is(err, datastore.ErrEventDeliveryNotFound) {
					eventDelivery = err.Error()
					statusCode = http.StatusNotFound
				}

				_ = render.Render(w, r, util.NewErrorResponse(eventDelivery, statusCode))
				return
			}

			r = r.WithContext(setEventDeliveryInContext(r.Context(), eventDelivery))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireDeliveryAttempt() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "deliveryAttemptID")
			attempts := GetDeliveryAttemptsFromContext(r.Context())

			attempt, err := findMessageDeliveryAttempt(attempts, id)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
				return
			}

			r = r.WithContext(setDeliveryAttemptInContext(r.Context(), attempt))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) GetDefaultGroup(r *http.Request, groupRepo datastore.GroupRepository) (*datastore.Group, error) {
	groups, err := groupRepo.LoadGroups(r.Context(), &datastore.GroupFilter{Names: []string{"default-group"}})
	if err != nil {
		return nil, err
	}

	if !(len(groups) > 0) {
		return nil, errors.New("no default group, please your config")
	}

	return groups[0], err
}

func (m *Middleware) RequireGroup() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var group *datastore.Group
			var err error
			var groupID string

			groupID = r.URL.Query().Get("groupId")

			if util.IsStringEmpty(groupID) {
				groupID = r.URL.Query().Get("groupID")
			}

			if util.IsStringEmpty(groupID) {
				groupID = chi.URLParam(r, "projectID")
			}

			if util.IsStringEmpty(groupID) {
				groupID = chi.URLParam(r, "groupID")
			}

			if util.IsStringEmpty(groupID) {
				authUser := GetAuthUserFromContext(r.Context())

				if authUser.Credential.Type == auth.CredentialTypeAPIKey {
					groupID = authUser.Role.Group
				}
			}

			if !util.IsStringEmpty(groupID) {
				groupCacheKey := convoy.GroupsCacheKey.Get(groupID).String()
				err = m.cache.Get(r.Context(), groupCacheKey, &group)
				if err != nil {
					_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
					return
				}

				if group == nil {
					group, err = m.groupRepo.FetchGroupByID(r.Context(), groupID)
					if err != nil {
						_ = render.Render(w, r, util.NewErrorResponse("failed to fetch group by id", http.StatusNotFound))
						return
					}
					err = m.cache.Set(r.Context(), groupCacheKey, &group, time.Minute*5)
					if err != nil {
						_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
						return
					}
				}
			}

			r = r.WithContext(setGroupInContext(r.Context(), group))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireAuth() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			creds, err := GetAuthFromRequest(r)
			if err != nil {
				m.logger.WithError(err).Error("failed to get auth from request")
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusUnauthorized))
				return
			}

			rc, err := realm_chain.Get()
			if err != nil {
				m.logger.WithError(err).Error("failed to get realm chain")
				_ = render.Render(w, r, util.NewErrorResponse("internal server error", http.StatusInternalServerError))
				return
			}

			authUser, err := rc.Authenticate(r.Context(), creds)
			if err != nil {
				m.logger.WithError(err).Error("failed to authenticate")
				_ = render.Render(w, r, util.NewErrorResponse("authorization failed", http.StatusUnauthorized))
				return
			}

			authCtx := context.WithValue(r.Context(), policies.AuthCtxKey, authUser)

			r = r.WithContext(setAuthUserInContext(authCtx, authUser))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireAuthorizedUser() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := GetAuthUserFromContext(r.Context())
			user, ok := authUser.Metadata.(*datastore.User)

			if !ok {
				m.logger.Warn("metadata missing in auth user object")
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}

			userID := chi.URLParam(r, "userID")
			dbUser, err := m.userRepo.FindUserByID(r.Context(), userID)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("failed to fetch user by id", http.StatusNotFound))
				return
			}

			if user.UID != dbUser.UID {
				_ = render.Render(w, r, util.NewErrorResponse(datastore.ErrNotAuthorisedToAccessDocument.Error(), http.StatusForbidden))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireBaseUrl() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cfg, err := config.Get()
			if err != nil {
				m.logger.WithError(err).Error("failed to load configuration")
				return
			}

			r = r.WithContext(setHostInContext(r.Context(), cfg.Host))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequirePermission(role auth.RoleType) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser := GetAuthUserFromContext(r.Context())

			if !authUser.Role.Type.Is(role) {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized role", http.StatusUnauthorized))
				return
			}

			group := GetGroupFromContext(r.Context())
			if group == nil {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized role", http.StatusUnauthorized))
				return
			}

			if !authUser.Role.HasGroup(group.UID) {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized to access group", http.StatusUnauthorized))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetAuthFromRequest(r *http.Request) (*auth.Credential, error) {
	val := r.Header.Get("Authorization")
	authInfo := strings.Split(val, " ")

	if len(authInfo) != 2 {
		err := errors.New("invalid header structure")
		apm.NoticeError(r.Context(), err)
		return nil, err
	}

	credType := auth.CredentialType(strings.ToUpper(authInfo[0]))
	switch credType {
	case auth.CredentialTypeBasic:

		credentials, err := base64.StdEncoding.DecodeString(authInfo[1])
		if err != nil {
			return nil, errors.New("invalid credentials")
		}

		creds := strings.Split(string(credentials), ":")

		if len(creds) != 2 {
			return nil, errors.New("invalid basic credentials")
		}

		return &auth.Credential{
			Type:     auth.CredentialTypeBasic,
			Username: creds[0],
			Password: creds[1],
		}, nil
	case auth.CredentialTypeAPIKey:
		authToken := authInfo[1]

		if util.IsStringEmpty(authToken) {
			return nil, errors.New("empty api key or token")
		}

		prefix := fmt.Sprintf("%s%s", util.Prefix, util.Seperator)
		if strings.HasPrefix(authToken, prefix) {
			return &auth.Credential{
				Type:   auth.CredentialTypeAPIKey,
				APIKey: authToken,
			}, nil
		}

		return &auth.Credential{
			Type:  auth.CredentialTypeJWT,
			Token: authToken,
		}, nil

	default:
		return nil, fmt.Errorf("unknown credential type: %s", credType.String())
	}
}

func (m *Middleware) Pagination(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPerPage := r.URL.Query().Get("perPage")
		rawPage := r.URL.Query().Get("page")
		rawSort := r.URL.Query().Get("sort")

		if len(rawPerPage) == 0 {
			rawPerPage = "20"
		}
		if len(rawPage) == 0 {
			rawPage = "0"
		}
		if len(rawSort) == 0 {
			rawSort = "-1"
		}

		var err error
		sort := -1 // desc by default
		order := strings.ToLower(rawSort)
		if order == "asc" {
			sort = 1
		}

		var perPage int
		if perPage, err = strconv.Atoi(rawPerPage); err != nil {
			perPage = 20
		}

		var page int
		if page, err = strconv.Atoi(rawPage); err != nil {
			page = 0
		}
		pageable := datastore.Pageable{
			Page:    page,
			PerPage: perPage,
			Sort:    sort,
		}
		r = r.WithContext(setPageableInContext(r.Context(), pageable))
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) LogHttpRequest() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()

			defer func() {
				requestFields := requestLogFields(r)
				responseFields := responseLogFields(ww, start)

				logFields := map[string]interface{}{
					"httpRequest":  requestFields,
					"httpResponse": responseFields,
				}

				lvl, err := m.statusLevel(ww.Status()).ToLogrusLevel()
				if err != nil {
					m.logger.WithError(err).Error("Failed to generate status level")
				}

				m.logger.WithFields(logFields).Log(lvl, requestFields["requestURL"])
			}()

			requestID := middleware.GetReqID(r.Context())
			ctx := log.NewContext(r.Context(), m.logger, log.Fields{"request_id": requestID})
			r = r.WithContext(ctx)

			next.ServeHTTP(ww, r)
		})
	}
}

func requestLogFields(r *http.Request) map[string]interface{} {
	scheme := "http"

	if r.TLS != nil {
		scheme = "https"
	}

	requestURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	requestFields := map[string]interface{}{
		"requestURL":    requestURL,
		"requestMethod": r.Method,
		"requestPath":   r.URL.Path,
		"remoteIP":      r.RemoteAddr,
		"proto":         r.Proto,
		"scheme":        scheme,
	}

	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		requestFields["x-request-id"] = reqID
	}

	if len(r.Header) > 0 {
		requestFields["header"] = headerFields(r.Header)
	}

	cfg, err := config.Get()
	if err != nil {
		return nil
	}

	if cfg.Tracer.Type == config.NewRelicTracerProvider {
		txn := newrelic.FromContext(r.Context()).GetLinkingMetadata()

		if cfg.Tracer.NewRelic.DistributedTracerEnabled {
			requestFields["traceID"] = txn.TraceID
			requestFields["spanID"] = txn.SpanID
		}

		requestFields["entityGUID"] = txn.EntityGUID
		requestFields["entityType"] = txn.EntityType
	}

	return requestFields
}

func responseLogFields(w middleware.WrapResponseWriter, t time.Time) map[string]interface{} {
	responseFields := map[string]interface{}{
		"status":  w.Status(),
		"byes":    w.BytesWritten(),
		"latency": time.Since(t),
	}

	if len(w.Header()) > 0 {
		responseFields["header"] = headerFields(w.Header())
	}

	return responseFields
}

func (m *Middleware) statusLevel(status int) log.Level {
	switch {
	case status <= 0:
		return log.WarnLevel
	case status < 400:
		return log.InfoLevel
	case status >= 400 && status < 500:
		return log.WarnLevel
	case status >= 500:
		return log.ErrorLevel
	default:
		return log.InfoLevel
	}
}

func headerFields(header http.Header) map[string]string {
	headerField := map[string]string{}

	for k, v := range header {
		k = strings.ToLower(k)
		switch {
		case len(v) == 0:
			continue
		case len(v) == 1:
			headerField[k] = v[0]
		default:
			headerField[k] = fmt.Sprintf("[%s]", strings.Join(v, "], ["))
		}
		if k == "authorization" || k == "cookie" || k == "set-cookie" {
			headerField[k] = "***"
		}
	}

	return headerField
}

var guestRoutes = []string{
	"/ui/auth/login",
	"/ui/auth/token/refresh",
	"/ui/organisations/process_invite",
	"/ui/users/token",
	"/ui/users/forgot-password",
	"/ui/users/reset-password",
	"/ui/users/verify_email",
	"/ui/auth/register",
}

func ShouldAuthRoute(r *http.Request) bool {
	for _, route := range guestRoutes {
		if r.URL.Path == route {
			return false
		}
	}

	return true
}

func EnsurePeriod(start time.Time, end time.Time) error {
	if start.Unix() > end.Unix() {
		return errors.New("startDate cannot be greater than endDate")
	}

	return nil
}

func (m *Middleware) ComputeDashboardMessages(ctx context.Context, orgId string, searchParams datastore.SearchParams, period datastore.Period) (uint64, []datastore.EventInterval, error) {
	var messagesSent uint64

	messages, err := m.eventRepo.LoadEventIntervals(ctx, orgId, searchParams, period, 1)
	if err != nil {
		m.logger.WithError(err).Error("failed to load message intervals - ")
		return 0, nil, err
	}

	for _, m := range messages {
		messagesSent += m.Count
	}

	return messagesSent, messages, nil
}

func (m *Middleware) RateLimitByGroupWithParams(requestLimit int, windowLength time.Duration) func(next http.Handler) http.Handler {
	return httprate.Limit(requestLimit, windowLength, httprate.WithKeyFuncs(func(req *http.Request) (string, error) {
		return GetGroupFromContext(req.Context()).UID, nil
	}))
}

func (m *Middleware) RateLimitByGroupID() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			group := GetGroupFromContext(r.Context())

			var rateLimitDuration time.Duration
			var err error
			if util.IsStringEmpty(group.RateLimitDuration) {
				rateLimitDuration, err = time.ParseDuration(convoy.RATE_LIMIT_DURATION)
				if err != nil {
					_ = render.Render(w, r, util.NewErrorResponse("an error occured parsing rate limit duration", http.StatusBadRequest))
					return
				}
			} else {
				rateLimitDuration, err = time.ParseDuration(group.RateLimitDuration)
				if err != nil {
					_ = render.Render(w, r, util.NewErrorResponse("an error occured parsing rate limit duration", http.StatusBadRequest))
					return
				}
			}

			var rateLimit int
			if group.RateLimit == 0 {
				rateLimit = convoy.RATE_LIMIT
			} else {
				rateLimit = group.RateLimit
			}

			res, err := m.limiter.Allow(r.Context(), group.UID, rateLimit, int(rateLimitDuration))
			if err != nil {
				message := "an error occured while getting rate limit"
				m.logger.WithError(err).Error(message)
				_ = render.Render(w, r, util.NewErrorResponse(message, http.StatusBadRequest))
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", int(math.Max(0, float64(res.Limit.Rate-1)))))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", int(math.Max(0, float64(res.Remaining-1)))))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%v", res.ResetAfter))

			// the Retry-After header should only be set when the rate limit has been reached
			if res.RetryAfter > time.Nanosecond {
				w.Header().Set("Retry-After", fmt.Sprintf("%v", res.RetryAfter))
			}

			if res.Remaining == 0 {
				_ = render.Render(w, r, util.NewErrorResponse("Too Many Requests", http.StatusTooManyRequests))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireApp() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appID := chi.URLParam(r, "appID")

			endpoints, err := m.endpointRepo.FindEndpointsByAppID(r.Context(), appID)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse("an error occurred while retrieving app details", http.StatusBadRequest))
				return
			}

			if len(endpoints) == 0 {
				_ = render.Render(w, r, util.NewErrorResponse("application not found", http.StatusNotFound))
				return
			}

			r = r.WithContext(setEndpointsInContext(r.Context(), endpoints))
			next.ServeHTTP(w, r)
		})
	}
}

func (m *Middleware) RequireAppEndpoint() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoints := GetEndpointsFromContext(r.Context())
			endPointId := chi.URLParam(r, "endpointID")

			for _, endpoint := range endpoints {
				if endpoint.UID == endPointId {
					r = r.WithContext(setEndpointInContext(r.Context(), &endpoint))
					next.ServeHTTP(w, r)
					return
				}
			}

			_ = render.Render(w, r, util.NewErrorResponse("endpoint not found", http.StatusBadRequest))
		})
	}
}

func (m *Middleware) RequireAppBelongsToGroup() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoints := GetEndpointsFromContext(r.Context())

			group := GetGroupFromContext(r.Context())

			if endpoints[0].GroupID != group.UID {
				_ = render.Render(w, r, util.NewErrorResponse("unauthorized", http.StatusUnauthorized))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func setEndpointIDInContext(ctx context.Context, endpointID string) context.Context {
	return context.WithValue(ctx, endpointIdCtx, endpointID)
}

func setOrganisationInContext(ctx context.Context,
	org *datastore.Organisation,
) context.Context {
	return context.WithValue(ctx, orgCtx, org)
}

func GetOrganisationFromContext(ctx context.Context) *datastore.Organisation {
	return ctx.Value(orgCtx).(*datastore.Organisation)
}

func setOrganisationMemberInContext(ctx context.Context,
	organisationMember *datastore.OrganisationMember,
) context.Context {
	return context.WithValue(ctx, orgMemberCtx, organisationMember)
}

func GetOrganisationMemberFromContext(ctx context.Context) *datastore.OrganisationMember {
	return ctx.Value(orgMemberCtx).(*datastore.OrganisationMember)
}

func setEventInContext(ctx context.Context,
	event *datastore.Event,
) context.Context {
	return context.WithValue(ctx, eventCtx, event)
}

func GetEventFromContext(ctx context.Context) *datastore.Event {
	return ctx.Value(eventCtx).(*datastore.Event)
}

func setEventDeliveryInContext(ctx context.Context,
	eventDelivery *datastore.EventDelivery,
) context.Context {
	return context.WithValue(ctx, eventDeliveryCtx, eventDelivery)
}

func GetEventDeliveryFromContext(ctx context.Context) *datastore.EventDelivery {
	return ctx.Value(eventDeliveryCtx).(*datastore.EventDelivery)
}

func setEndpointInContext(ctx context.Context,
	endpoint *datastore.Endpoint,
) context.Context {
	return context.WithValue(ctx, endpointCtx, endpoint)
}

func GetEndpointFromContext(ctx context.Context) *datastore.Endpoint {
	return ctx.Value(endpointCtx).(*datastore.Endpoint)
}

func setEndpointsInContext(ctx context.Context, endpoints []datastore.Endpoint) context.Context {
	return context.WithValue(ctx, endpointsCtx, endpoints)
}

func GetEndpointsFromContext(ctx context.Context) []datastore.Endpoint {
	return ctx.Value(endpointsCtx).([]datastore.Endpoint)
}

func setGroupInContext(ctx context.Context, group *datastore.Group) context.Context {
	return context.WithValue(ctx, groupCtx, group)
}

func GetGroupFromContext(ctx context.Context) *datastore.Group {
	return ctx.Value(groupCtx).(*datastore.Group)
}

func setPageableInContext(ctx context.Context, pageable datastore.Pageable) context.Context {
	return context.WithValue(ctx, pageableCtx, pageable)
}

func GetPageableFromContext(ctx context.Context) datastore.Pageable {
	return ctx.Value(pageableCtx).(datastore.Pageable)
}

func GetPaginationDataFromContext(ctx context.Context) *datastore.PaginationData {
	return ctx.Value(pageDataCtx).(*datastore.PaginationData)
}

func setDeliveryAttemptInContext(ctx context.Context, attempt *datastore.DeliveryAttempt) context.Context {
	return context.WithValue(ctx, deliveryAttemptsCtx, attempt)
}

func GetDeliveryAttemptFromContext(ctx context.Context) *datastore.DeliveryAttempt {
	return ctx.Value(deliveryAttemptsCtx).(*datastore.DeliveryAttempt)
}

func SetDeliveryAttemptsInContext(ctx context.Context,
	attempts *[]datastore.DeliveryAttempt,
) context.Context {
	return context.WithValue(ctx, deliveryAttemptsCtx, attempts)
}

func GetDeliveryAttemptsFromContext(ctx context.Context) *[]datastore.DeliveryAttempt {
	return ctx.Value(deliveryAttemptsCtx).(*[]datastore.DeliveryAttempt)
}

func setAuthUserInContext(ctx context.Context, a *auth.AuthenticatedUser) context.Context {
	return context.WithValue(ctx, authUserCtx, a)
}

func GetAuthUserFromContext(ctx context.Context) *auth.AuthenticatedUser {
	return ctx.Value(authUserCtx).(*auth.AuthenticatedUser)
}

func setUserInContext(ctx context.Context, a *datastore.User) context.Context {
	return context.WithValue(ctx, userCtx, a)
}

func GetUserFromContext(ctx context.Context) *datastore.User {
	return ctx.Value(userCtx).(*datastore.User)
}

func GetAuthLoginFromContext(ctx context.Context) *AuthorizedLogin {
	return ctx.Value(authLoginCtx).(*AuthorizedLogin)
}

func setHostInContext(ctx context.Context, baseUrl string) context.Context {
	return context.WithValue(ctx, hostCtx, baseUrl)
}

func GetHostFromContext(ctx context.Context) string {
	return ctx.Value(hostCtx).(string)
}

func GetEndpointIDFromContext(r *http.Request) string {
	if endpointID, ok := r.Context().Value(endpointIdCtx).(string); ok {
		return endpointID
	}

	return r.URL.Query().Get("endpointId")
}

func GetSourceIDFromContext(r *http.Request) string {
	return r.URL.Query().Get("sourceId")
}

func setPortalLinkInContext(ctx context.Context, pl *datastore.PortalLink) context.Context {
	return context.WithValue(ctx, portalLinkCtx, pl)
}

func GetPortalLinkFromContext(ctx context.Context) *datastore.PortalLink {
	return ctx.Value(portalLinkCtx).(*datastore.PortalLink)
}

func setEndpointIDsInContext(ctx context.Context, endpointIDs []string) context.Context {
	return context.WithValue(ctx, endpointIdsCtx, endpointIDs)
}

func GetEndpointIDsFromContext(ctx context.Context) []string {
	var endpoints []string

	if endpointIDs, ok := ctx.Value(endpointIdsCtx).([]string); ok {
		return endpointIDs
	}

	return endpoints
}

func findMessageDeliveryAttempt(attempts *[]datastore.DeliveryAttempt, id string) (*datastore.DeliveryAttempt, error) {
	for _, a := range *attempts {
		if a.UID == id {
			return &a, nil
		}
	}
	return nil, datastore.ErrEventDeliveryAttemptNotFound
}
