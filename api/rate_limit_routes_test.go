package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	noopLicenser "github.com/frain-dev/convoy/internal/pkg/license/noop"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/testenv"
)

// Sentinel knob values so the captured rate proves which config field the
// route's limiter was built from. The middleware multiplies by a 60s window.
const (
	probeApiRateLimit       = 7
	probeInstanceIngestRate = 11
)

const (
	probeUsername = "rate-limit-probe"
	probePassword = "rate-limit-probe-password"
)

// newRateLimitProbeHandler builds a handler with a mock rate limiter. The mock
// rejects the request, so route middleware order and knob selection can be
// asserted without a database, auth, or Redis.
func newRateLimitProbeHandler(t *testing.T, rate *mocks.MockRateLimiter) *ApplicationHandler {
	t.Helper()

	err := config.LoadConfig("")
	require.NoError(t, err)

	cfg, err := config.Get()
	require.NoError(t, err)

	cfg.ApiRateLimit = probeApiRateLimit
	cfg.InstanceIngestRate = probeInstanceIngestRate

	return &ApplicationHandler{
		A: &types.APIOptions{
			Logger:   testenv.NewLogger(t),
			Rate:     rate,
			FFlag:    fflag.NewFFlag(nil),
			Licenser: noopLicenser.NewLicenser(),
		},
		cfg: cfg,
	}
}

// Both /ingest surfaces (control plane and data plane/agent) must be limited
// by InstanceIngestRate. The data plane previously used ApiRateLimit, so
// CONVOY_INSTANCE_INGEST_RATE silently did nothing for agent ingest.
func TestIngestRoutesUseInstanceIngestRate(t *testing.T) {
	tests := []struct {
		name   string
		router func(a *ApplicationHandler) http.Handler
	}{
		{
			name:   "control plane",
			router: func(a *ApplicationHandler) http.Handler { return a.BuildControlPlaneRoutes() },
		},
		{
			name:   "data plane",
			router: func(a *ApplicationHandler) http.Handler { return a.BuildDataPlaneRoutes() },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rate := mocks.NewMockRateLimiter(ctrl)
			handler := newRateLimitProbeHandler(t, rate)

			rate.EXPECT().
				AllowWithDuration(gomock.Any(), middleware.RateLimitBucketIngest, probeInstanceIngestRate*60, 60).
				Return(errors.New("rate limit exceeded"))

			req := httptest.NewRequest(http.MethodPost, "/ingest/mask123", nil)
			w := httptest.NewRecorder()
			tc.router(handler).ServeHTTP(w, req)

			require.Equal(t, http.StatusTooManyRequests, w.Code)
		})
	}
}

// The public API surface must charge its own bucket. Sharing one key with the
// ingest mounts made CONVOY_API_RATE_LIMIT and CONVOY_INSTANCE_INGEST_RATE a
// single knob, and charged that knob twice on the events write path.
func TestProjectRoutesUseApiRateLimitBucket(t *testing.T) {
	tests := []struct {
		name   string
		router func(a *ApplicationHandler) http.Handler
	}{
		{
			name:   "control plane",
			router: func(a *ApplicationHandler) http.Handler { return a.BuildControlPlaneRoutes() },
		},
		{
			name:   "data plane",
			router: func(a *ApplicationHandler) http.Handler { return a.BuildDataPlaneRoutes() },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			rate := mocks.NewMockRateLimiter(ctrl)
			handler := newRateLimitProbeHandler(t, rate)
			initProbeFileRealm(t, handler.cfg)

			rate.EXPECT().
				AllowWithDuration(gomock.Any(), middleware.RateLimitBucketAPI, probeApiRateLimit*60, 60).
				Return(errors.New("rate limit exceeded"))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/probe-project/events", nil)
			req.SetBasicAuth(probeUsername, probePassword)
			w := httptest.NewRecorder()
			tc.router(handler).ServeHTTP(w, req)

			require.Equal(t, http.StatusTooManyRequests, w.Code)
		})
	}
}

// initProbeFileRealm authenticates the probe request from static credentials so
// the request reaches the rate limiter without a database or Redis.
func initProbeFileRealm(t *testing.T, cfg config.Configuration) {
	t.Helper()

	authCfg := cfg.Auth
	authCfg.Native.Enabled = false
	authCfg.Jwt.Enabled = false
	authCfg.Portal.Enabled = false
	authCfg.File.Basic = config.BasicAuthConfig{{
		Username: probeUsername,
		Password: probePassword,
		Role:     auth.Role{Type: auth.RoleInstanceAdmin},
	}}

	require.NoError(t, realm_chain.Init(&authCfg, nil, nil, nil, nil, testenv.NewLogger(t)))
}
