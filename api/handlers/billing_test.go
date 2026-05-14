package handlers

import (
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
			SelfHosted bool `json:"self_hosted"`
			License    struct {
				Configured bool `json:"configured"`
			} `json:"license"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Status)
	require.True(t, resp.Data.SelfHosted)
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
