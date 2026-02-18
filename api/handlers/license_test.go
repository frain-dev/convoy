package handlers

import (
	"context"
	"encoding/json"
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
	"github.com/frain-dev/convoy/pkg/log"
)

func TestGetLicenseFeatures_InstanceLevel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLicenser := mocks.NewMockLicenser(ctrl)
	instanceFeatures := json.RawMessage(`{"EnterpriseSSO":true,"PortalLinks":true}`)

	handler := &Handler{
		A: &types.APIOptions{
			Licenser:     mockLicenser,
			OrgRepo:      mocks.NewMockOrganisationRepository(ctrl),
			ProjectRepo:  mocks.NewMockProjectRepository(ctrl),
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
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{Enabled: true}},
			BillingClient: &billing.MockBillingClient{},
			Logger:        log.FromContext(context.Background()),
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
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{Enabled: true}},
			BillingClient: &billing.MockBillingClient{},
			Logger:        log.FromContext(context.Background()),
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

func TestGetLicenseFeatures_OrgLevel_BillingRequiredWhenNoLicenseData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-no-license"
	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)

	handler := &Handler{
		A: &types.APIOptions{
			Cfg:           config.Configuration{Billing: config.BillingConfiguration{Enabled: true}},
			BillingClient: &billing.MockBillingClient{},
			Logger:        log.FromContext(context.Background()),
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
