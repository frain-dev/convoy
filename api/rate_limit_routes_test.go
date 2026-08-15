package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	noopLicenser "github.com/frain-dev/convoy/internal/pkg/license/noop"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
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
				Return(limiter.NewRateLimitExceeded(time.Second))

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
//
// The event routes here are the neighbours of the intake routes, which are now
// registered a level up so they escape this mount. They prove the neighbours
// still resolve through the projects subtree and still charge the API bucket.
func TestProjectRoutesUseApiRateLimitBucket(t *testing.T) {
	tests := []struct {
		name   string
		paths  []apiProbeRoute
		router func(a *ApplicationHandler) http.Handler
	}{
		{
			name: "control plane",
			paths: []apiProbeRoute{
				{method: http.MethodGet, path: "/api/v1/projects/probe-project/events"},
				{method: http.MethodGet, path: "/api/v1/projects/probe-project/events/countbatchreplayevents"},
				{method: http.MethodPost, path: "/api/v1/projects/probe-project/events/batchreplay"},
				{method: http.MethodGet, path: "/api/v1/projects/probe-project/events/probe-event"},
				{method: http.MethodPut, path: "/api/v1/projects/probe-project/events/probe-event/replay"},
				{method: http.MethodGet, path: "/api/v1/projects/probe-project/endpoints"},
			},
			router: func(a *ApplicationHandler) http.Handler { return a.BuildControlPlaneRoutes() },
		},
		{
			name: "data plane",
			paths: []apiProbeRoute{
				{method: http.MethodGet, path: "/api/v1/projects/probe-project/events"},
				{method: http.MethodPost, path: "/api/v1/projects/probe-project/events/batchreplay"},
				{method: http.MethodGet, path: "/api/v1/projects/probe-project/events/probe-event"},
				{method: http.MethodPut, path: "/api/v1/projects/probe-project/events/probe-event/replay"},
			},
			router: func(a *ApplicationHandler) http.Handler { return a.BuildDataPlaneRoutes() },
		},
	}

	for _, tc := range tests {
		for _, route := range tc.paths {
			t.Run(tc.name+" "+route.method+" "+route.path, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				rate := mocks.NewMockRateLimiter(ctrl)
				handler := newRateLimitProbeHandler(t, rate)
				initProbeFileRealm(t, handler.cfg)

				rate.EXPECT().
					AllowWithDuration(gomock.Any(), middleware.RateLimitBucketAPI, probeApiRateLimit*60, 60).
					Return(limiter.NewRateLimitExceeded(time.Second))

				req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				req.SetBasicAuth(probeUsername, probePassword)
				w := httptest.NewRecorder()
				tc.router(handler).ServeHTTP(w, req)

				require.Equal(t, http.StatusTooManyRequests, w.Code)
			})
		}
	}
}

type apiProbeRoute struct {
	method string
	path   string
}

// A limiter backend failure on the API surface must still reject. Fail open
// belongs to event intake only, so an outage of the limiter must not turn the
// authenticated API into an unmetered one.
func TestProjectRoutesFailClosedOnLimiterBackendFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	rate := mocks.NewMockRateLimiter(ctrl)
	handler := newRateLimitProbeHandler(t, rate)
	initProbeFileRealm(t, handler.cfg)

	rate.EXPECT().
		AllowWithDuration(gomock.Any(), middleware.RateLimitBucketAPI, probeApiRateLimit*60, 60).
		Return(errors.New("dial tcp 127.0.0.1:6379: connect: connection refused"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/probe-project/events", nil)
	req.SetBasicAuth(probeUsername, probePassword)
	w := httptest.NewRecorder()
	handler.BuildControlPlaneRoutes().ServeHTTP(w, req)

	require.Equal(t, http.StatusTooManyRequests, w.Code)
}

// Event intake must be charged to the ingest bucket and to nothing else. A fail
// open policy on the ingest bucket is cosmetic while a sibling limiter on the
// same request path still rejects on a backend failure, so the assertion is the
// exact set of buckets the path charges. The mock is strict, so a charge to the
// API bucket fails the test.
func TestEventIntakeChargesTheIngestBucketOnly(t *testing.T) {
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

	intakePaths := []string{
		"/api/v1/projects/probe-project/events",
		"/api/v1/projects/probe-project/events/fanout",
		"/api/v1/projects/probe-project/events/broadcast",
		"/api/v1/projects/probe-project/events/dynamic",
	}

	for _, tc := range tests {
		for _, path := range intakePaths {
			t.Run(tc.name+" "+path, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				rate := mocks.NewMockRateLimiter(ctrl)
				handler := newRateLimitProbeHandler(t, rate)
				initProbeFileRealm(t, handler.cfg)

				rate.EXPECT().
					AllowWithDuration(gomock.Any(), middleware.RateLimitBucketIngest, probeInstanceIngestRate*60, 60).
					Return(limiter.NewRateLimitExceeded(time.Second))

				req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"event_type":"*","data":{}}`))
				req.Header.Set("Content-Type", "application/json")
				req.SetBasicAuth(probeUsername, probePassword)
				w := httptest.NewRecorder()
				tc.router(handler).ServeHTTP(w, req)

				require.Equal(t, http.StatusTooManyRequests, w.Code)
			})
		}
	}
}

// failingRateLimiter stands in for an unreachable limiter backend, which is a
// different outcome from a genuine over-limit result.
type failingRateLimiter struct{}

func (failingRateLimiter) Allow(_ context.Context, _ string, _ int) error {
	return errors.New("dial tcp 127.0.0.1:6379: connect: connection refused")
}

func (l failingRateLimiter) AllowWithDuration(ctx context.Context, key string, rate, _ int) error {
	return l.Allow(ctx, key, rate)
}

// rejectingRateLimiter reports a genuine over-limit result for every bucket.
type rejectingRateLimiter struct{}

func (rejectingRateLimiter) Allow(_ context.Context, _ string, _ int) error {
	return limiter.NewRateLimitExceeded(time.Second)
}

func (l rejectingRateLimiter) AllowWithDuration(ctx context.Context, key string, rate, _ int) error {
	return l.Allow(ctx, key, rate)
}

// recordingRateLimiter admits every request and records which buckets a request
// was charged to, which is what proves the traversal map of a route.
type recordingRateLimiter struct {
	mu      sync.Mutex
	buckets []string
}

func (r *recordingRateLimiter) Allow(ctx context.Context, key string, rate int) error {
	return r.AllowWithDuration(ctx, key, rate, 1)
}

func (r *recordingRateLimiter) AllowWithDuration(_ context.Context, key string, _, _ int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.buckets = append(r.buckets, key)

	return nil
}

func (r *recordingRateLimiter) charged() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]string(nil), r.buckets...)
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
