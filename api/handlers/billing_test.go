package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	licensesvc "github.com/frain-dev/convoy/internal/pkg/license/service"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/services"
)

type noCallBillingClient struct {
	*billing.MockBillingClient
}

func (noCallBillingClient) GetOrganisation(context.Context, string) (*billing.Response[billing.BillingOrganisation], error) {
	panic("billing client must not be called when billing is disabled")
}

func (noCallBillingClient) CreateOrganisation(context.Context, billing.BillingOrganisation) (*billing.Response[billing.BillingOrganisation], error) {
	panic("billing client must not be called when billing is disabled")
}

func TestGetInternalOrganisationID_BillingUnavailable_DoesNotCallBilling(t *testing.T) {
	base := &billing.MockBillingClient{}
	client := noCallBillingClient{MockBillingClient: base}

	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:    config.Configuration{},
				Logger: log.New("convoy", log.LevelInfo),
			},
		},
		BillingClient: nil,
	}

	req := httptest.NewRequest(http.MethodGet, "/ui/billing/organisations/org-123/internal_id", nil)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("orgID", "org-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, chiCtx))

	w := httptest.NewRecorder()

	h.GetInternalOrganisationID(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Contains(t, body["message"], "cloud org billing is not configured")
	_ = client
}

func provisionalTrialSeed(t *testing.T, orgID string) string {
	t.Helper()
	enc, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{
		Provisional:  true,
		Entitlements: provisionalTrialEntitlements(),
	})
	require.NoError(t, err)
	return enc
}

func TestProvisionTrialCap_ReplacesWithFullMarkedTrialSet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-trial-provision"
	stale, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{
		Key:          "stale-key",
		Entitlements: map[string]interface{}{"user_limit": 25, "enterprise_sso": true},
	})
	require.NoError(t, err)

	orgRepo, currentData, writes := statefulTrialOrgRepo(ctrl, orgID, stale)
	h := &BillingHandler{
		Handler: &Handler{A: &types.APIOptions{
			Cfg:     config.Configuration{Billing: config.BillingConfiguration{APIKey: "cloud-key"}},
			Logger:  log.New("convoy", log.LevelInfo),
			OrgRepo: orgRepo,
		}},
	}

	h.provisionTrialCap(context.Background(), orgID)

	require.Len(t, writes(), 1, "the seed is a single synchronous write")
	payload, err := license.DecryptLicenseData(orgID, currentData())
	require.NoError(t, err)
	require.True(t, payload.Provisional, "seed must carry the provisional marker")
	require.Equal(t, map[string]interface{}{
		"daily_event_limit": float64(provisionalTrialDailyEventLimit),
		"org_limit":         float64(provisionalTrialOrgLimit),
		"user_limit":        float64(provisionalTrialUserLimit),
		"project_limit":     float64(provisionalTrialProjectLimit),
	}, payload.Entitlements, "seed must be exactly the trial set: stale entitlements replaced, all four caps present")
	require.Empty(t, payload.Key, "stale license key must not survive into the trial seed")
}

func TestProvisionalSeed_EnforcesOrgAndUserLimitsImmediately(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-trial-immediate-caps"
	orgRepo, currentData, _ := statefulTrialOrgRepo(ctrl, orgID, "")
	h := &BillingHandler{
		Handler: &Handler{A: &types.APIOptions{
			Cfg:     config.Configuration{Billing: config.BillingConfiguration{APIKey: "cloud-key"}},
			Logger:  log.New("convoy", log.LevelInfo),
			OrgRepo: orgRepo,
		}},
	}
	h.provisionTrialCap(context.Background(), orgID)
	seeded := currentData()
	require.NotEmpty(t, seeded)

	memberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)

	memberRepo.EXPECT().
		LoadUserOrganisationsPaged(gomock.Any(), "user-1", gomock.Any()).
		Return([]datastore.Organisation{{UID: orgID, LicenseData: seeded}}, datastore.PaginationData{}, nil)
	memberRepo.EXPECT().
		CountUserOrganisations(gomock.Any(), "user-1", "").
		Return(int64(1), nil)
	allowed, err := services.CheckUserOrgCreationAllowed(context.Background(), &datastore.User{UID: "user-1"},
		services.UserOrgLimitDeps{OrgMemberRepo: memberRepo, Logger: h.A.Logger})
	require.NoError(t, err)
	require.False(t, allowed, "org_limit 1 must gate org creation immediately after the seed")

	memberRepo.EXPECT().
		CountOrganisationMembers(gomock.Any(), orgID).
		Return(int64(1), nil)
	allowed, err = services.CheckOrganisationUserLimit(context.Background(),
		&datastore.Organisation{UID: orgID, LicenseData: seeded}, false,
		services.OrgUserLimitDeps{OrgMemberRepo: memberRepo, Logger: h.A.Logger})
	require.NoError(t, err)
	require.False(t, allowed, "user_limit 1 must gate member additions immediately after the seed")
}

