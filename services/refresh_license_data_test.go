package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// newLicenseServer returns a license-service stub that answers ValidateLicense with the
// given entitlements (array form), so RefreshLicenseDataForOrg sees a real refresh
// payload without reaching the billing service.
func newLicenseServer(t *testing.T, entitlements map[string]interface{}) *httptest.Server {
	t.Helper()
	items := make([]licensesvc.EntitlementItem, 0, len(entitlements))
	for k, v := range entitlements {
		items = append(items, licensesvc.EntitlementItem{Key: k, Value: v})
	}
	raw, err := json.Marshal(items)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := licensesvc.LicenseValidationResponse{
			Status: true,
			Data: &licensesvc.LicenseValidationData{
				Valid:        true,
				Status:       "active",
				Entitlements: raw,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestRefreshLicenseDataForOrg_ProvisionalCapGuard covers the shared-writer invariant:
// a marked provisional trial seed's daily_event_limit may only be replaced by a
// COMPLETE authoritative payload that itself carries daily_event_limit. A refresh
// missing it (billing fail-open / stale) must preserve the cap (fail closed).
func TestRefreshLicenseDataForOrg_ProvisionalCapGuard(t *testing.T) {
	encrypt := func(t *testing.T, orgID string, payload *license.LicenseDataPayload) string {
		t.Helper()
		enc, err := license.EncryptLicenseData(orgID, payload)
		require.NoError(t, err)
		return enc
	}

	newDeps := func(orgRepo datastore.OrganisationRepository) RefreshLicenseDataDeps {
		return RefreshLicenseDataDeps{
			OrgRepo:       orgRepo,
			BillingClient: &billing.MockBillingClient{GetOrganisationLicenseKey: "lk-test"},
			Logger:        log.New("convoy", log.LevelInfo),
		}
	}

	t.Run("preserves_provisional_cap_when_refresh_lacks_daily_event_limit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgID := "org-refresh-provisional"
		provisional := encrypt(t, orgID, &license.LicenseDataPayload{
			Provisional:  true,
			Entitlements: map[string]interface{}{"daily_event_limit": 100, "org_limit": 1, "user_limit": 1, "project_limit": 1},
		})
		// Billing fail-open / stale: authoritative-shaped payload WITHOUT the event cap.
		srv := newLicenseServer(t, map[string]interface{}{"project_limit": 5})
		licClient := licensesvc.NewClient(licensesvc.Config{Host: srv.URL, RetryCount: 0})

		orgRepo := mocks.NewMockOrganisationRepository(ctrl)
		// Fresh re-read at decision time returns the provisional seed.
		orgRepo.EXPECT().
			FetchOrganisationByID(gomock.Any(), orgID).
			Return(&datastore.Organisation{UID: orgID, LicenseData: provisional}, nil)
		// No UpdateOrganisationLicenseData expectation: any write fails the test.

		RefreshLicenseDataForOrg(context.Background(), datastore.Organisation{UID: orgID, LicenseData: provisional}, "", true, newDeps(orgRepo), licClient)
	})

	t.Run("replaces_provisional_cap_when_refresh_carries_daily_event_limit", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgID := "org-refresh-authoritative"
		provisional := encrypt(t, orgID, &license.LicenseDataPayload{
			Provisional:  true,
			Entitlements: map[string]interface{}{"daily_event_limit": 100},
		})
		// Authoritative trial entitlements: carry the event cap, so the seed is replaced.
		srv := newLicenseServer(t, map[string]interface{}{"daily_event_limit": 500, "project_limit": 3})
		licClient := licensesvc.NewClient(licensesvc.Config{Host: srv.URL, RetryCount: 0})

		orgRepo := mocks.NewMockOrganisationRepository(ctrl)
		// Guard is skipped when the incoming payload carries the cap, so no fresh
		// re-read happens; the write proceeds directly.
		orgRepo.EXPECT().
			UpdateOrganisationLicenseData(gomock.Any(), orgID, gomock.Any()).
			Return(nil)

		RefreshLicenseDataForOrg(context.Background(), datastore.Organisation{UID: orgID, LicenseData: provisional}, "", true, newDeps(orgRepo), licClient)
	})

	t.Run("replaces_non_provisional_data_when_refresh_lacks_cap", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgID := "org-refresh-paid"
		// No provisional marker (e.g. paid/converted org): a cap-less refresh writes freely.
		srv := newLicenseServer(t, map[string]interface{}{"project_limit": 10})
		licClient := licensesvc.NewClient(licensesvc.Config{Host: srv.URL, RetryCount: 0})

		orgRepo := mocks.NewMockOrganisationRepository(ctrl)
		orgRepo.EXPECT().
			FetchOrganisationByID(gomock.Any(), orgID).
			Return(&datastore.Organisation{UID: orgID, LicenseData: ""}, nil)
		orgRepo.EXPECT().
			UpdateOrganisationLicenseData(gomock.Any(), orgID, gomock.Any()).
			Return(nil)

		RefreshLicenseDataForOrg(context.Background(), datastore.Organisation{UID: orgID, LicenseData: ""}, "", true, newDeps(orgRepo), licClient)
	})
}

