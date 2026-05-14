package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/billing"
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

	client := &billing.MockBillingClient{}
	h := &BillingHandler{
		Handler: &Handler{
			A: &types.APIOptions{
				Cfg:    config.Configuration{LicenseKey: "lk_test"},
				Logger: log.New("convoy", log.LevelError),
				Billing: &selfHostedForwardingStrategy{
					billingStrategySpy: &billingStrategySpy{},
					client:             client,
				},
			},
		},
		BillingClient: client,
	}

	req := httptest.NewRequest(http.MethodPost, "/ui/self-hosted-billing/verify-email", strings.NewReader(`{"code":" ABC123 "}`))
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
						Cfg:    config.Configuration{LicenseKey: "lk_test"},
						Logger: log.New("convoy", log.LevelError),
						Billing: &selfHostedForwardingStrategy{
							billingStrategySpy: &billingStrategySpy{},
							client:             client,
						},
					},
				},
				BillingClient: client,
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