func TestHasAuthoritativeEntitlements(t *testing.T) {
	orgID := "org-authoritative-check"

	encrypt := func(payload *license.LicenseDataPayload) string {
		enc, err := license.EncryptLicenseData(orgID, payload)
		require.NoError(t, err)
		return enc
	}

	tests := []struct {
		name        string
		licenseData string
		want        bool
	}{
		{
			name:        "empty license_data",
			licenseData: "",
			want:        false,
		},
		{
			name:        "marked provisional seed with full trial set",
			licenseData: provisionalTrialSeed(t, orgID),
			want:        false,
		},
		{
			name: "unmarked authoritative trial entitlements",
			licenseData: encrypt(&license.LicenseDataPayload{Key: "trial-key", Entitlements: map[string]interface{}{
				"daily_event_limit": 100, "org_limit": 1, "user_limit": 1, "project_limit": 1,
			}}),
			want: true,
		},
		{
			name:        "unmarked payload without entitlements",
			licenseData: encrypt(&license.LicenseDataPayload{Key: "k"}),
			want:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, license.HasAuthoritativeEntitlements(orgID, tt.licenseData))
		})
	}
}

func statefulTrialOrgRepo(ctrl *gomock.Controller, orgID, initial string) (*mocks.MockOrganisationRepository, func() string, func() []string) {
	data := initial
	var writes []string
	repo := mocks.NewMockOrganisationRepository(ctrl)
	repo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		DoAndReturn(func(context.Context, string) (*datastore.Organisation, error) {
			return &datastore.Organisation{UID: orgID, LicenseData: data}, nil
		}).AnyTimes()
	repo.EXPECT().
		UpdateOrganisationLicenseData(gomock.Any(), orgID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, d string) error {
			data = d
			writes = append(writes, d)
			return nil
		}).AnyTimes()
	return repo, func() string { return data }, func() []string { return writes }
}

func trialReconcileHandler(orgRepo datastore.OrganisationRepository, bc billing.Client) (*BillingHandler, services.RefreshLicenseDataDeps) {
	cfg := config.Configuration{Billing: config.BillingConfiguration{APIKey: "cloud-key"}}
	logger := log.New("convoy", log.LevelInfo)
	h := &BillingHandler{
		Handler: &Handler{A: &types.APIOptions{
			Cfg:           cfg,
			Logger:        logger,
			OrgRepo:       orgRepo,
			BillingClient: bc,
		}},
		BillingClient: bc,
	}
	deps := services.RefreshLicenseDataDeps{
		OrgRepo:       orgRepo,
		BillingClient: bc,
		Logger:        logger,
		Cfg:           cfg,
	}
	return h, deps
}

const (
	fullTrialEntitlementsJSON = `[
		{"key": "daily_event_limit", "value": 100},
		{"key": "org_limit", "value": 1},
		{"key": "user_limit", "value": 1},
		{"key": "project_limit", "value": 1}
	]`
	capLessEntitlementsJSON = `[
		{"key": "org_limit", "value": 1},
		{"key": "user_limit", "value": 1},
		{"key": "project_limit", "value": 1}
	]`
)

