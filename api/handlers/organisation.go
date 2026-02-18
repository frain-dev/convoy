package handlers

import (
    "errors"
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/render"
    "gopkg.in/guregu/null.v4"

    "github.com/frain-dev/convoy/api/models"
    "github.com/frain-dev/convoy/api/policies"
    "github.com/frain-dev/convoy/auth"
    "github.com/frain-dev/convoy/database/postgres"
    "github.com/frain-dev/convoy/datastore"
    "github.com/frain-dev/convoy/internal/organisation_members"
    "github.com/frain-dev/convoy/internal/organisations"
    "github.com/frain-dev/convoy/internal/pkg/batch_tracker"
    fflag "github.com/frain-dev/convoy/internal/pkg/fflag"
    m "github.com/frain-dev/convoy/internal/pkg/middleware"
    "github.com/frain-dev/convoy/internal/projects"
    "github.com/frain-dev/convoy/pkg/log"
    "github.com/frain-dev/convoy/services"
    "github.com/frain-dev/convoy/util"
    "github.com/frain-dev/convoy/worker/task"
)

var (
    ErrNotEarlyAdopterFeature = errors.New("not early adopter feature")
    ErrOverrideNotAllowed     = errors.New("override not allowed")
    ErrNotLicensed            = errors.New("not licensed")
)

func (h *Handler) GetOrganisation(w http.ResponseWriter, r *http.Request) {
    org, err := h.retrieveOrganisation(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if err = h.A.Authz.Authorize(r.Context(), string(policies.PermissionOrganisationManage), org); err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Organisation fetched successfully", org, http.StatusOK))
}

func (h *Handler) GetOrganisationsPaged(w http.ResponseWriter, r *http.Request) { // TODO: change to GetUserOrganisationsPaged
    pageable := m.GetPageableFromContext(r.Context())
    user, err := h.retrieveUser(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    organisations, paginationData, err := organisation_members.New(h.A.Logger, h.A.DB).LoadUserOrganisationsPaged(r.Context(), user.UID, pageable)
    if err != nil {
        log.FromContext(r.Context()).WithError(err).Error("failed to fetch user organisations")
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
        models.PagedResponse{Content: &organisations, Pagination: &paginationData}, http.StatusOK))
}

func (h *Handler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
    var newOrg datastore.OrganisationRequest
    err := util.ReadJSON(r, &newOrg)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Failed to parse organisation creation request: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    user, err := h.retrieveUser(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if err = h.A.Authz.Authorize(r.Context(), string(policies.PermissionOrganisationAdd), user); err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
        return
    }

    orgRepo := organisations.New(h.A.Logger, h.A.DB)
    co := services.CreateOrganisationService{
        OrgRepo:       orgRepo,
        OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
        NewOrg:        &newOrg,
        User:          user,
        Licenser:      h.A.Licenser,
        Logger:        h.A.Logger,
    }

    organisation, err := co.Run(r.Context())
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("OrganisationRequest created successfully", organisation, http.StatusCreated))
}

func (h *Handler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
    var orgUpdate datastore.OrganisationRequest
    err := util.ReadJSON(r, &orgUpdate)
    if err != nil {
        h.A.Logger.WithError(err).Errorf("Failed to parse organisation update request: %v", err)
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    org, err := h.retrieveOrganisation(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if err = h.A.Authz.Authorize(r.Context(), string(policies.PermissionOrganisationManage), org); err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
        return
    }

    orgRepo := organisations.New(h.A.Logger, h.A.DB)
    us := services.UpdateOrganisationService{
        OrgRepo:       orgRepo,
        OrgMemberRepo: organisation_members.New(h.A.Logger, h.A.DB),
        Org:           org,
        Update:        &orgUpdate,
    }

    org, err = us.Run(r.Context())
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("OrganisationRequest updated successfully", org, http.StatusAccepted))
}

func (h *Handler) DeleteOrganisation(w http.ResponseWriter, r *http.Request) {
    org, err := h.retrieveOrganisation(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if err = h.A.Authz.Authorize(r.Context(), string(policies.PermissionOrganisationManageAll), org); err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
        return
    }

    orgRepo := organisations.New(h.A.Logger, h.A.DB)
    err = orgRepo.DeleteOrganisation(r.Context(), org.UID)
    if err != nil {
        log.FromContext(r.Context()).WithError(err).Error("failed to delete organisation")
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("OrganisationRequest deleted successfully", nil, http.StatusOK))
}