// TestClearOrgLicenseData exercises the centralized clear guard: license_data may move
// empty -> provisional -> authoritative, never provisional -> empty. Empty writes must
// preserve a payload carrying the explicit provisional marker, while genuinely
// unlicensed orgs (empty, or unmarked stale data — even with the same entitlement keys
// as the trial seed) keep the fail-closed clear behavior.
func TestClearOrgLicenseData(t *testing.T) {
	encrypt := func(t *testing.T, orgID string, payload *license.LicenseDataPayload) string {
		t.Helper()
		enc, err := license.EncryptLicenseData(orgID, payload)
		require.NoError(t, err)
		return enc
	}

	t.Run("preserves_marked_provisional_seed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgID := "org-guard-provisional"
		provisional := encrypt(t, orgID, &license.LicenseDataPayload{
			Provisional: true,
			Entitlements: map[string]interface{}{
				"daily_event_limit": 100, "org_limit": 1, "user_limit": 1, "project_limit": 1,
			},
		})

		orgRepo := mocks.NewMockOrganisationRepository(ctrl)
		orgRepo.EXPECT().
			FetchOrganisationByID(gomock.Any(), orgID).
			Return(&datastore.Organisation{UID: orgID, LicenseData: provisional}, nil)
		// No UpdateOrganisationLicenseData expectation: any write fails the test.

		err := ClearOrgLicenseData(context.Background(), RefreshLicenseDataDeps{
			OrgRepo: orgRepo,
			Logger:  log.New("convoy", log.LevelInfo),
		}, orgID)
		require.NoError(t, err)
	})

	t.Run("clears_stale_authoritative_data", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgID := "org-guard-stale"
		// Unmarked payload with the same keys as the trial seed: only the marker,
		// not entitlement-key heuristics, protects data from the fail-closed clear.
		stale := encrypt(t, orgID, &license.LicenseDataPayload{Entitlements: map[string]interface{}{
			"daily_event_limit": 100, "org_limit": 1, "user_limit": 1, "project_limit": 1,
		}})

		orgRepo := mocks.NewMockOrganisationRepository(ctrl)
		orgRepo.EXPECT().
			FetchOrganisationByID(gomock.Any(), orgID).
			Return(&datastore.Organisation{UID: orgID, LicenseData: stale}, nil)
		orgRepo.EXPECT().
			UpdateOrganisationLicenseData(gomock.Any(), orgID, "").
			Return(nil)

		err := ClearOrgLicenseData(context.Background(), RefreshLicenseDataDeps{
			OrgRepo: orgRepo,
			Logger:  log.New("convoy", log.LevelInfo),
		}, orgID)
		require.NoError(t, err)
	})

	t.Run("noop_when_already_empty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgID := "org-guard-empty"
		orgRepo := mocks.NewMockOrganisationRepository(ctrl)
		orgRepo.EXPECT().
			FetchOrganisationByID(gomock.Any(), orgID).
			Return(&datastore.Organisation{UID: orgID, LicenseData: ""}, nil)

		err := ClearOrgLicenseData(context.Background(), RefreshLicenseDataDeps{
			OrgRepo: orgRepo,
			Logger:  log.New("convoy", log.LevelInfo),
		}, orgID)
		require.NoError(t, err)
	})

	t.Run("propagates_fetch_error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		orgID := "org-guard-fetch-err"
		orgRepo := mocks.NewMockOrganisationRepository(ctrl)
		orgRepo.EXPECT().
			FetchOrganisationByID(gomock.Any(), orgID).
			Return(nil, errors.New("db unavailable"))

		err := ClearOrgLicenseData(context.Background(), RefreshLicenseDataDeps{
			OrgRepo: orgRepo,
			Logger:  log.New("convoy", log.LevelInfo),
		}, orgID)
		require.Error(t, err)
	})
}

func TestCheckOrganisationProjectLimit_NoKey_ReturnsFalseNil(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Cloud org billing configured, mock returns no org license key → resolveKey returns "" → (false, nil).
	mockBilling := &billing.MockBillingClient{}
	// GetOrganisationLicenseKey left empty so Data.Key is ""

	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	// LoadProjects must not be called when key is ""
	// (no EXPECT so any call would fail the test)

	org := &datastore.Organisation{UID: "org-1", LicenseData: ""}
	deps := OrgProjectLimitDeps{
		BillingClient: mockBilling,
		ProjectRepo:   mockProjectRepo,
		Cfg: config.Configuration{
			Billing:    config.BillingConfiguration{APIKey: "test-key"},
			LicenseKey: "",
		},
		Logger: log.New("convoy", log.LevelInfo),
	}

	allowed, err := CheckOrganisationProjectLimit(ctx, org, deps)
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestCheckOrganisationProjectLimit_ProvisionalOrgLicenseData(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-provisional-project-cap"
	enc, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{
		Key: "lk",
		Entitlements: map[string]interface{}{
			"project_limit": int64(1),
		},
		Provisional: true,
	})
	require.NoError(t, err)

	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	mockProjectRepo.EXPECT().
		LoadProjects(ctx, &datastore.ProjectFilter{OrgID: orgID}).
		Return([]*datastore.Project{{UID: "proj-1"}}, nil)

	org := &datastore.Organisation{UID: orgID, LicenseData: enc}
	deps := OrgProjectLimitDeps{
		ProjectRepo: mockProjectRepo,
		Logger:      log.New("convoy", log.LevelInfo),
	}

	allowed, err := CheckOrganisationProjectLimit(ctx, org, deps)
	require.NoError(t, err)
	require.False(t, allowed)
}