func switchableTrialLicenseServer(t *testing.T, initialEntitlements string) (*httptest.Server, func(string)) {
	t.Helper()
	var mu sync.Mutex
	entitlements := initialEntitlements
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		e := entitlements
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{
			"status": true,
			"message": "ok",
			"data": {
				"valid": true,
				"status": "active",
				"entitlements": ` + e + `
			}
		}`))
		require.NoError(t, err)
	}))
	t.Cleanup(srv.Close)
	return srv, func(e string) {
		mu.Lock()
		entitlements = e
		mu.Unlock()
	}
}

func trialLicenseServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv, _ := switchableTrialLicenseServer(t, fullTrialEntitlementsJSON)
	return srv
}

func TestReconcileTrialCapOnce_NoWritesWhileMarkedProvisional(t *testing.T) {
	cases := []struct {
		name string
		bc   func() billing.Client
	}{
		{
			name: "billing returns no license key",
			bc:   func() billing.Client { return &billing.MockBillingClient{} },
		},
		{
			name: "billing lookup fails",
			bc: func() billing.Client {
				return &failingGetOrganisationLicenseClient{MockBillingClient: &billing.MockBillingClient{}}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orgID := "org-trial-no-writes"
			seed := provisionalTrialSeed(t, orgID)
			orgRepo, currentData, writes := statefulTrialOrgRepo(ctrl, orgID, seed)
			h, deps := trialReconcileHandler(orgRepo, tc.bc())
			licClient := licensesvc.NewClient(licensesvc.Config{Host: "http://127.0.0.1:0", Logger: h.A.Logger})

			done := h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient)

			require.False(t, done, "poll must continue until authoritative entitlements land")
			require.Empty(t, writes(), "a refresh that yields nothing must perform zero writes")
			require.Equal(t, seed, currentData(), "the marked provisional set must be untouched")
		})
	}
}

func TestReconcileTrialCapOnce_EmptyLicenseDataSeedsCapWithSingleWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-trial-seed-single-write"
	orgRepo, currentData, writes := statefulTrialOrgRepo(ctrl, orgID, "")
	bc := &billing.MockBillingClient{}
	h, deps := trialReconcileHandler(orgRepo, bc)
	licClient := licensesvc.NewClient(licensesvc.Config{Host: "http://127.0.0.1:0", Logger: h.A.Logger})

	done := h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient)

	require.False(t, done)
	require.Len(t, writes(), 1, "exactly one write: the provisional seed")
	require.NotEmpty(t, writes()[0], "the single write must carry the provisional payload, never empty")
	require.True(t, license.IsProvisional(orgID, currentData()), "recovered seed must carry the marker")
	require.Equal(t, int64(provisionalTrialDailyEventLimit), license.DailyEventLimit(orgID, currentData()))
	for _, key := range []string{"org_limit", "user_limit", "project_limit"} {
		limit, applies := license.OrgEntitlementCap(orgID, currentData(), key)
		require.True(t, applies, "recovered seed must carry %s", key)
		require.Equal(t, int64(1), limit)
	}
}

func TestReconcileTrialCapOnce_ProvisionalDoesNotStopPolling_AuthoritativeDoes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-trial-authoritative"
	seed := provisionalTrialSeed(t, orgID)

	orgRepo, currentData, writes := statefulTrialOrgRepo(ctrl, orgID, seed)
	bc := &billing.MockBillingClient{}
	h, deps := trialReconcileHandler(orgRepo, bc)
	licSrv := trialLicenseServer(t)
	licClient := licensesvc.NewClient(licensesvc.Config{Host: licSrv.URL, Logger: h.A.Logger})

	done := h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient)
	require.False(t, done, "the marked provisional seed must not stop polling")
	require.Equal(t, seed, currentData())

	bc.GetOrganisationLicenseKey = "trial-license-key"
	done = h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient)
	require.True(t, done, "poll must stop once authoritative entitlements land")
	require.False(t, license.IsProvisional(orgID, currentData()), "authoritative write must drop the marker")
	require.True(t, license.HasAuthoritativeEntitlements(orgID, currentData()))
	require.Equal(t, int64(100), license.DailyEventLimit(orgID, currentData()))
	for _, w := range writes() {
		require.NotEmpty(t, w, "no write across both phases may carry empty license_data")
	}
}

func TestReconcileTrialCapOnce_ExhaustionLeavesProvisionalCap(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-trial-exhaust"
	seed := provisionalTrialSeed(t, orgID)

	orgRepo, currentData, writes := statefulTrialOrgRepo(ctrl, orgID, seed)
	bc := &billing.MockBillingClient{}
	h, deps := trialReconcileHandler(orgRepo, bc)
	licClient := licensesvc.NewClient(licensesvc.Config{Host: "http://127.0.0.1:0", Logger: h.A.Logger})

	for attempt := 0; attempt < trialCapActivateAttempts; attempt++ {
		require.False(t, h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient))
	}
	require.Empty(t, writes(), "no attempt may write license_data while the marked seed is intact")
	require.Equal(t, seed, currentData(), "exhausted polling budget must leave the marked provisional set in place")
	require.True(t, license.IsProvisional(orgID, currentData()))
	require.Equal(t, int64(provisionalTrialDailyEventLimit), license.DailyEventLimit(orgID, currentData()))
}

func TestReconcileTrialCapOnce_RejectsCaplessAuthoritativePayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-trial-capless-reject"
	seed := provisionalTrialSeed(t, orgID)

	orgRepo, currentData, writes := statefulTrialOrgRepo(ctrl, orgID, seed)
	bc := &billing.MockBillingClient{GetOrganisationLicenseKey: "trial-license-key"}
	h, deps := trialReconcileHandler(orgRepo, bc)
	licSrv, setEntitlements := switchableTrialLicenseServer(t, capLessEntitlementsJSON)
	licClient := licensesvc.NewClient(licensesvc.Config{Host: licSrv.URL, Logger: h.A.Logger})

	done := h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient)
	require.False(t, done, "an authoritative payload without daily_event_limit must not stop the poll")
	require.True(t, license.IsProvisional(orgID, currentData()), "rejecting cycle must re-seed the marked provisional set")
	require.Equal(t, int64(provisionalTrialDailyEventLimit), license.DailyEventLimit(orgID, currentData()),
		"org must stay event-capped after rejecting the fail-open payload")

	setEntitlements(fullTrialEntitlementsJSON)
	done = h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient)
	require.True(t, done, "the complete trial entitlements must end the poll")
	require.False(t, license.IsProvisional(orgID, currentData()))
	require.True(t, license.HasAuthoritativeEntitlements(orgID, currentData()))
	require.Equal(t, int64(100), license.DailyEventLimit(orgID, currentData()))
	for _, w := range writes() {
		require.NotEmpty(t, w, "no cycle may write empty license_data")
	}
}

func TestReconcileTrialCapOnce_ExhaustionAfterCaplessPayloadsKeepsCapped(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-trial-capless-exhaust"
	seed := provisionalTrialSeed(t, orgID)

	orgRepo, currentData, writes := statefulTrialOrgRepo(ctrl, orgID, seed)
	bc := &billing.MockBillingClient{GetOrganisationLicenseKey: "trial-license-key"}
	h, deps := trialReconcileHandler(orgRepo, bc)
	licSrv, _ := switchableTrialLicenseServer(t, capLessEntitlementsJSON)
	licClient := licensesvc.NewClient(licensesvc.Config{Host: licSrv.URL, Logger: h.A.Logger})

	for attempt := 0; attempt < trialCapActivateAttempts; attempt++ {
		require.False(t, h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient),
			"a cap-less payload must never end the poll")
	}
	require.True(t, license.IsProvisional(orgID, currentData()), "exhaustion must leave the marked provisional set")
	require.Equal(t, int64(provisionalTrialDailyEventLimit), license.DailyEventLimit(orgID, currentData()),
		"org must remain event-capped after exhaustion on fail-open payloads")
	for _, w := range writes() {
		require.NotEmpty(t, w, "no attempt may write empty license_data")
	}
}

func TestReconcileTrialCapOnce_PaidAuthoritativeWithoutDailyEventLimitStopsPolling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-paid-no-event-cap"
	paidSeed, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{
		Key:          "paid-key",
		Entitlements: map[string]interface{}{"user_limit": float64(50), "org_limit": float64(5)},
	})
	require.NoError(t, err)

	orgRepo, currentData, writes := statefulTrialOrgRepo(ctrl, orgID, paidSeed)
	bc := &billing.MockBillingClient{GetOrganisationLicenseKey: "paid-license-key"}
	h, deps := trialReconcileHandler(orgRepo, bc)
	licSrv, _ := switchableTrialLicenseServer(t, capLessEntitlementsJSON)
	licClient := licensesvc.NewClient(licensesvc.Config{Host: licSrv.URL, Logger: h.A.Logger})

	done := h.reconcileTrialCapOnce(context.Background(), orgID, true, deps, licClient)

	require.True(t, done, "paid authoritative payloads without daily_event_limit must stop polling")
	require.False(t, license.IsProvisional(orgID, currentData()), "paid payload must not be replaced with a provisional trial seed")
	for _, w := range writes() {
		payload, err := license.DecryptLicenseData(orgID, w)
		require.NoError(t, err)
		require.False(t, payload.Provisional, "poll must not re-seed provisional trial caps over paid entitlements")
	}
}

func TestIsBillingOrgNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "Organisation not found",
			err:  errors.New("Organisation not found"),
			want: true,
		},
		{
			name: "organisation not found lowercase",
			err:  errors.New("organisation not found"),
			want: true,
		},
		{
			name: "billing service error message",
			err:  errors.New("billing service error: Organisation not found"),
			want: true,
		},
		{
			name: "other error",
			err:  errors.New("failed to fetch organisation data"),
			want: false,
		},
		{
			name: "not found without organisation",
			err:  errors.New("resource not found"),
			want: false,
		},
		{
			name: "organisation without not found",
			err:  errors.New("organisation validation failed"),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBillingOrgNotFound(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}