func (h *Handler) UpdateOrganisationFeatureFlags(w http.ResponseWriter, r *http.Request) {
    var featureFlagsUpdate datastore.UpdateOrganisationFeatureFlags
    err := util.ReadJSON(r, &featureFlagsUpdate)
    if err != nil {
        h.A.Logger.WithError(err).Error("Failed to parse feature flags update request")
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    org, err := h.retrieveOrganisation(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if err = h.A.Authz.Authorize(r.Context(), string(policies.PermissionOrganisationManage), org); err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized", http.StatusForbidden))
        return
    }

    user, err := h.retrieveUser(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    // Users can only update user-controlled feature flags (Early Adopter features)
    for featureKey, enabled := range featureFlagsUpdate.FeatureFlags {
        if err := h.updateFeatureFlag(w, r, featureKey, enabled, org, user); err != nil {
            return
        }
    }

    _ = render.Render(w, r, util.NewServerResponse("Feature flags updated successfully", nil, http.StatusOK))
}

func (h *Handler) GetEarlyAdopterFeatures(w http.ResponseWriter, r *http.Request) {
    org, err := h.retrieveOrganisation(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _, err = h.retrieveMembership(r)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: must be a member of the organisation", http.StatusForbidden))
        return
    }

    features := fflag.GetEarlyAdopterFeatures()
    responseFeatures := make([]models.EarlyAdopterFeature, 0, len(features))

    earlyAdopterFeatures, err := postgres.LoadEarlyAdopterFeaturesByOrg(r.Context(), h.A.DB, org.UID)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    featureMap := make(map[string]*datastore.EarlyAdopterFeature)
    for i := range earlyAdopterFeatures {
        featureMap[earlyAdopterFeatures[i].FeatureKey] = &earlyAdopterFeatures[i]
    }

    featureNames := map[fflag.FeatureFlagKey]string{
        fflag.MTLS:               "mTLS",
        fflag.OAuthTokenExchange: "OAuth Token Exchange",
    }

    featureDescriptions := map[fflag.FeatureFlagKey]string{
        fflag.MTLS:               "Mutual TLS support for secure endpoint communication",
        fflag.OAuthTokenExchange: "OAuth token exchange functionality for endpoint authentication",
    }

    for _, featureKey := range features {
        enabled := false
        if feature, ok := featureMap[string(featureKey)]; ok {
            enabled = feature.Enabled
        }

        responseFeatures = append(responseFeatures, models.EarlyAdopterFeature{
            Key:         string(featureKey),
            Name:        featureNames[featureKey],
            Description: featureDescriptions[featureKey],
            Enabled:     enabled,
        })
    }

    _ = render.Render(w, r, util.NewServerResponse("Early adopter features fetched successfully", responseFeatures, http.StatusOK))
}

func (h *Handler) GetOrganisationFeatureFlags(w http.ResponseWriter, r *http.Request) {
    org, err := h.retrieveOrganisation(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _, err = h.retrieveMembership(r)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: must be a member of the organisation", http.StatusForbidden))
        return
    }

    featureFlags := make(map[string]bool)

    for featureKey := range fflag.DefaultFeaturesState {
        enabled := h.A.FFlag.CanAccessOrgFeature(
            r.Context(), featureKey, h.A.FeatureFlagFetcher, h.A.EarlyAdopterFeatureFetcher, org.UID)
        featureFlags[string(featureKey)] = enabled
    }

    _ = render.Render(w, r, util.NewServerResponse("Feature flags fetched successfully", featureFlags, http.StatusOK))
}

func (h *Handler) updateFeatureFlag(w http.ResponseWriter, r *http.Request, featureKey string, enabled bool, org *datastore.Organisation, user *datastore.User) error {
    flagKey := fflag.FeatureFlagKey(featureKey)
    if !fflag.IsEarlyAdopterFeature(flagKey) {
        _ = render.Render(w, r, util.NewErrorResponse(
            "Feature flag "+featureKey+" is not user-controlled and cannot be updated via API", http.StatusBadRequest))
        return ErrNotEarlyAdopterFeature
    }

    if !h.isEarlyAdopterFeatureLicensed(flagKey) {
        _ = render.Render(w, r, util.NewErrorResponse(
            "Feature flag "+featureKey+" is not available in your license plan", http.StatusForbidden))
        return ErrNotLicensed
    }

    feature := &datastore.EarlyAdopterFeature{
        OrganisationID: org.UID,
        FeatureKey:     featureKey,
        Enabled:        enabled,
        EnabledBy:      null.StringFrom(user.UID),
    }

    if enabled {
        feature.EnabledAt = null.TimeFrom(time.Now())
    }

    err := postgres.UpsertEarlyAdopterFeature(r.Context(), h.A.DB, feature)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return err
    }

    return nil
}

func (h *Handler) isEarlyAdopterFeatureLicensed(featureKey fflag.FeatureFlagKey) bool {
    switch featureKey {
    case fflag.MTLS:
        return h.A.Licenser.MutualTLS()
    case fflag.OAuthTokenExchange:
        return h.A.Licenser.OAuth2EndpointAuth()
    default:
        return false
    }
}

// isInstanceAdmin checks if the current user is an instance admin
func (h *Handler) isInstanceAdmin(r *http.Request) bool {
    user, err := h.retrieveUser(r)
    if err != nil {
        return false
    }

    member, err := organisation_members.New(h.A.Logger, h.A.DB).FetchInstanceAdminByUserID(r.Context(), user.UID)
    if err != nil {
        return false
    }

    return member.Role.Type == auth.RoleInstanceAdmin
}

// GetAllFeatureFlags returns all system feature flags (instance admin only)
func (h *Handler) GetAllFeatureFlags(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    flags, err := postgres.LoadFeatureFlags(r.Context(), h.A.DB)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Feature flags fetched successfully", flags, http.StatusOK))
}

// GetAllOrganisations returns all organizations (instance admin only)
func (h *Handler) GetAllOrganisations(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    pageable := m.GetPageableFromContext(r.Context())
    // Set a higher default limit for admin (1000 records)
    if pageable.PerPage == 0 {
        pageable.PerPage = 1000
    }
    pageable.SetCursors()

    // Get search query parameter
    search := r.URL.Query().Get("search")

    orgRepo := organisations.New(h.A.Logger, h.A.DB)
    var orgs []datastore.Organisation
    var paginationData datastore.PaginationData
    var err error

    if search != "" {
        orgs, paginationData, err = orgRepo.LoadOrganisationsPagedWithSearch(r.Context(), pageable, search)
    } else {
        orgs, paginationData, err = orgRepo.LoadOrganisationsPaged(r.Context(), pageable)
    }

    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
        models.PagedResponse{Content: &orgs, Pagination: &paginationData}, http.StatusOK))
}

