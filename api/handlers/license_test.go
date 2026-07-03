package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// asOrgMember attaches an authenticated user to the request context and registers the
// membership lookup that the org-scoped enrichment gate in GetLicenseFeatures performs.
func asOrgMember(req *http.Request, mockOrgMemberRepo *mocks.MockOrganisationMemberRepository, userID, orgID string) *http.Request {
	authUser := &auth.AuthenticatedUser{User: &datastore.User{UID: userID}}
	mockOrgMemberRepo.EXPECT().
		FetchOrganisationMemberByUserID(gomock.Any(), userID, orgID).
		Return(&datastore.OrganisationMember{UID: "member-1", UserID: userID, OrganisationID: orgID}, nil)
	return req.WithContext(context.WithValue(req.Context(), convoy.AuthUserCtx, authUser))
}

type failingGetOrganisationLicenseClient struct {
	*billing.MockBillingClient
}

func (c *failingGetOrganisationLicenseClient) GetOrganisationLicense(_ context.Context, _ string) (*billing.Response[billing.OrganisationLicense], error) {
	return nil, errors.New("billing temporarily unavailable")
}

func TestGetLicenseFeatures_InstanceLevel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLicenser := mocks.NewMockLicenser(ctrl)
	instanceFeatures := json.RawMessage(`{"EnterpriseSSO":true,"PortalLinks":true}`)

	handler := &Handler{
		A: &types.APIOptions{
			Licenser:    mockLicenser,
			OrgRepo:     mocks.NewMockOrganisationRepository(ctrl),
			ProjectRepo: mocks.NewMockProjectRepository(ctrl),
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/license/features", nil)
	w := httptest.NewRecorder()

	mockLicenser.EXPECT().
		FeatureListJSON(gomock.Any()).
		Return(instanceFeatures, nil)

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, instanceFeatures, resp.Data)
}

func TestGetPortalLicenseFeatures_SelfHosted_UsesDeploymentLicenser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLicenser := mocks.NewMockLicenser(ctrl)
	deploymentFeatures := json.RawMessage(`{"PortalLinks":true,"AdvancedSubscriptions":false}`)

	// No billing config and no billing client: self-hosted / non-org-billing.
	handler := &Handler{
		A: &types.APIOptions{
			Licenser: mockLicenser,
		},
	}

	mockLicenser.EXPECT().
		FeatureListJSON(gomock.Any()).
		Return(deploymentFeatures, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal-api/license/features", nil)
	w := httptest.NewRecorder()

	handler.GetPortalLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, deploymentFeatures, resp.Data)
}

// TestGetPortalLicenseFeatures_IgnoresClientOrgID proves the portal handler never
// branches into org-scoped features off a client-supplied orgID. Unlike
// GetLicenseFeatures, the org is derived from the portal token only, so a portal
// token cannot read another org's plan by passing ?orgID=. In self-hosted mode it
// must still resolve to the deployment licenser and never touch the org repo.
func TestGetPortalLicenseFeatures_IgnoresClientOrgID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLicenser := mocks.NewMockLicenser(ctrl)
	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	deploymentFeatures := json.RawMessage(`{"PortalLinks":true}`)

	handler := &Handler{
		A: &types.APIOptions{
			Licenser: mockLicenser,
			OrgRepo:  mockOrgRepo,
		},
	}

	// Deployment licenser is consulted; org repo is never called with the
	// attacker-supplied orgID (no EXPECT means a call would fail the test).
	mockLicenser.EXPECT().
		FeatureListJSON(gomock.Any()).
		Return(deploymentFeatures, nil)

	req := httptest.NewRequest(http.MethodGet, "/portal-api/license/features?orgID=attacker-org", nil)
	w := httptest.NewRecorder()

	handler.GetPortalLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, deploymentFeatures, resp.Data)
}

