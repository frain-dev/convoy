package handlers

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/render"

    "github.com/frain-dev/convoy/api/models"
    "github.com/frain-dev/convoy/database/postgres"
    "github.com/frain-dev/convoy/datastore"
    "github.com/frain-dev/convoy/internal/pkg/fflag"
    "github.com/frain-dev/convoy/internal/pkg/middleware"
    "github.com/frain-dev/convoy/pkg/circuit_breaker"
    "github.com/frain-dev/convoy/pkg/constants"
    "github.com/frain-dev/convoy/pkg/log"
    "github.com/frain-dev/convoy/pkg/msgpack"
    "github.com/frain-dev/convoy/services"
    "github.com/frain-dev/convoy/util"
)

// CreateEndpoint
//
//	@Summary		Create an endpoint
//	@Description	This endpoint creates an endpoint
//	@Tags			Endpoints
//	@Id				CreateEndpoint
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			endpoint	body		models.CreateEndpoint	true	"Endpoint Details"
//	@Success		201			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints [post]
func (h *Handler) CreateEndpoint(w http.ResponseWriter, r *http.Request) {
    authUser := middleware.GetAuthUserFromContext(r.Context())

    err := h.RM.VersionRequest(r, "CreateEndpoint")
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Version request failed for CreateEndpoint: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request", http.StatusBadRequest))
        return
    }

    var e models.CreateEndpoint

    err = util.ReadJSON(r, &e)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Failed to parse endpoint creation request: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    // Set default content type if not provided
    if e.ContentType == "" {
        e.ContentType = constants.ContentTypeJSON
    }

    err = e.Validate()
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Endpoint creation validation failed: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
        return
    }

    project, err := h.retrieveProject(r)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Failed to retrieve project: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusBadRequest))
        return
    }

    if h.IsReqWithPortalLinkToken(authUser) {
        pLink, innerErr := h.retrievePortalLinkFromToken(r)
        if innerErr != nil {
            _ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
            return
        }

        e.OwnerID = pLink.OwnerID
    }

    ce := services.CreateEndpointService{
        EndpointRepo:   postgres.NewEndpointRepo(h.A.DB),
        ProjectRepo:    postgres.NewProjectRepo(h.A.DB),
        PortalLinkRepo: postgres.NewPortalLinkRepo(h.A.DB),
        Licenser:       h.A.Licenser,
        E:              e,
        ProjectID:      project.UID,
        FeatureFlag:    h.A.FFlag,
        Logger:         h.A.Logger,
    }

    endpoint, err := ce.Run(r.Context())
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resp := &models.EndpointResponse{Endpoint: endpoint}
    serverResponse := util.NewServerResponse(
        "Endpoint created successfully",
        resp, http.StatusCreated)

    rb, err := json.Marshal(serverResponse)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resBytes, err := h.RM.VersionResponse(r, rb, "CreateEndpoint")
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    util.WriteResponse(w, r, resBytes, http.StatusCreated)
}

// GetEndpoint
//
//	@Summary		Retrieve endpoint
//	@Description	This endpoint fetches an endpoint
//	@Id				GetEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [get]
func (h *Handler) GetEndpoint(w http.ResponseWriter, r *http.Request) {
    project, err := h.retrieveProject(r)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Failed to retrieve project: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusBadRequest))
        return
    }

    endpointID := chi.URLParam(r, "endpointID")
    endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Failed to retrieve endpoint: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Resource not found", http.StatusNotFound))
        return
    }

    resp := &models.EndpointResponse{Endpoint: endpoint}
    serverResponse := util.NewServerResponse(
        "Endpoint fetched successfully", resp, http.StatusOK)

    rb, err := json.Marshal(serverResponse)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resBytes, err := h.RM.VersionResponse(r, rb, "GetEndpoint")
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    util.WriteResponse(w, r, resBytes, http.StatusOK)
}