// GetOrganisationOverrides returns all feature flag overrides for a specific organization (instance admin only)
func (h *Handler) GetOrganisationOverrides(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    orgID := chi.URLParam(r, "orgID")
    if orgID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
        return
    }

    overrides, err := postgres.LoadFeatureFlagOverridesByOwner(r.Context(), h.A.DB, "organisation", orgID)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    // Enrich overrides with feature flag keys for easier frontend mapping
    type OverrideWithKey struct {
        datastore.FeatureFlagOverride
        FeatureKey string `json:"feature_key"`
    }

    enrichedOverrides := make([]OverrideWithKey, 0, len(overrides))
    for i := range overrides {
        featureFlag, err := postgres.FetchFeatureFlagByID(r.Context(), h.A.DB, overrides[i].FeatureFlagID)
        if err != nil {
            log.FromContext(r.Context()).WithError(err).Warnf("Failed to fetch feature flag for override: %s", overrides[i].FeatureFlagID)
            continue
        }

        enrichedOverrides = append(enrichedOverrides, OverrideWithKey{
            FeatureFlagOverride: overrides[i],
            FeatureKey:          featureFlag.FeatureKey,
        })
    }

    _ = render.Render(w, r, util.NewServerResponse("Feature flag overrides fetched successfully", enrichedOverrides, http.StatusOK))
}

