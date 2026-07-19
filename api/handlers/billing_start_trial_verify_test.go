package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/stretchr/testify/require"
)

func TestCloudTrialEmailVerified(t *testing.T) {
	cloudCfg := config.Configuration{
		Billing: config.BillingConfiguration{APIKey: "cloud-key", URL: "http://billing.test"},
	}
	ossCfg := config.Configuration{}

	cases := []struct {
		name   string
		cfg    config.Configuration
		user   *datastore.User
		wantOK bool
	}{
		{
			name:   "oss always ok",
			cfg:    ossCfg,
			user:   &datastore.User{UID: "u1", EmailVerified: false},
			wantOK: true,
		},
		{
			name:   "cloud unverified rejected",
			cfg:    cloudCfg,
			user:   &datastore.User{UID: "u1", EmailVerified: false},
			wantOK: false,
		},
		{
			name:   "cloud verified ok",
			cfg:    cloudCfg,
			user:   &datastore.User{UID: "u1", EmailVerified: true},
			wantOK: true,
		},
		{
			name:   "cloud missing user rejected",
			cfg:    cloudCfg,
			user:   nil,
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &BillingHandler{
				Handler: &Handler{
					A: &types.APIOptions{
						Cfg:    tc.cfg,
						Logger: log.New("convoy", log.LevelInfo),
					},
				},
			}
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			if tc.user != nil {
				req = req.WithContext(context.WithValue(req.Context(), convoy.AuthUserCtx, &auth.AuthenticatedUser{
					User: tc.user,
				}))
			}
			require.Equal(t, tc.wantOK, h.cloudTrialEmailVerified(req))
		})
	}
}