func TestGetLicenseFeatures_OrgLevel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-123"
	entitlements := map[string]interface{}{
		"enterprise_sso": true,
		"portal_links":   true,
		"user_limit":     int64(10),
	}
	payload := &license.LicenseDataPayload{Key: "lk", Entitlements: entitlements}
	encrypted, err := license.EncryptLicenseData(orgID, payload)
	require.NoError(t, err)

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().CountOrganisationMembers(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &failingGetOrganisationLicenseClient{MockBillingClient: &billing.MockBillingClient{}},
			Logger:        log.New("convoy", log.LevelInfo),
			OrgRepo:       mockOrgRepo,
			OrgMemberRepo: mockOrgMemberRepo,
			ProjectRepo:   mockProjectRepo,
		},
	}

	mockOrgMemberRepo.EXPECT().CountUserOrganisations(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: encrypted}, nil)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
	req = asOrgMember(req, mockOrgMemberRepo, "user-1", orgID)
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	var data map[string]interface{}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	require.True(t, data["EnterpriseSSO"].(bool))
	require.True(t, data["PortalLinks"].(bool))
	ul, ok := data["user_limit"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(10), ul["limit"])
}

func TestGetLicenseFeatures_OrgLevel_Header(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-456"
	entitlements := map[string]interface{}{"portal_links": true}
	payload := &license.LicenseDataPayload{Key: "k", Entitlements: entitlements}
	encrypted, err := license.EncryptLicenseData(orgID, payload)
	require.NoError(t, err)

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().CountOrganisationMembers(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &failingGetOrganisationLicenseClient{MockBillingClient: &billing.MockBillingClient{}},
			Logger:        log.New("convoy", log.LevelInfo),
			OrgRepo:       mockOrgRepo,
			OrgMemberRepo: mockOrgMemberRepo,
			ProjectRepo:   mockProjectRepo,
		},
	}

	mockOrgMemberRepo.EXPECT().CountUserOrganisations(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: encrypted}, nil)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/license/features", nil)
	req.Header.Set("X-Organisation-Id", orgID)
	req = asOrgMember(req, mockOrgMemberRepo, "user-1", orgID)
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	var data map[string]interface{}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	require.True(t, data["PortalLinks"].(bool))
}

func TestGetLicenseFeatures_OrgLevel_BillingRequiredWhenBillingReturnsNoLicenseKey(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-stale-license"
	entitlements := map[string]interface{}{
		"enterprise_sso": true,
		"portal_links":   true,
		"user_limit":     int64(10),
	}
	payload := &license.LicenseDataPayload{Key: "stale-key", Entitlements: entitlements}
	encrypted, err := license.EncryptLicenseData(orgID, payload)
	require.NoError(t, err)

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().CountOrganisationMembers(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &billing.MockBillingClient{},
			Logger:        log.New("convoy", log.LevelInfo),
			OrgRepo:       mockOrgRepo,
			OrgMemberRepo: mockOrgMemberRepo,
			ProjectRepo:   mockProjectRepo,
		},
	}

	mockOrgMemberRepo.EXPECT().CountUserOrganisations(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	// Fetched twice: once by serveOrgLicenseFeatures, once by the guarded clear
	// (services.ClearOrgLicenseData re-reads before deciding to persist empty).
	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: encrypted}, nil).
		Times(2)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)
	// Stale authoritative entitlements (user_limit present, not a provisional cap)
	// must still be cleared when billing definitively reports no license.
	mockOrgRepo.EXPECT().
		UpdateOrganisationLicenseData(gomock.Any(), orgID, "").
		Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
	req = asOrgMember(req, mockOrgMemberRepo, "user-1", orgID)
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	var data map[string]interface{}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	require.False(t, data["EnterpriseSSO"].(bool))
	require.False(t, data["PortalLinks"].(bool))
	ul, ok := data["user_limit"].(map[string]interface{})
	require.True(t, ok)
	require.False(t, ul["allowed"].(bool))
	require.False(t, ul["available"].(bool))

}