// UpdateOrganisationOverride creates or updates a feature flag override for any organization (instance admin only)
func (h *Handler) UpdateOrganisationOverride(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    orgID := chi.URLParam(r, "orgID")
    if orgID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
        return
    }

    var overrideRequest datastore.UpdateOrganisationOverride
    err := util.ReadJSON(r, &overrideRequest)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    user, err := h.retrieveUser(r)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    // Fetch the feature flag
    featureFlag, err := postgres.FetchFeatureFlagByKey(r.Context(), h.A.DB, overrideRequest.FeatureKey)
    if err != nil {
        if errors.Is(err, postgres.ErrFeatureFlagNotFound) {
            _ = render.Render(w, r, util.NewErrorResponse("Feature flag not found: "+overrideRequest.FeatureKey, http.StatusBadRequest))
            return
        }
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if overrideRequest.FeatureKey != "circuit-breaker" {
        _ = render.Render(w, r, util.NewErrorResponse(
            "Feature flag "+overrideRequest.FeatureKey+" does not support org overrides. Use the early adopter features API for user-controlled features.", http.StatusBadRequest))
        return
    }

    override := &datastore.FeatureFlagOverride{
        FeatureFlagID: featureFlag.UID,
        OwnerType:     "organisation",
        OwnerID:       orgID,
        Enabled:       overrideRequest.Enabled,
        EnabledBy:     null.StringFrom(user.UID),
    }

    if overrideRequest.Enabled {
        override.EnabledAt = null.TimeFrom(time.Now())
    }

    err = postgres.UpsertFeatureFlagOverride(r.Context(), h.A.DB, override)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Feature flag override updated successfully", override, http.StatusOK))
}

// DeleteOrganisationOverride deletes a feature flag override for any organization (instance admin only)
func (h *Handler) DeleteOrganisationOverride(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    orgID := chi.URLParam(r, "orgID")
    if orgID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
        return
    }

    featureKey := chi.URLParam(r, "featureKey")
    if featureKey == "" {
        _ = render.Render(w, r, util.NewErrorResponse("feature key is required", http.StatusBadRequest))
        return
    }

    // Fetch the feature flag to get its ID
    featureFlag, err := postgres.FetchFeatureFlagByKey(r.Context(), h.A.DB, featureKey)
    if err != nil {
        if errors.Is(err, postgres.ErrFeatureFlagNotFound) {
            _ = render.Render(w, r, util.NewErrorResponse("Feature flag not found: "+featureKey, http.StatusBadRequest))
            return
        }
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    err = postgres.DeleteFeatureFlagOverride(r.Context(), h.A.DB, "organisation", orgID, featureFlag.UID)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Feature flag override deleted successfully", nil, http.StatusOK))
}

// GetOrganisationCircuitBreakerConfig returns the circuit breaker configuration for an organization (instance admin only)
// It gets the config from the first project in the organization, or returns defaults if no projects exist
func (h *Handler) GetOrganisationCircuitBreakerConfig(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    orgID := chi.URLParam(r, "orgID")
    if orgID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
        return
    }

    projectRepo := projects.New(h.A.Logger, h.A.DB)
    projects, err := projectRepo.LoadProjects(r.Context(), &datastore.ProjectFilter{OrgID: orgID})
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if len(projects) == 0 {
        defaults := datastore.DefaultCircuitBreakerConfiguration
        response := map[string]interface{}{
            "sample_rate":                   defaults.SampleRate,
            "error_timeout":                 defaults.ErrorTimeout,
            "failure_threshold":             defaults.FailureThreshold,
            "success_threshold":             defaults.SuccessThreshold,
            "observability_window":          defaults.ObservabilityWindow,
            "minimum_request_count":         defaults.MinimumRequestCount,
            "consecutive_failure_threshold": defaults.ConsecutiveFailureThreshold,
        }
        _ = render.Render(w, r, util.NewServerResponse("Circuit breaker configuration fetched successfully", response, http.StatusOK))
        return
    }

    project := projects[0]
    config := project.Config.GetCircuitBreakerConfig()

    response := map[string]interface{}{
        "sample_rate":                   config.SampleRate,
        "error_timeout":                 config.ErrorTimeout,
        "failure_threshold":             config.FailureThreshold,
        "success_threshold":             config.SuccessThreshold,
        "observability_window":          config.ObservabilityWindow,
        "minimum_request_count":         config.MinimumRequestCount,
        "consecutive_failure_threshold": config.ConsecutiveFailureThreshold,
    }

    _ = render.Render(w, r, util.NewServerResponse("Circuit breaker configuration fetched successfully", response, http.StatusOK))
}

