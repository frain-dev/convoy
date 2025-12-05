package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/policies"
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

	overrides, err := postgres.LoadFeatureFlagOverridesByOwner(r.Context(), h.A.DB, "organisation", org.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return
	}

	overrideMap := make(map[string]*datastore.FeatureFlagOverride)
	for i := range overrides {
		overrideMap[overrides[i].FeatureFlagID] = &overrides[i]
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
		featureFlag, err := postgres.FetchFeatureFlagByKey(r.Context(), h.A.DB, string(featureKey))
		if err != nil {
			log.FromContext(r.Context()).Warnf("Feature flag not found in database: %s, error: %v", string(featureKey), err)
			continue
		}

		enabled := featureFlag.Enabled
		if override, ok := overrideMap[featureFlag.UID]; ok {
			enabled = override.Enabled
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

	// Get all system feature flags
	allFeatureFlags := []fflag.FeatureFlagKey{
		fflag.IpRules,
		fflag.Prometheus,
		fflag.CircuitBreaker,
		fflag.FullTextSearch,
		fflag.RetentionPolicy,
		fflag.ReadReplicas,
		fflag.CredentialEncryption,
		fflag.MTLS,
		fflag.OAuthTokenExchange,
	}

	// Build response map using CanAccessOrgFeature for consistency
	featureFlags := make(map[string]bool)

	for _, featureKey := range allFeatureFlags {
		enabled := h.A.FFlag.CanAccessOrgFeature(
			r.Context(), featureKey, h.A.FeatureFlagFetcher, org.UID)
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

	featureFlag, err := postgres.FetchFeatureFlagByKey(r.Context(), h.A.DB, featureKey)
	if err != nil {
		if errors.Is(err, postgres.ErrFeatureFlagNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("Feature flag not found: "+featureKey, http.StatusBadRequest))
			return err
		}
		_ = render.Render(w, r, util.NewServiceErrResponse(err))
		return err
	}

	if !featureFlag.AllowOverride {
		_ = render.Render(w, r, util.NewErrorResponse(
			"Feature flag "+featureKey+" does not allow overrides", http.StatusBadRequest))
		return ErrOverrideNotAllowed
	}

	if !h.isEarlyAdopterFeatureLicensed(flagKey) {
		_ = render.Render(w, r, util.NewErrorResponse(
			"Feature flag "+featureKey+" is not available in your license plan", http.StatusForbidden))
		return ErrNotLicensed
	}

	override := &datastore.FeatureFlagOverride{
		FeatureFlagID: featureFlag.UID,
		OwnerType:     "organisation",
		OwnerID:       org.UID,
		Enabled:       enabled,
		EnabledBy:     null.StringFrom(user.UID),
	}

	if enabled {
		override.EnabledAt = null.TimeFrom(time.Now())
	}

	err = postgres.UpsertFeatureFlagOverride(r.Context(), h.A.DB, override)
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