// TestGetLicenseFeatures_OrgLevel_NoLicenseKeyPreservesProvisionalCap proves a routine
// post-trial /license/features call while billing has not yet exposed the trial license
// (no key) must NOT erase the marked provisional trial seed, and must serve the seeded
// trial caps in the feature list (display parity: the UI sees the same org/user/project
// caps the backend enforces, not the billing-required list). The guarded clear
// (services.ClearOrgLicenseData) skips the empty write on the marker, so the org stays
// capped; no UpdateOrganisationLicenseData expectation means any write fails the test.
func TestGetLicenseFeatures_OrgLevel_NoLicenseKeyPreservesProvisionalCap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-provisional-preserved"
	provisional, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{
		Provisional: true,
		Entitlements: map[string]interface{}{
			"daily_event_limit": 100, "org_limit": 1, "user_limit": 1, "project_limit": 1,
		},
	})
	require.NoError(t, err)

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().CountOrganisationMembers(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &billing.MockBillingClient{}, // no license key: billing answers definitively
			Logger:        log.New("convoy", log.LevelInfo),
			OrgRepo:       mockOrgRepo,
			OrgMemberRepo: mockOrgMemberRepo,
			ProjectRepo:   mockProjectRepo,
		},
	}

	mockOrgMemberRepo.EXPECT().CountUserOrganisations(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	// Fetched by serveOrgLicenseFeatures and re-read by the guarded clear.
	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: provisional}, nil).
		Times(2)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
	req = asOrgMember(req, mockOrgMemberRepo, "user-1", orgID)
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	// The seeded trial caps must be visible, not the billing-required zero limits.
	for _, key := range []string{"org_limit", "user_limit", "project_limit"} {
		block, ok := data[key].(map[string]interface{})
		require.True(t, ok, "feature list must carry %s", key)
		require.Equal(t, float64(1), block["limit"], "%s must show the seeded trial cap", key)
		require.True(t, block["available"].(bool), "%s must be available (not billing-required)", key)
		require.True(t, block["allowed"].(bool), "%s under cap must be allowed (usage counts are 0)", key)
	}
	require.False(t, data["EnterpriseSSO"].(bool), "boolean features absent from the seed stay off")
}

func TestGetLicenseFeatures_OrgLevel_BillingRequiredWhenStaleLicenseClearFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-stale-license-clear-fails"
	entitlements := map[string]interface{}{
		"enterprise_sso": true,
		"portal_links":   true,
	}
	payload := &license.LicenseDataPayload{Key: "stale-key", Entitlements: entitlements}
	encrypted, err := license.EncryptLicenseData(orgID, payload)
	require.NoError(t, err)

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().CountOrganisationMembers(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &billing.MockBillingClient{},
			Logger:        log.New("convoy", log.LevelInfo),
			OrgRepo:       mockOrgRepo,
			OrgMemberRepo: mockOrgMemberRepo,
			ProjectRepo:   mockProjectRepo,
		},
	}

	mockOrgMemberRepo.EXPECT().CountUserOrganisations(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	// Fetched twice: serveOrgLicenseFeatures + the guarded clear's re-read.
	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: encrypted}, nil).
		Times(2)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)
	mockOrgRepo.EXPECT().
		UpdateOrganisationLicenseData(gomock.Any(), orgID, "").
		Return(errors.New("db unavailable"))

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
	req = asOrgMember(req, mockOrgMemberRepo, "user-1", orgID)
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	var data map[string]interface{}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	require.False(t, data["EnterpriseSSO"].(bool))
	require.False(t, data["PortalLinks"].(bool))
}

// TestGetLicenseFeatures_GuestWithOrgID_ServesInstanceFeatures proves the org-scoped
// enrichment gate: an unauthenticated caller supplying an orgID (guessed or known ULID)
// must get the instance-level feature list and never trigger org repo or member count
// lookups (no EXPECT on those mocks means any call fails the test).
func TestGetLicenseFeatures_GuestWithOrgID_ServesInstanceFeatures(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLicenser := mocks.NewMockLicenser(ctrl)
	instanceFeatures := json.RawMessage(`{"PortalLinks":true}`)

	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &billing.MockBillingClient{},
			Logger:        log.New("convoy", log.LevelInfo),
			Licenser:      mockLicenser,
			OrgRepo:       mocks.NewMockOrganisationRepository(ctrl),
			OrgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
			ProjectRepo:   mocks.NewMockProjectRepository(ctrl),
		},
	}

	mockLicenser.EXPECT().
		FeatureListJSON(gomock.Any()).
		Return(instanceFeatures, nil)

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID=guessed-org-ulid", nil)
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, instanceFeatures, resp.Data)
	require.NotContains(t, string(resp.Data), "user_limit")
}

