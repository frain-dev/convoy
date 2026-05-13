package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authz "github.com/Subomi/go-authz"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// billingStrategySpy implements billing.Strategy for handler tests. It records
// org IDs passed to read helpers that should be tenancy-checked at the HTTP layer.
type billingStrategySpy struct {
	GetPlansCalls       []string
	GetTaxIDTypesCalls  []string
	LicenseSummaryCalls []string
}

func (s *billingStrategySpy) Mode() config.BillingMode { return config.BillingModeCloud }

func (s *billingStrategySpy) GetUsage(ctx context.Context, orgID string) (*billing.Response[billing.Usage], error) {
	return &billing.Response[billing.Usage]{Status: true, Message: "ok", Data: billing.Usage{OrganisationID: orgID}}, nil
}

func (s *billingStrategySpy) GetInvoices(ctx context.Context, orgID string) (*billing.Response[[]billing.Invoice], error) {
	return &billing.Response[[]billing.Invoice]{Status: true, Data: []billing.Invoice{}}, nil
}

func (s *billingStrategySpy) GetInvoice(ctx context.Context, orgID, invoiceID string) (*billing.Response[billing.Invoice], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) DownloadInvoice(ctx context.Context, orgID, invoiceID string) (*http.Response, string, error) {
	return nil, "", billing.ErrNoLicense
}

func (s *billingStrategySpy) GetSubscription(ctx context.Context, orgID string) (*billing.Response[billing.BillingSubscription], error) {
	return &billing.Response[billing.BillingSubscription]{Status: true, Data: billing.BillingSubscription{}}, nil
}

func (s *billingStrategySpy) GetSubscriptions(ctx context.Context, orgID string) (*billing.Response[[]billing.BillingSubscription], error) {
	return &billing.Response[[]billing.BillingSubscription]{Status: true, Data: nil}, nil
}

func (s *billingStrategySpy) GetPaymentMethods(ctx context.Context, orgID string) (*billing.Response[[]billing.PaymentMethod], error) {
	return &billing.Response[[]billing.PaymentMethod]{Status: true, Data: nil}, nil
}

func (s *billingStrategySpy) GetSetupIntent(ctx context.Context, orgID string) (*billing.Response[billing.SetupIntent], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) GetPlans(ctx context.Context, orgID string) (*billing.Response[[]billing.Plan], error) {
	s.GetPlansCalls = append(s.GetPlansCalls, orgID)
	return &billing.Response[[]billing.Plan]{Status: true, Message: "ok", Data: []billing.Plan{}}, nil
}

func (s *billingStrategySpy) GetTaxIDTypes(ctx context.Context, orgID string) (*billing.Response[[]billing.TaxIDType], error) {
	s.GetTaxIDTypesCalls = append(s.GetTaxIDTypesCalls, orgID)
	return &billing.Response[[]billing.TaxIDType]{Status: true, Message: "ok", Data: []billing.TaxIDType{}}, nil
}

func (s *billingStrategySpy) GetOrganisation(ctx context.Context, orgID string) (*billing.Response[billing.BillingOrganisation], error) {
	return &billing.Response[billing.BillingOrganisation]{Status: true, Data: billing.BillingOrganisation{ExternalID: orgID}}, nil
}

func (s *billingStrategySpy) GetInternalOrganisationID(ctx context.Context, orgID string) (string, error) {
	return "", nil
}

func (s *billingStrategySpy) LicenseSummary(ctx context.Context, orgID string) billing.LicenseSummary {
	s.LicenseSummaryCalls = append(s.LicenseSummaryCalls, orgID)
	return billing.LicenseSummary{Configured: true, MaskedKey: "lk_****_test", HasEntitlements: true}
}

func (s *billingStrategySpy) CreateOrganisation(ctx context.Context, data billing.BillingOrganisation) (*billing.Response[billing.BillingOrganisation], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) OnboardSubscription(ctx context.Context, orgID string, req billing.OnboardSubscriptionRequest) (*billing.Response[billing.Checkout], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) UpgradeSubscription(ctx context.Context, orgID, subscriptionID string, req billing.UpgradeSubscriptionRequest) (*billing.Response[billing.Checkout], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) DeleteSubscription(ctx context.Context, orgID, subscriptionID string) (*billing.Response[interface{}], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) SetDefaultPaymentMethod(ctx context.Context, orgID, pmID string) (*billing.Response[interface{}], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) DeletePaymentMethod(ctx context.Context, orgID, pmID string) (*billing.Response[interface{}], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) UpdateOrganisation(ctx context.Context, orgID string, data billing.BillingOrganisation) (*billing.Response[billing.BillingOrganisation], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) UpdateOrganisationTaxID(ctx context.Context, orgID string, data billing.UpdateOrganisationTaxIDRequest) (*billing.Response[billing.BillingOrganisation], error) {
	return nil, billing.ErrNoLicense
}

