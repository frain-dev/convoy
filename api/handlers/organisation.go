package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	fflag "github.com/frain-dev/convoy/internal/pkg/fflag"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
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

	organisations, paginationData, err := postgres.NewOrgMemberRepo(h.A.DB).LoadUserOrganisationsPaged(r.Context(), user.UID, pageable)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to fetch user organisations")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
		models.PagedResponse{Content: &organisations, Pagination: &paginationData}, http.StatusOK))
}

func (h *Handler) CreateOrganisation(w http.ResponseWriter, r *http.Request) {
	var newOrg models.Organisation
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

	co := services.CreateOrganisationService{
		OrgRepo:       postgres.NewOrgRepo(h.A.DB),
		OrgMemberRepo: postgres.NewOrgMemberRepo(h.A.DB),
		NewOrg:        &newOrg,
		User:          user,
		Licenser:      h.A.Licenser,
	}

	organisation, err := co.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation created successfully", organisation, http.StatusCreated))
}

func (h *Handler) UpdateOrganisation(w http.ResponseWriter, r *http.Request) {
	var orgUpdate models.Organisation
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

	us := services.UpdateOrganisationService{
		OrgRepo:       postgres.NewOrgRepo(h.A.DB),
		OrgMemberRepo: postgres.NewOrgMemberRepo(h.A.DB),
		Org:           org,
		Update:        &orgUpdate,
	}

	org, err = us.Run(r.Context())
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation updated successfully", org, http.StatusAccepted))
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

	err = postgres.NewOrgRepo(h.A.DB).DeleteOrganisation(r.Context(), org.UID)
	if err != nil {
		log.FromContext(r.Context()).WithError(err).Error("failed to delete organisation")
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisation deleted successfully", nil, http.StatusOK))
}

func (h *Handler) UpdateOrganisationFeatureFlags(w http.ResponseWriter, r *http.Request) {
	var featureFlagsUpdate models.UpdateOrganisationFeatureFlags
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

	member, err := postgres.NewOrgMemberRepo(h.A.DB).FetchInstanceAdminByUserID(r.Context(), user.UID)
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

	orgRepo := postgres.NewOrgRepo(h.A.DB)
	var organisations []datastore.Organisation
	var paginationData datastore.PaginationData
	var err error

	if search != "" {
		organisations, paginationData, err = orgRepo.LoadOrganisationsPagedWithSearch(r.Context(), pageable, search)
	} else {
		organisations, paginationData, err = orgRepo.LoadOrganisationsPaged(r.Context(), pageable)
	}

	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Organisations fetched successfully",
		models.PagedResponse{Content: &organisations, Pagination: &paginationData}, http.StatusOK))
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

	var overrideRequest models.UpdateOrganisationOverride
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