// TestGetLicenseFeatures_AuthedMember_GetsMemberCount proves the legitimate dashboard
// path still works: an authenticated member of the requested org gets org-scoped
// enrichment including the resolved member count and org count.
func TestGetLicenseFeatures_AuthedMember_GetsMemberCount(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-member-counts"
	entitlements := map[string]interface{}{
		"portal_links": true,
		"user_limit":   int64(10),
		"org_limit":    int64(5),
	}
	payload := &license.LicenseDataPayload{Key: "lk", Entitlements: entitlements}
	encrypted, err := license.EncryptLicenseData(orgID, payload)
	require.NoError(t, err)

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &failingGetOrganisationLicenseClient{MockBillingClient: &billing.MockBillingClient{}},
			Logger:        log.New("convoy", log.LevelInfo),
			OrgRepo:       mockOrgRepo,
			OrgMemberRepo: mockOrgMemberRepo,
			ProjectRepo:   mockProjectRepo,
		},
	}

	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: encrypted}, nil)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)
	mockOrgMemberRepo.EXPECT().
		CountOrganisationMembers(gomock.Any(), orgID).
		Return(int64(3), nil)
	mockOrgMemberRepo.EXPECT().
		CountUserOrganisations(gomock.Any(), "user-1", "").
		Return(int64(1), nil)

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
	req = asOrgMember(req, mockOrgMemberRepo, "user-1", orgID)
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	var data map[string]interface{}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	ul, ok := data["user_limit"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(3), ul["current"])
	require.False(t, ul["limit_reached"].(bool))
	ol, ok := data["org_limit"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, float64(1), ol["current"])
}

// TestGetLicenseFeatures_AuthedNonMember_ServesInstanceFeatures proves an authenticated
// user who is not a member of the requested org is treated like a guest: instance-level
// features, no member count, no org repo lookups (fail closed on the membership check).
func TestGetLicenseFeatures_AuthedNonMember_ServesInstanceFeatures(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-not-mine"
	mockLicenser := mocks.NewMockLicenser(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	instanceFeatures := json.RawMessage(`{"PortalLinks":true}`)

	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &billing.MockBillingClient{},
			Logger:        log.New("convoy", log.LevelInfo),
			Licenser:      mockLicenser,
			OrgRepo:       mocks.NewMockOrganisationRepository(ctrl),
			OrgMemberRepo: mockOrgMemberRepo,
			ProjectRepo:   mocks.NewMockProjectRepository(ctrl),
		},
	}

	mockOrgMemberRepo.EXPECT().
		FetchOrganisationMemberByUserID(gomock.Any(), "user-1", orgID).
		Return(nil, errors.New("organisation member not found"))
	mockLicenser.EXPECT().
		FeatureListJSON(gomock.Any()).
		Return(instanceFeatures, nil)

	authUser := &auth.AuthenticatedUser{User: &datastore.User{UID: "user-1"}}
	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
	req = req.WithContext(context.WithValue(req.Context(), convoy.AuthUserCtx, authUser))
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, instanceFeatures, resp.Data)
	require.NotContains(t, string(resp.Data), "user_limit")
}

func TestGetLicenseFeatures_OrgLevel_BillingRequiredWhenNoLicenseData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-no-license"
	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().CountOrganisationMembers(gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()

	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "test-key"}},
			BillingClient: &billing.MockBillingClient{},
			Logger:        log.New("convoy", log.LevelInfo),
			OrgRepo:       mockOrgRepo,
			OrgMemberRepo: mockOrgMemberRepo,
			ProjectRepo:   mockProjectRepo,
		},
	}

	mockOrgMemberRepo.EXPECT().CountUserOrganisations(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).AnyTimes()
	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: ""}, nil)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
	req = asOrgMember(req, mockOrgMemberRepo, "user-1", orgID)
	w := httptest.NewRecorder()

	handler.GetLicenseFeatures(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data json.RawMessage `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	var data map[string]interface{}
	err = json.Unmarshal(resp.Data, &data)
	require.NoError(t, err)
	pl, ok := data["project_limit"].(map[string]interface{})
	require.True(t, ok)
	require.False(t, pl["allowed"].(bool))
	require.False(t, pl["available"].(bool))

}