func (s *billingStrategySpy) UpdateOrganisationAddress(ctx context.Context, orgID string, data billing.UpdateOrganisationAddressRequest) (*billing.Response[billing.BillingOrganisation], error) {
	return nil, billing.ErrNoLicense
}

func authRequestWithUser(r *http.Request, userUID string) *http.Request {
	au := &auth.AuthenticatedUser{User: &datastore.User{UID: userUID}}
	return r.WithContext(context.WithValue(r.Context(), convoy.AuthUserCtx, au))
}

func TestBillingServiceErrorStatus_mapsBillingUnauthorizedTo422(t *testing.T) {
	t.Parallel()

	err := &billing.ServiceError{StatusCode: http.StatusUnauthorized, Message: "billing said no"}
	require.Equal(t, http.StatusUnprocessableEntity, billingServiceErrorStatus(err))
}

func TestBillingServiceErrorStatus_preservesForbiddenFromBilling(t *testing.T) {
	t.Parallel()

	err := &billing.ServiceError{StatusCode: http.StatusForbidden, Message: "billing said no"}
	require.Equal(t, http.StatusForbidden, billingServiceErrorStatus(err))
}

func TestBillingServiceErrorStatus_mapsNoLicenseTo422(t *testing.T) {
	t.Parallel()

	require.Equal(t, http.StatusUnprocessableEntity, billingServiceErrorStatus(billing.ErrNoLicense))
}

func TestBillingServiceErrorStatus_preservesOtherBillingHTTPStatusCodes(t *testing.T) {
	t.Parallel()

	cases := []int{
		http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
	}

	for _, statusCode := range cases {
		statusCode := statusCode
		t.Run(http.StatusText(statusCode), func(t *testing.T) {
			t.Parallel()

			err := &billing.ServiceError{StatusCode: statusCode, Message: "billing service error"}
			require.Equal(t, statusCode, billingServiceErrorStatus(err))
		})
	}
}

type countingCreateBillingClient struct {
	*billing.MockBillingClient
	createCalls int
}

func (c *countingCreateBillingClient) CreateOrganisation(ctx context.Context, orgData billing.BillingOrganisation) (*billing.Response[billing.BillingOrganisation], error) {
	c.createCalls++
	return c.MockBillingClient.CreateOrganisation(ctx, orgData)
}

