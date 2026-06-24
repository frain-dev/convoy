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

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

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

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
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

	req := httptest.NewRequest(http.MethodGet, "/license/features", nil)
	req.Header.Set("X-Organisation-Id", orgID)
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

	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: encrypted}, nil)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)
	mockOrgRepo.EXPECT().
		UpdateOrganisationLicenseData(gomock.Any(), orgID, "").
		Return(nil)

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
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

	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: encrypted}, nil)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)
	mockOrgRepo.EXPECT().
		UpdateOrganisationLicenseData(gomock.Any(), orgID, "").
		Return(errors.New("db unavailable"))

	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
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

func TestGetLicenseFeatures_OrgLevel_BillingRequiredWhenNoLicenseData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-no-license"
	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)

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

	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: ""}, nil)
	mockProjectRepo.EXPECT().
		LoadProjects(gomock.Any(), gomock.Any()).
		Return([]*datastore.Project{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/license/features?orgID="+orgID, nil)
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