// GetEndpoints
//
//	@Summary		List all endpoints
//	@Description	This endpoint fetches an endpoints
//	@Tags			Endpoints
//	@Id				GetEndpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			request		query		models.QueryListEndpoint	false	"Query Params"
//	@Success		200			{object}	util.ServerResponse{data=models.PagedResponse{content=[]models.EndpointResponse}}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints [get]
func (h *Handler) GetEndpoints(w http.ResponseWriter, r *http.Request) {
    project, err := h.retrieveProject(r)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Failed to retrieve project: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusBadRequest))
        return
    }

    var q *models.QueryListEndpoint
    data := q.Transform(r)

    authUser := middleware.GetAuthUserFromContext(r.Context())
    if h.IsReqWithPortalLinkToken(authUser) {
        portalLink, innerErr := h.retrievePortalLinkFromToken(r)
        if innerErr != nil {
            _ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
            return
        }

        endpointIDs, innerErr := h.getEndpoints(r, portalLink)
        if innerErr != nil {
            _ = render.Render(w, r, util.NewServiceErrResponse(innerErr))
            return
        }

        if len(endpointIDs) == 0 {
            _ = render.Render(w, r, util.NewServerResponse("App events fetched successfully",
                models.PagedResponse{Content: endpointIDs, Pagination: &datastore.PaginationData{PerPage: int64(data.Filter.Pageable.PerPage)}}, http.StatusOK))
            return
        }

        data.Filter.EndpointIDs = endpointIDs
    }

    endpoints, paginationData, err := postgres.NewEndpointRepo(h.A.DB).LoadEndpointsPaged(r.Context(), project.UID, data.Filter, data.Pageable)
    if err != nil {
        h.A.Logger.WithError(err).Error("failed to load endpoints")
        _ = render.Render(w, r, util.NewErrorResponse("Failed to load endpoints", http.StatusBadRequest))
        return
    }

    if h.A.FFlag.CanAccessFeature(fflag.CircuitBreaker) && h.A.Licenser.CircuitBreaking() && len(endpoints) > 0 {
        // fetch keys from redis and mutate endpoints slice
        keys := make([]string, len(endpoints))
        for i := 0; i < len(endpoints); i++ {
            keys[i] = fmt.Sprintf("breaker:%s", endpoints[i].UID)
        }

        cbs, err := h.A.Redis.MGet(r.Context(), keys...).Result()
        if err != nil {
            _ = render.Render(w, r, util.NewServiceErrResponse(err))
            return
        }

        for i := 0; i < len(cbs); i++ {
            if cbs[i] != nil {
                str, ok := cbs[i].(string)
                if ok {
                    var c circuit_breaker.CircuitBreaker
                    asBytes := []byte(str)
                    innerErr := msgpack.DecodeMsgPack(asBytes, &c)
                    if innerErr != nil {
                        continue
                    }
                    endpoints[i].FailureRate = c.FailureRate
                }
            }
        }
    }

    resp := models.NewListResponse(endpoints, func(endpoint datastore.Endpoint) models.EndpointResponse {
        return models.EndpointResponse{Endpoint: &endpoint}
    })

    serverResponse := util.NewServerResponse(
        "Endpoints fetched successfully",
        models.PagedResponse{Content: &resp, Pagination: &paginationData}, http.StatusOK)

    rb, err := json.Marshal(serverResponse)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resBytes, err := h.RM.VersionResponse(r, rb, "GetEndpoints")
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    util.WriteResponse(w, r, resBytes, http.StatusOK)
}

// UpdateEndpoint
//
//	@Summary		Update an endpoint
//	@Description	This endpoint updates an endpoint
//	@Id				UpdateEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			endpointID	path		string					true	"Endpoint ID"
//	@Param			endpoint	body		models.UpdateEndpoint	true	"Endpoint Details"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [put]
func (h *Handler) UpdateEndpoint(w http.ResponseWriter, r *http.Request) {
    err := h.RM.VersionRequest(r, "UpdateEndpoint")
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    project, err := h.retrieveProject(r)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    var e models.UpdateEndpoint

    err = util.ReadJSON(r, &e)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    endpointID := chi.URLParam(r, "endpointID")
    endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    // Set default content type if not provided
    if e.ContentType == nil || *e.ContentType == "" {
        defaultContentType := constants.ContentTypeJSON
        e.ContentType = &defaultContentType
    }

    err = e.Validate()
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    ce := services.UpdateEndpointService{
        Cache:        h.A.Cache,
        EndpointRepo: postgres.NewEndpointRepo(h.A.DB),
        ProjectRepo:  postgres.NewProjectRepo(h.A.DB),
        Licenser:     h.A.Licenser,
        FeatureFlag:  h.A.FFlag,
        Logger:       h.A.Logger,
        E:            e,
        Endpoint:     endpoint,
        Project:      project,
    }

    endpoint, err = ce.Run(r.Context())
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resp := &models.EndpointResponse{Endpoint: endpoint}
    serverResponse := util.NewServerResponse("Endpoint updated successfully", resp, http.StatusAccepted)

    rb, err := json.Marshal(serverResponse)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resBytes, err := h.RM.VersionResponse(r, rb, "UpdateEndpoint")
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    util.WriteResponse(w, r, resBytes, http.StatusAccepted)
}