// UpdateOrganisationCircuitBreakerConfig updates the circuit breaker configuration for all projects in an organization (instance admin only)
func (h *Handler) UpdateOrganisationCircuitBreakerConfig(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    orgID := chi.URLParam(r, "orgID")
    if orgID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("organisation ID is required", http.StatusBadRequest))
        return
    }

    var configRequest datastore.UpdateOrganisationCircuitBreakerConfig
    err := util.ReadJSON(r, &configRequest)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    // Validate thresholds
    if configRequest.FailureThreshold > 100 {
        _ = render.Render(w, r, util.NewErrorResponse("failure_threshold must be between 0 and 100", http.StatusBadRequest))
        return
    }
    if configRequest.SuccessThreshold > 100 {
        _ = render.Render(w, r, util.NewErrorResponse("success_threshold must be between 0 and 100", http.StatusBadRequest))
        return
    }
    if configRequest.ObservabilityWindow == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("observability_window must be greater than 0", http.StatusBadRequest))
        return
    }

    projectRepo := projects.New(h.A.Logger, h.A.DB)
    projects, err := projectRepo.LoadProjects(r.Context(), &datastore.ProjectFilter{OrgID: orgID})
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if len(projects) == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("No projects found for this organization", http.StatusBadRequest))
        return
    }

    if configRequest.SampleRate == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("sample_rate must be greater than 0", http.StatusBadRequest))
        return
    }
    if configRequest.ErrorTimeout == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("error_timeout must be greater than 0", http.StatusBadRequest))
        return
    }

    // Convert to datastore model
    config := &datastore.CircuitBreakerConfiguration{
        SampleRate:                  configRequest.SampleRate,
        ErrorTimeout:                configRequest.ErrorTimeout,
        FailureThreshold:            configRequest.FailureThreshold,
        SuccessThreshold:            configRequest.SuccessThreshold,
        ObservabilityWindow:         configRequest.ObservabilityWindow,
        MinimumRequestCount:         configRequest.MinimumRequestCount,
        ConsecutiveFailureThreshold: configRequest.ConsecutiveFailureThreshold,
    }

    for _, project := range projects {
        if project.Config == nil {
            project.Config = &datastore.ProjectConfig{}
        }
        project.Config.CircuitBreaker = config
        err = projectRepo.UpdateProject(r.Context(), project)
        if err != nil {
            log.FromContext(r.Context()).WithError(err).Errorf("Failed to update circuit breaker config for project %s", project.UID)
            _ = render.Render(w, r, util.NewServiceErrResponse(err))
            return
        }
    }

    response := map[string]interface{}{
        "sample_rate":                   config.SampleRate,
        "error_timeout":                 config.ErrorTimeout,
        "failure_threshold":             config.FailureThreshold,
        "success_threshold":             config.SuccessThreshold,
        "observability_window":          config.ObservabilityWindow,
        "minimum_request_count":         config.MinimumRequestCount,
        "consecutive_failure_threshold": config.ConsecutiveFailureThreshold,
    }

    _ = render.Render(w, r, util.NewServerResponse("Circuit breaker configuration updated successfully for all projects", response, http.StatusOK))
}

// GetProjectCircuitBreakerConfig returns the circuit breaker configuration for a specific project (instance admin only)
func (h *Handler) GetProjectCircuitBreakerConfig(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    projectID := chi.URLParam(r, "projectID")
    if projectID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("project ID is required", http.StatusBadRequest))
        return
    }

    projectRepo := projects.New(h.A.Logger, h.A.DB)
    project, err := projectRepo.FetchProjectByID(r.Context(), projectID)
    if err != nil {
        if errors.Is(err, datastore.ErrProjectNotFound) {
            _ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusNotFound))
            return
        }
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    config := project.Config.GetCircuitBreakerConfig()

    response := map[string]interface{}{
        "sample_rate":                   config.SampleRate,
        "error_timeout":                 config.ErrorTimeout,
        "failure_threshold":             config.FailureThreshold,
        "success_threshold":             config.SuccessThreshold,
        "observability_window":          config.ObservabilityWindow,
        "minimum_request_count":         config.MinimumRequestCount,
        "consecutive_failure_threshold": config.ConsecutiveFailureThreshold,
    }

    _ = render.Render(w, r, util.NewServerResponse("Circuit breaker configuration fetched successfully", response, http.StatusOK))
}