func TestBillingCreateOrganisation_requiresActiveWorkspaceBillingAccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)

	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), "org-scope").
		Return(&datastore.Organisation{UID: "org-scope"}, nil)
	mockOrgMemberRepo.EXPECT().
		FetchOrganisationMemberByUserID(gomock.Any(), "user-1", "org-scope").
		Return(&datastore.OrganisationMember{Role: auth.Role{Type: auth.RoleProjectAdmin}}, nil)
	mockOrgMemberRepo.EXPECT().
		FetchInstanceAdminByUserID(gomock.Any(), "user-1").
		Return(nil, sql.ErrNoRows).
		AnyTimes()

	bp := &policies.BillingPolicy{
		BasePolicy:             authz.NewBasePolicy(),
		OrganisationMemberRepo: mockOrgMemberRepo,
	}
	bp.SetRule(string(policies.PermissionManage), authz.RuleFunc(bp.Manage))
	az, err := authz.NewAuthz(&authz.AuthzOpts{})
	require.NoError(t, err)
	require.NoError(t, az.RegisterPolicy(bp))

	mockInner := &billing.MockBillingClient{}
	counting := &countingCreateBillingClient{MockBillingClient: mockInner}

	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "sk_cloud"}},
				Logger:        log.New("convoy", log.LevelError),
				Authz:         az,
				OrgRepo:       mockOrgRepo,
				OrgMemberRepo: mockOrgMemberRepo,
			},
		},
		BillingClient: counting,
	}

	body := `{"name":"Malicious Billing Org","external_id":"ext-mal","billing_email":"a@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/ui/billing/organisations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Organisation-Id", "org-scope")
	req = authRequestWithUser(req, "user-1")

	w := httptest.NewRecorder()
	h.CreateOrganisation(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Zero(t, counting.createCalls)
}

func TestBillingCreateOrganisation_rejectsExternalIDNotMatchingHeader(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)

	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), "org-scope").
		Return(&datastore.Organisation{UID: "org-scope"}, nil)
	mockOrgMemberRepo.EXPECT().
		FetchOrganisationMemberByUserID(gomock.Any(), "user-1", "org-scope").
		Return(&datastore.OrganisationMember{Role: auth.Role{Type: auth.RoleBillingAdmin}}, nil)

	bp := &policies.BillingPolicy{
		BasePolicy:             authz.NewBasePolicy(),
		OrganisationMemberRepo: mockOrgMemberRepo,
	}
	bp.SetRule(string(policies.PermissionManage), authz.RuleFunc(bp.Manage))
	az, err := authz.NewAuthz(&authz.AuthzOpts{})
	require.NoError(t, err)
	require.NoError(t, az.RegisterPolicy(bp))

	mockInner := &billing.MockBillingClient{}
	counting := &countingCreateBillingClient{MockBillingClient: mockInner}

	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "sk_cloud"}},
				Logger:        log.New("convoy", log.LevelError),
				Authz:         az,
				OrgRepo:       mockOrgRepo,
				OrgMemberRepo: mockOrgMemberRepo,
			},
		},
		BillingClient: counting,
	}

	body := `{"name":"Legit Org","external_id":"other-org","billing_email":"a@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/ui/billing/organisations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Organisation-Id", "org-scope")
	req = authRequestWithUser(req, "user-1")

	w := httptest.NewRecorder()
	h.CreateOrganisation(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Zero(t, counting.createCalls)
}

func TestBillingGetPlans_rejectsNonMemberOrgIDQuery(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().
		FetchOrganisationMemberByUserID(gomock.Any(), "user-member-of-other-org-only", "org-victim").
		Return(nil, errors.New("not a member"))

	spy := &billingStrategySpy{}
	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "sk_live_test"}, LicenseKey: ""},
				Logger:        log.New("convoy", log.LevelError),
				Billing:       spy,
				OrgMemberRepo: mockOrgMemberRepo,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/ui/billing/plans?org_id=org-victim", nil)
	req = authRequestWithUser(req, "user-member-of-other-org-only")

	w := httptest.NewRecorder()
	h.GetPlans(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, spy.GetPlansCalls)
}

func TestBillingGetTaxIDTypes_rejectsNonMemberOrgIDQuery(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().
		FetchOrganisationMemberByUserID(gomock.Any(), "user-member-of-other-org-only", "org-victim").
		Return(nil, errors.New("not a member"))

	spy := &billingStrategySpy{}
	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:           config.Configuration{Billing: config.BillingConfiguration{APIKey: "sk_live_test"}, LicenseKey: ""},
				Logger:        log.New("convoy", log.LevelError),
				Billing:       spy,
				OrgMemberRepo: mockOrgMemberRepo,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/ui/billing/tax_id_types?org_id=org-victim", nil)
	req = authRequestWithUser(req, "user-member-of-other-org-only")

	w := httptest.NewRecorder()
	h.GetTaxIDTypes(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Empty(t, spy.GetTaxIDTypesCalls)
}

func TestBillingConfigUsesValidatedOrgIDQueryForLicenseSummary(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	mockOrgMemberRepo.EXPECT().
		FetchOrganisationMemberByUserID(gomock.Any(), "user-1", "org-scope").
		Return(&datastore.OrganisationMember{}, nil)

	spy := &billingStrategySpy{}
	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:           config.Configuration{LicenseKey: "lk-instance"},
				Logger:        log.New("convoy", log.LevelError),
				Billing:       spy,
				OrgMemberRepo: mockOrgMemberRepo,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/ui/billing/config?org_id=org-scope", nil)
	req = authRequestWithUser(req, "user-1")

	w := httptest.NewRecorder()
	h.GetBillingConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"org-scope"}, spy.LicenseSummaryCalls)
}