// DeleteEndpoint
//
//	@Summary		Delete endpoint
//	@Description	This endpoint deletes an endpoint
//	@Tags			Endpoints
//	@Id				DeleteEndpoint
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		200			{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID} [delete]
func (h *Handler) DeleteEndpoint(w http.ResponseWriter, r *http.Request) {
    project, err := h.retrieveProject(r)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    endpointID := chi.URLParam(r, "endpointID")
    endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    err = postgres.NewEndpointRepo(h.A.DB).DeleteEndpoint(r.Context(), endpoint, project.UID)
    if err != nil {
        log.FromContext(r.Context()).WithError(err).Error("failed to delete endpoint")
        _ = render.Render(w, r, util.NewErrorResponse("failed to delete endpoint", http.StatusBadRequest))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Endpoint deleted successfully", nil, http.StatusOK))
}

// ExpireSecret
//
//	@Summary		Roll endpoint secret
//	@Description	This endpoint expires and re-generates the endpoint secret.
//	@Id				ExpireSecret
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string				true	"Project ID"
//	@Param			endpointID	path		string				true	"Endpoint ID"
//	@Param			endpoint	body		models.ExpireSecret	true	"Expire Secret Body Parameters"
//	@Success		200			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/expire_secret [put]
func (h *Handler) ExpireSecret(w http.ResponseWriter, r *http.Request) {
    project, err := h.retrieveProject(r)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    var e *models.ExpireSecret
    err = util.ReadJSON(r, &e)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    endpointID := chi.URLParam(r, "endpointID")
    endpoint, err := h.retrieveEndpoint(r.Context(), endpointID, project.UID)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    xs := services.ExpireSecretService{
        Queuer:       h.A.Queue,
        Cache:        h.A.Cache,
        EndpointRepo: postgres.NewEndpointRepo(h.A.DB),
        ProjectRepo:  postgres.NewProjectRepo(h.A.DB),
        S:            e,
        Endpoint:     endpoint,
        Project:      project,
    }

    endpoint, err = xs.Run(r.Context())
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resp := &models.EndpointResponse{Endpoint: endpoint}
    _ = render.Render(w, r, util.NewServerResponse("endpoint secret expired successfully",
        resp, http.StatusOK))
}

// PauseEndpoint
//
//	@Summary		Pause endpoint
//	@Description	Toggles an endpoint's status between active and paused states
//	@Id				PauseEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/pause [put]
func (h *Handler) PauseEndpoint(w http.ResponseWriter, r *http.Request) {
    project, err := h.retrieveProject(r)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    ps := services.PauseEndpointService{
        EndpointRepo: postgres.NewEndpointRepo(h.A.DB),
        ProjectID:    project.UID,
        EndpointId:   chi.URLParam(r, "endpointID"),
    }

    endpoint, err := ps.Run(r.Context())
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resp := &models.EndpointResponse{Endpoint: endpoint}
    serverResponse := util.NewServerResponse("endpoint status updated successfully", resp, http.StatusAccepted)

    rb, err := json.Marshal(serverResponse)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resBytes, err := h.RM.VersionResponse(r, rb, "UpdateEndpoint")
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    util.WriteResponse(w, r, resBytes, http.StatusAccepted)
}

// ActivateEndpoint
//
//	@Summary		Activate endpoint
//	@Description	Activated an inactive endpoint
//	@Id				ActivateEndpoint
//	@Tags			Endpoints
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string	true	"Project ID"
//	@Param			endpointID	path		string	true	"Endpoint ID"
//	@Success		202			{object}	util.ServerResponse{data=models.EndpointResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/{endpointID}/activate [post]
func (h *Handler) ActivateEndpoint(w http.ResponseWriter, r *http.Request) {
    if !h.A.Licenser.CircuitBreaking() || !h.A.FFlag.CanAccessFeature(fflag.CircuitBreaker) {
        _ = render.Render(w, r, util.NewErrorResponse("feature not enabled", http.StatusBadRequest))
        return
    }

    project, err := h.retrieveProject(r)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
        return
    }

    aes := services.ActivateEndpointService{
        EndpointRepo: postgres.NewEndpointRepo(h.A.DB),
        ProjectID:    project.UID,
        EndpointId:   chi.URLParam(r, "endpointID"),
    }

    endpoint, err := aes.Run(r.Context())
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    cbs, err := h.A.Redis.Get(r.Context(), fmt.Sprintf("breaker:%s", endpoint.UID)).Result()
    if err != nil {
        h.A.Logger.WithError(err).Error("failed to find circuit breaker")
    }

    if len(cbs) > 0 {
        c, innerErr := circuit_breaker.NewCircuitBreakerFromStore([]byte(cbs), h.A.Logger.(*log.Logger))
        if innerErr != nil {
            h.A.Logger.WithError(innerErr).Error("failed to decode circuit breaker")
        } else {
            c.Reset(time.Now())
            b, msgPackErr := msgpack.EncodeMsgPack(c)
            if msgPackErr != nil {
                h.A.Logger.WithError(msgPackErr).Error("failed to encode circuit breaker")
            }
            h.A.Redis.Set(r.Context(), fmt.Sprintf("breaker:%s", endpoint.UID), b, time.Minute*5)
        }
    }

    resp := &models.EndpointResponse{Endpoint: endpoint}
    serverResponse := util.NewServerResponse("endpoint status successfully activated", resp, http.StatusAccepted)

    rb, err := json.Marshal(serverResponse)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    resBytes, err := h.RM.VersionResponse(r, rb, "UpdateEndpoint")
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    util.WriteResponse(w, r, resBytes, http.StatusAccepted)
}