// UpdateProjectCircuitBreakerConfig updates the circuit breaker configuration for a specific project (instance admin only)
func (h *Handler) UpdateProjectCircuitBreakerConfig(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    projectID := chi.URLParam(r, "projectID")
    if projectID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("project ID is required", http.StatusBadRequest))
        return
    }

    var configRequest datastore.UpdateOrganisationCircuitBreakerConfig
    err := util.ReadJSON(r, &configRequest)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    if configRequest.FailureThreshold > 100 {
        _ = render.Render(w, r, util.NewErrorResponse("failure_threshold must be between 0 and 100", http.StatusBadRequest))
        return
    }
    if configRequest.SuccessThreshold > 100 {
        _ = render.Render(w, r, util.NewErrorResponse("success_threshold must be between 0 and 100", http.StatusBadRequest))
        return
    }
    if configRequest.ObservabilityWindow == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("observability_window must be greater than 0", http.StatusBadRequest))
        return
    }

    projectRepo := projects.New(h.A.Logger, h.A.DB)
    project, err := projectRepo.FetchProjectByID(r.Context(), projectID)
    if err != nil {
        if errors.Is(err, datastore.ErrProjectNotFound) {
            _ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusNotFound))
            return
        }
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    if configRequest.SampleRate == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("sample_rate must be greater than 0", http.StatusBadRequest))
        return
    }
    if configRequest.ErrorTimeout == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("error_timeout must be greater than 0", http.StatusBadRequest))
        return
    }

    config := &datastore.CircuitBreakerConfiguration{
        SampleRate:                  configRequest.SampleRate,
        ErrorTimeout:                configRequest.ErrorTimeout,
        FailureThreshold:            configRequest.FailureThreshold,
        SuccessThreshold:            configRequest.SuccessThreshold,
        ObservabilityWindow:         configRequest.ObservabilityWindow,
        MinimumRequestCount:         configRequest.MinimumRequestCount,
        ConsecutiveFailureThreshold: configRequest.ConsecutiveFailureThreshold,
    }

    if project.Config == nil {
        project.Config = &datastore.ProjectConfig{}
    }
    project.Config.CircuitBreaker = config
    err = projectRepo.UpdateProject(r.Context(), project)
    if err != nil {
        log.FromContext(r.Context()).WithError(err).Errorf("Failed to update circuit breaker config for project %s", project.UID)
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    response := map[string]interface{}{
        "sample_rate":                   config.SampleRate,
        "error_timeout":                 config.ErrorTimeout,
        "failure_threshold":             config.FailureThreshold,
        "success_threshold":             config.SuccessThreshold,
        "observability_window":          config.ObservabilityWindow,
        "minimum_request_count":         config.MinimumRequestCount,
        "consecutive_failure_threshold": config.ConsecutiveFailureThreshold,
    }

    _ = render.Render(w, r, util.NewServerResponse("Circuit breaker configuration updated successfully", response, http.StatusOK))
}

