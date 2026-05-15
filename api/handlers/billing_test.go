package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authz "github.com/Subomi/go-authz"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/policies"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type selfHostedForwardingStrategy struct {
	*billingStrategySpy
	client billing.Client
}

func (s *selfHostedForwardingStrategy) SelfHostedRegisterEmail(ctx context.Context, req billing.SelfHostedRegisterEmailRequest) (*billing.Response[billing.SelfHostedRegisterEmailData], error) {
	return s.client.SelfHostedRegisterEmail(ctx, req)
}

func (s *selfHostedForwardingStrategy) SelfHostedVerifyEmail(ctx context.Context, code string) (*billing.Response[billing.SelfHostedVerifyEmailData], error) {
	return s.client.SelfHostedVerifyEmail(ctx, code)
}

func TestGetBillingEnabled_ReportsModeAndAlwaysEnabled(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cfg      config.Configuration
		wantMode config.BillingMode
		wantSelf bool
	}{
		{
			name:     "cloud when API key set",
			cfg:      config.Configuration{Billing: config.BillingConfiguration{APIKey: "sk_x"}},
			wantMode: config.BillingModeCloud,
			wantSelf: false,
		},
		{
			name:     "licensed when license key set",
			cfg:      config.Configuration{LicenseKey: "lk_x"},
			wantMode: config.BillingModeLicensed,
			wantSelf: true,
		},
		{
			name:     "unlicensed when neither set",
			cfg:      config.Configuration{},
			wantMode: config.BillingModeUnlicensed,
			wantSelf: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := &BillingHandler{
				Handler: &Handler{
					A: &types.APIOptions{
						Cfg:    tc.cfg,
						Logger: log.New("convoy", log.LevelError),
					},
				},
			}

			req := httptest.NewRequest(http.MethodGet, "/ui/billing/enabled", nil)
			w := httptest.NewRecorder()
			h.GetBillingEnabled(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			var resp struct {
				Status bool `json:"status"`
				Data   struct {
					Enabled    bool               `json:"enabled"`
					Mode       config.BillingMode `json:"mode"`
					SelfHosted bool               `json:"self_hosted"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.True(t, resp.Status)
			require.True(t, resp.Data.Enabled, "billing is always enabled post-refactor")
			require.Equal(t, tc.wantMode, resp.Data.Mode)
			require.Equal(t, tc.wantSelf, resp.Data.SelfHosted)
		})
	}
}

func TestGetBillingConfig_SelfHostedOrglessReturnsConfig(t *testing.T) {
	t.Parallel()

	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:    config.Configuration{LicenseKey: "lk_test"},
				Logger: log.New("convoy", log.LevelError),
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/ui/billing/config", nil)
	w := httptest.NewRecorder()
	h.GetBillingConfig(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Status bool `json:"status"`
		Data   struct {
			SelfHosted      bool           `json:"self_hosted"`
			PaymentProvider map[string]any `json:"payment_provider"`
			License         struct {
				Configured bool `json:"configured"`
			} `json:"license"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Status)
	require.True(t, resp.Data.SelfHosted)
	require.Nil(t, resp.Data.PaymentProvider)
	require.False(t, resp.Data.License.Configured)
}

func TestSelfHostedVerifyEmail_TrimsCodeBeforeBillingCall(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
	mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)

	mockOrgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), "org-scope").
		Return(&datastore.Organisation{UID: "org-scope"}, nil).
		AnyTimes()
	mockOrgMemberRepo.EXPECT().
		FetchOrganisationMemberByUserID(gomock.Any(), "user-1", "org-scope").
		Return(&datastore.OrganisationMember{Role: auth.Role{Type: auth.RoleBillingAdmin}}, nil).
		AnyTimes()
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

	client := &billing.MockBillingClient{}
	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:           config.Configuration{LicenseKey: "lk_test"},
				Logger:        log.New("convoy", log.LevelError),
				Authz:         az,
				OrgRepo:       mockOrgRepo,
				OrgMemberRepo: mockOrgMemberRepo,
				Billing: &selfHostedForwardingStrategy{
					billingStrategySpy: &billingStrategySpy{},
					client:             client,
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/self-hosted-billing/verify-email", strings.NewReader(`{"code":" ABC123 "}`))
	req.Header.Set("X-Organisation-Id", "org-scope")
	req = authRequestWithUser(req, "user-1")
	w := httptest.NewRecorder()
	h.SelfHostedVerifyEmail(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "ABC123", client.LastSelfHostedVerifyEmailCode)
}

func TestSelfHostedBilling_MapsServiceErrorStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		statusCode int
		call       func(*BillingHandler) *httptest.ResponseRecorder
	}{
		{
			name:       "register email",
			statusCode: http.StatusTooManyRequests,
			call: func(h *BillingHandler) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, "/ui/self-hosted-billing/register-email", strings.NewReader(`{"email":"owner@example.com"}`))
				req.Header.Set("X-Organisation-Id", "org-scope")
				req = authRequestWithUser(req, "user-1")
				w := httptest.NewRecorder()
				h.SelfHostedRegisterEmail(w, req)
				return w
			},
		},
		{
			name:       "verify email",
			statusCode: http.StatusUnprocessableEntity,
			call: func(h *BillingHandler) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPost, "/ui/self-hosted-billing/verify-email", strings.NewReader(`{"code":"ABC123"}`))
				req.Header.Set("X-Organisation-Id", "org-scope")
				req = authRequestWithUser(req, "user-1")
				w := httptest.NewRecorder()
				h.SelfHostedVerifyEmail(w, req)
				return w
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockOrgRepo := mocks.NewMockOrganisationRepository(ctrl)
			mockOrgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
			mockOrgRepo.EXPECT().
				FetchOrganisationByID(gomock.Any(), "org-scope").
				Return(&datastore.Organisation{UID: "org-scope"}, nil).
				AnyTimes()
			mockOrgMemberRepo.EXPECT().
				FetchOrganisationMemberByUserID(gomock.Any(), "user-1", "org-scope").
				Return(&datastore.OrganisationMember{Role: auth.Role{Type: auth.RoleBillingAdmin}}, nil).
				AnyTimes()
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

			client := &selfHostedErrorBillingClient{
				MockBillingClient: &billing.MockBillingClient{},
				err: &billing.ServiceError{
					StatusCode: tc.statusCode,
					Message:    "upstream rejected request",
				},
			}
			h := &BillingHandler{
				Handler: &Handler{
					A: &types.APIOptions{
						Cfg:           config.Configuration{LicenseKey: "lk_test"},
						Logger:        log.New("convoy", log.LevelError),
						Authz:         az,
						OrgRepo:       mockOrgRepo,
						OrgMemberRepo: mockOrgMemberRepo,
						Billing: &selfHostedForwardingStrategy{
							billingStrategySpy: &billingStrategySpy{},
							client:             client,
						},
					},
				},
			}

			w := tc.call(h)

			require.Equal(t, tc.statusCode, w.Code)
		})
	}
}

type selfHostedErrorBillingClient struct {
	*billing.MockBillingClient
	err error
}

func (c *selfHostedErrorBillingClient) SelfHostedRegisterEmail(ctx context.Context, req billing.SelfHostedRegisterEmailRequest) (*billing.Response[billing.SelfHostedRegisterEmailData], error) {
	return nil, c.err
}

func (c *selfHostedErrorBillingClient) SelfHostedVerifyEmail(ctx context.Context, code string) (*billing.Response[billing.SelfHostedVerifyEmailData], error) {
	return nil, c.err
}