// TestOAuth2Connection
//
//	@Summary		Test OAuth2 connection
//	@Description	This endpoint tests the OAuth2 connection by attempting to exchange a token
//	@Tags			Endpoints
//	@Id				TestOAuth2Connection
//	@Accept			json
//	@Produce		json
//	@Param			projectID	path		string					true	"Project ID"
//	@Param			oauth2		body		models.TestOAuth2Request	true	"OAuth2 Configuration"
//	@Success		200			{object}	util.ServerResponse{data=models.TestOAuth2Response}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/endpoints/oauth2/test [post]
func (h *Handler) TestOAuth2Connection(w http.ResponseWriter, r *http.Request) {
    var testReq models.TestOAuth2Request
    err := util.ReadJSON(r, &testReq)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Failed to parse OAuth2 test request: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    err = testReq.Validate()
    if err != nil {
        h.A.Logger.WithError(err).Errorf("OAuth2 test request validation failed: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Invalid input provided", http.StatusBadRequest))
        return
    }

    // Transform API model to datastore model
    if testReq.OAuth2 == nil {
        _ = render.Render(w, r, util.NewErrorResponse("OAuth2 configuration is required", http.StatusBadRequest))
        return
    }
    oauth2Config := testReq.OAuth2.Transform()
    if oauth2Config == nil {
        _ = render.Render(w, r, util.NewErrorResponse("OAuth2 configuration is required", http.StatusBadRequest))
        return
    }

    // Create a temporary endpoint for testing
    testEndpoint := &datastore.Endpoint{
        UID: "test",
        Authentication: &datastore.EndpointAuthentication{
            Type:   datastore.OAuth2Authentication,
            OAuth2: oauth2Config,
        },
    }

    // Initialize OAuth2 token service
    oauth2Service := services.NewOAuth2TokenService(h.A.Cache, h.A.Logger)

    // Get authorization header (includes token type)
    authHeader, err := oauth2Service.GetAuthorizationHeader(r.Context(), testEndpoint)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("OAuth2 token exchange failed: %v", err)
        _ = render.Render(w, r, util.NewServerResponse(
            "OAuth2 connection test failed",
            models.TestOAuth2Response{
                Success: false,
                Error:   err.Error(),
            },
            http.StatusOK,
        ))
        return
    }

    // Parse token type and access token from authorization header
    // Format: "TokenType access_token" (e.g., "Bearer token123" or "CustomType token123")
    parts := strings.SplitN(authHeader, " ", 2)
    tokenType := "Bearer" // Default
    accessToken := ""
    if len(parts) == 2 {
        tokenType = parts[0]
        accessToken = parts[1]
    } else {
        // Fallback if format is unexpected
        accessToken = authHeader
    }

    // Get the cached token to return full response details (including expires_at)
    cacheKey := "oauth2_token:test"
    var cachedToken services.CachedToken
    err = h.A.Cache.Get(r.Context(), cacheKey, &cachedToken)

    var expiresAt time.Time
    if err == nil {
        // Use token type from cache if available (more accurate)
        if cachedToken.TokenType != "" {
            tokenType = cachedToken.TokenType
        }
        if cachedToken.AccessToken != "" {
            accessToken = cachedToken.AccessToken
        }
        expiresAt = cachedToken.ExpiresAt
    }

    // Return full response with token details
    resp := models.TestOAuth2Response{
        Success:     true,
        AccessToken: accessToken,
        TokenType:   tokenType,
        ExpiresAt:   expiresAt,
        Message:     "OAuth2 connection successful",
    }

    // Clean up test cache entry
    _ = h.A.Cache.Delete(r.Context(), cacheKey)

    _ = render.Render(w, r, util.NewServerResponse("OAuth2 connection test successful", resp, http.StatusOK))
}

func (h *Handler) retrieveEndpoint(ctx context.Context, endpointID, projectID string) (*datastore.Endpoint, error) {
    endpointRepo := postgres.NewEndpointRepo(h.A.DB)
    return endpointRepo.FindEndpointByID(ctx, endpointID, projectID)
}