// UpdateFeatureFlag updates the default enabled state or allow_override of a system feature flag (instance admin only)
func (h *Handler) UpdateFeatureFlag(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    featureKey := chi.URLParam(r, "featureKey")
    if featureKey == "" {
        _ = render.Render(w, r, util.NewErrorResponse("feature key is required", http.StatusBadRequest))
        return
    }

    var updateRequest models.UpdateFeatureFlagRequest
    err := util.ReadJSON(r, &updateRequest)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    // Fetch the feature flag
    featureFlag, err := postgres.FetchFeatureFlagByKey(r.Context(), h.A.DB, featureKey)
    if err != nil {
        if errors.Is(err, postgres.ErrFeatureFlagNotFound) {
            _ = render.Render(w, r, util.NewErrorResponse("Feature flag not found: "+featureKey, http.StatusBadRequest))
            return
        }
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    // Update enabled state if provided
    if updateRequest.Enabled != nil {
        err = postgres.UpdateFeatureFlag(r.Context(), h.A.DB, featureFlag.UID, *updateRequest.Enabled)
        if err != nil {
            _ = render.Render(w, r, util.NewServiceErrResponse(err))
            return
        }
    }

    updatedFlag, err := postgres.FetchFeatureFlagByID(r.Context(), h.A.DB, featureFlag.UID)
    if err != nil {
        _ = render.Render(w, r, util.NewServiceErrResponse(err))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Feature flag updated successfully", updatedFlag, http.StatusOK))
}

// RetryEventDeliveries retries event deliveries with a particular status in a timeframe (instance admin only)
func (h *Handler) RetryEventDeliveries(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    var retryRequest models.RetryEventDeliveriesRequest
    err := util.ReadJSON(r, &retryRequest)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
        return
    }

    if retryRequest.Status == "" {
        _ = render.Render(w, r, util.NewErrorResponse("status is required", http.StatusBadRequest))
        return
    }
    if retryRequest.Time == "" {
        _ = render.Render(w, r, util.NewErrorResponse("time is required", http.StatusBadRequest))
        return
    }

    // Parse status(es) - can be single status or comma-separated multiple statuses
    statusStrings := strings.Split(retryRequest.Status, ",")
    statuses := make([]datastore.EventDeliveryStatus, 0, len(statusStrings))

    for _, statusStr := range statusStrings {
        statusStr = strings.TrimSpace(statusStr)
        if statusStr == "" {
            continue
        }
        status := datastore.EventDeliveryStatus(statusStr)
        if !status.IsValid() {
            _ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("invalid status '%s': must be one of Scheduled, Processing, Retry, Failure, Success, Discarded", statusStr), http.StatusBadRequest))
            return
        }
        statuses = append(statuses, status)
    }

    if len(statuses) == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("at least one valid status is required", http.StatusBadRequest))
        return
    }

    _, err = time.ParseDuration(retryRequest.Time)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("invalid time format: must be a valid duration (e.g., 1h, 30m, 5h)", http.StatusBadRequest))
        return
    }

    if h.A.Queue == nil {
        _ = render.Render(w, r, util.NewErrorResponse("Queue not configured: retry is only available with Redis queue", http.StatusBadRequest))
        return
    }

    if h.A.Redis == nil {
        _ = render.Render(w, r, util.NewErrorResponse("Redis not configured: batch tracking requires Redis", http.StatusBadRequest))
        return
    }

    // Generate batch ID and create tracker
    tracker := batch_tracker.NewBatchTracker(h.A.Redis)
    batchID := tracker.GenerateBatchID()

    // Run retry in background goroutine - don't block the response
    go func() {
        task.RetryEventDeliveriesWithTracker(h.A.DB, h.A.Queue, statuses, retryRequest.Time, retryRequest.EventID, batchID, tracker)
    }()

    _ = render.Render(w, r, util.NewServerResponse("Event deliveries retry initiated successfully", map[string]interface{}{
        "batch_id": batchID,
        "status":   retryRequest.Status,
        "time":     retryRequest.Time,
        "event_id": retryRequest.EventID,
        "message":  "Retry process started in background",
    }, http.StatusOK))
}

// CountRetryEventDeliveries counts event deliveries with a particular status in a timeframe (instance admin only)
func (h *Handler) CountRetryEventDeliveries(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    status := r.URL.Query().Get("status")
    timeStr := r.URL.Query().Get("time")
    eventID := r.URL.Query().Get("event_id")

    if status == "" {
        _ = render.Render(w, r, util.NewErrorResponse("status is required", http.StatusBadRequest))
        return
    }
    if timeStr == "" {
        _ = render.Render(w, r, util.NewErrorResponse("time is required", http.StatusBadRequest))
        return
    }

    // Parse status(es) - can be single status or comma-separated multiple statuses
    statusStrings := strings.Split(status, ",")
    statuses := make([]datastore.EventDeliveryStatus, 0, len(statusStrings))

    for _, statusStr := range statusStrings {
        statusStr = strings.TrimSpace(statusStr)
        if statusStr == "" {
            continue
        }
        deliveryStatus := datastore.EventDeliveryStatus(statusStr)
        if !deliveryStatus.IsValid() {
            _ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("invalid status '%s': must be one of Scheduled, Processing, Retry, Failure, Success, Discarded", statusStr), http.StatusBadRequest))
            return
        }
        statuses = append(statuses, deliveryStatus)
    }

    if len(statuses) == 0 {
        _ = render.Render(w, r, util.NewErrorResponse("at least one valid status is required", http.StatusBadRequest))
        return
    }

    duration, err := time.ParseDuration(timeStr)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("invalid time format: must be a valid duration (e.g., 1h, 30m, 5h)", http.StatusBadRequest))
        return
    }

    now := time.Now()
    then := now.Add(-duration)

    searchParams := datastore.SearchParams{
        CreatedAtStart: then.Unix(),
        CreatedAtEnd:   now.Unix(),
    }

    eventDeliveryRepo := postgres.NewEventDeliveryRepo(h.A.DB)

    // Count across all statuses
    var totalCount int64
    if eventID != "" {
        // If eventID is provided, use CountEventDeliveries which supports eventID filter
        // This method accepts multiple statuses, so we can pass all at once
        totalCount, err = eventDeliveryRepo.CountEventDeliveries(r.Context(), "", []string{}, eventID, statuses, searchParams)
        if err != nil {
            log.FromContext(r.Context()).WithError(err).Error("failed to count event deliveries")
            _ = render.Render(w, r, util.NewErrorResponse("failed to count event deliveries", http.StatusInternalServerError))
            return
        }
    } else {
        // Otherwise, count each status separately and sum them
        for _, deliveryStatus := range statuses {
            count, err := eventDeliveryRepo.CountDeliveriesByStatus(r.Context(), "", deliveryStatus, searchParams)
            if err != nil {
                log.FromContext(r.Context()).WithError(err).Error("failed to count event deliveries")
                _ = render.Render(w, r, util.NewErrorResponse("failed to count event deliveries", http.StatusInternalServerError))
                return
            }
            totalCount += count
        }
    }

    _ = render.Render(w, r, util.NewServerResponse("Event deliveries count successful", map[string]interface{}{
        "num": totalCount,
    }, http.StatusOK))
}

// GetBatchProgress retrieves the progress of a batch retry operation (instance admin only)
func (h *Handler) GetBatchProgress(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    batchID := chi.URLParam(r, "batchID")
    if batchID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("batch_id is required", http.StatusBadRequest))
        return
    }

    if h.A.Redis == nil {
        _ = render.Render(w, r, util.NewErrorResponse("Redis not configured: batch tracking requires Redis", http.StatusInternalServerError))
        return
    }

    tracker := batch_tracker.NewBatchTracker(h.A.Redis)

    // Sync counters before retrieving to get latest progress
    if err := tracker.SyncCounters(r.Context(), batchID); err != nil {
        log.FromContext(r.Context()).WithError(err).Warn("failed to sync batch counters, continuing with cached data")
    }

    progress, err := tracker.GetBatch(r.Context(), batchID)
    if err != nil {
        _ = render.Render(w, r, util.NewErrorResponse("batch not found: "+err.Error(), http.StatusNotFound))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Batch progress retrieved successfully", progress, http.StatusOK))
}

// ListBatchProgress retrieves all batch retry operations (instance admin only)
func (h *Handler) ListBatchProgress(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    if h.A.Redis == nil {
        _ = render.Render(w, r, util.NewErrorResponse("Redis not configured: batch tracking requires Redis", http.StatusInternalServerError))
        return
    }

    tracker := batch_tracker.NewBatchTracker(h.A.Redis)

    batches, err := tracker.ListBatches(r.Context())
    if err != nil {
        log.FromContext(r.Context()).WithError(err).Error("failed to list batches")
        _ = render.Render(w, r, util.NewErrorResponse("failed to list batches: "+err.Error(), http.StatusInternalServerError))
        return
    }

    // Sync counters for all batches to get latest progress
    for _, batch := range batches {
        if err := tracker.SyncCounters(r.Context(), batch.BatchID); err != nil {
            log.FromContext(r.Context()).WithError(err).Warnf("failed to sync counters for batch %s", batch.BatchID)
        }
        // Re-fetch to get synced data
        if syncedBatch, err := tracker.GetBatch(r.Context(), batch.BatchID); err == nil {
            *batch = *syncedBatch
        }
    }

    _ = render.Render(w, r, util.NewServerResponse("Batches retrieved successfully", batches, http.StatusOK))
}

// DeleteBatchProgress deletes a batch from Redis (instance admin only)
func (h *Handler) DeleteBatchProgress(w http.ResponseWriter, r *http.Request) {
    if !h.isInstanceAdmin(r) {
        _ = render.Render(w, r, util.NewErrorResponse("Unauthorized: instance admin access required", http.StatusForbidden))
        return
    }

    batchID := chi.URLParam(r, "batchID")
    if batchID == "" {
        _ = render.Render(w, r, util.NewErrorResponse("batch_id is required", http.StatusBadRequest))
        return
    }

    if h.A.Redis == nil {
        _ = render.Render(w, r, util.NewErrorResponse("Redis not configured: batch tracking requires Redis", http.StatusInternalServerError))
        return
    }

    tracker := batch_tracker.NewBatchTracker(h.A.Redis)

    if err := tracker.DeleteBatch(r.Context(), batchID); err != nil {
        log.FromContext(r.Context()).WithError(err).Error("failed to delete batch")
        _ = render.Render(w, r, util.NewErrorResponse("failed to delete batch: "+err.Error(), http.StatusInternalServerError))
        return
    }

    _ = render.Render(w, r, util.NewServerResponse("Batch deleted successfully", nil, http.StatusOK))
}
