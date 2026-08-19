package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	noopLicenser "github.com/frain-dev/convoy/internal/pkg/license/noop"
	"github.com/frain-dev/convoy/internal/pkg/limiter"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// The middleware turns a per-second knob into a per-minute window, so a knob of
// 1 yields 60 tokens per bucket.
const (
	testRateKnob    = 1
	testWindowGrant = testRateKnob * 60
)

// TestMain enables metrics before any test serves a request. The metrics
// instance resolves once per process, so a handler that ran first with metrics
// off would leave the counters permanently disabled for the whole binary.
func TestMain(m *testing.M) {
	// The JWT realm is enabled by default and config validation reads its
	// secrets straight from the environment.
	env := map[string]string{
		"CONVOY_JWT_SECRET":         "middleware-test-secret",
		"CONVOY_JWT_REFRESH_SECRET": "middleware-test-refresh-secret",
	}

	for key, value := range env {
		if err := os.Setenv(key, value); err != nil {
			panic(err)
		}
	}

	if err := config.LoadConfig(""); err != nil {
		panic(err)
	}

	if err := config.Override(&config.Configuration{
		Metrics: config.MetricsConfiguration{IsEnabled: true, Backend: config.PrometheusMetricsProvider},
	}); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

// countingLimiter is a fixed window bucket per key. It records how many times
// each bucket was charged, which is what proves a request spends one token per
// bucket rather than two from a bucket sized for one limit.
type countingLimiter struct {
	mu    sync.Mutex
	calls map[string]int
}

func newCountingLimiter() *countingLimiter {
	return &countingLimiter{calls: map[string]int{}}
}

func (c *countingLimiter) Allow(ctx context.Context, key string, rate int) error {
	return c.AllowWithDuration(ctx, key, rate, 1)
}

func (c *countingLimiter) AllowWithDuration(_ context.Context, key string, rate, _ int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls[key]++
	if c.calls[key] > rate {
		return limiter.NewRateLimitExceeded(time.Second)
	}

	return nil
}

func (c *countingLimiter) charged(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.calls[key]
}

// failingLimiter reports a backend transport failure, which is not a limit hit.
type failingLimiter struct{}

func (failingLimiter) Allow(_ context.Context, _ string, _ int) error {
	return errors.New("dial tcp 127.0.0.1:6379: connect: connection refused")
}

func (l failingLimiter) AllowWithDuration(ctx context.Context, key string, rate, _ int) error {
	return l.Allow(ctx, key, rate)
}

// rejectingLimiter always reports over limit with a fixed delay.
type rejectingLimiter struct {
	delay time.Duration
}

func (l rejectingLimiter) Allow(ctx context.Context, key string, rate int) error {
	return l.AllowWithDuration(ctx, key, rate, 1)
}

func (l rejectingLimiter) AllowWithDuration(_ context.Context, _ string, _, _ int) error {
	return limiter.NewRateLimitExceeded(l.delay)
}

// recordingLogger captures the levels and messages the handler emits. Only the
// two methods the handler uses are implemented; anything else panics on the nil
// embedded interface, which is the intent.
type recordingLogger struct {
	log.Logger

	mu     sync.Mutex
	errors []string
	debugs []string
	fields []any
}

func (l *recordingLogger) ErrorContext(_ context.Context, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.errors = append(l.errors, msg)
	l.fields = append(l.fields, args...)
}

func (l *recordingLogger) DebugContext(_ context.Context, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.debugs = append(l.debugs, msg)
	l.fields = append(l.fields, args...)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func rateLimitOpts(rate limiter.RateLimiter, logger log.Logger) *types.APIOptions {
	return &types.APIOptions{
		Rate:     rate,
		Logger:   logger,
		Licenser: noopLicenser.NewLicenser(),
	}
}

func serve(handler http.Handler) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/events", nil))

	return w
}

// rateLimitCount reads one bucket and outcome sample from the registry the
// /metrics endpoint serves, so the assertion covers registration as well as the
// increment. Tests compare a delta rather than an absolute value, so they do not
// depend on what other tests in the binary recorded.
func rateLimitCount(t *testing.T, bucket, outcome string) float64 {
	t.Helper()

	m := metrics.GetDPInstance(noopLicenser.NewLicenser())
	require.True(t, m.IsEnabled, "metrics must be enabled to assert rate limiter counters")

	families, err := metrics.Reg().Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != "convoy_rate_limit_total" {
			continue
		}

		for _, sample := range family.GetMetric() {
			labels := map[string]string{}
			for _, label := range sample.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}

			if labels["bucket"] == bucket && labels["outcome"] == outcome {
				return sample.GetCounter().GetValue()
			}
		}
	}

	return 0
}

func TestRateLimiterHandlerChargesItsOwnBucketOnce(t *testing.T) {
	rate := newCountingLimiter()
	handler := RateLimiterHandler(rateLimitOpts(rate, &recordingLogger{}), RateLimitBucketIngest, testRateKnob, FailOpen)(okHandler())

	require.Equal(t, http.StatusOK, serve(handler).Code)

	require.Equal(t, 1, rate.charged(RateLimitBucketIngest))
	require.Equal(t, 0, rate.charged(RateLimitBucketAPI))
}

// Each mount point owns its bucket, so exhausting one must not reject the
// other. A shared key made api_rate_limit and instance_ingest_rate one knob.
func TestRateLimiterHandlerBucketsAreIndependent(t *testing.T) {
	tests := []struct {
		name       string
		exhausted  string
		unaffected string
	}{
		{name: "api exhausted", exhausted: RateLimitBucketAPI, unaffected: RateLimitBucketIngest},
		{name: "ingest exhausted", exhausted: RateLimitBucketIngest, unaffected: RateLimitBucketAPI},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rate := newCountingLimiter()
			opts := rateLimitOpts(rate, &recordingLogger{})
			exhausted := RateLimiterHandler(opts, tc.exhausted, testRateKnob, FailClosed)(okHandler())
			unaffected := RateLimiterHandler(opts, tc.unaffected, testRateKnob, FailClosed)(okHandler())

			for i := 0; i < testWindowGrant; i++ {
				require.Equal(t, http.StatusOK, serve(exhausted).Code)
			}
			require.Equal(t, http.StatusTooManyRequests, serve(exhausted).Code)

			require.Equal(t, http.StatusOK, serve(unaffected).Code)
		})
	}
}

// The configured limit must be the limit the caller gets. When both mounts
// shared one bucket every request spent two tokens, halving it.
func TestRateLimiterHandlerEffectiveLimitMatchesConfiguredRate(t *testing.T) {
	rate := newCountingLimiter()
	handler := RateLimiterHandler(rateLimitOpts(rate, &recordingLogger{}), RateLimitBucketAPI, testRateKnob, FailClosed)(okHandler())

	for i := 0; i < testWindowGrant; i++ {
		require.Equal(t, http.StatusOK, serve(handler).Code, "request %d rejected before the configured limit", i+1)
	}

	require.Equal(t, http.StatusTooManyRequests, serve(handler).Code)
}

// RFC 9110 wants delta-seconds. A Unix timestamp here reads as roughly 56,000
// years to a client.
func TestRateLimiterHandlerRetryAfterIsDeltaSeconds(t *testing.T) {
	tests := []struct {
		name     string
		delay    time.Duration
		expected string
	}{
		{name: "whole seconds", delay: 90 * time.Second, expected: "90"},
		{name: "rounds up so the client never retries early", delay: 1500 * time.Millisecond, expected: "2"},
		{name: "floors at one second when no delay is reported", delay: 0, expected: "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := rateLimitOpts(rejectingLimiter{delay: tc.delay}, &recordingLogger{})
			handler := RateLimiterHandler(opts, RateLimitBucketAPI, testRateKnob, FailClosed)(okHandler())

			w := serve(handler)
			require.Equal(t, http.StatusTooManyRequests, w.Code)
			require.Equal(t, tc.expected, w.Header().Get("Retry-After"))
			require.Equal(t, tc.expected, w.Header().Get("X-RateLimit-Reset"))

			seconds, err := strconv.Atoi(w.Header().Get("Retry-After"))
			require.NoError(t, err)
			require.GreaterOrEqual(t, seconds, 1)
			// An epoch timestamp is ~1.7e9; a delay never plausibly exceeds a day.
			require.LessOrEqual(t, seconds, 86400)
		})
	}
}

// A genuine limit hit is a 429 whatever the mount's policy. Only the backend
// failure branch differs, so a fail open mount must not become unlimited.
func TestRateLimiterHandlerRejectsLimitHitUnderEveryPolicy(t *testing.T) {
	tests := []struct {
		name   string
		policy RateLimiterFailurePolicy
	}{
		{name: "fail closed", policy: FailClosed},
		{name: "fail open", policy: FailOpen},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := &recordingLogger{}
			before := rateLimitCount(t, RateLimitBucketIngest, metrics.RateLimitOutcomeRejected)

			opts := rateLimitOpts(rejectingLimiter{delay: time.Second}, logger)
			handler := RateLimiterHandler(opts, RateLimitBucketIngest, testRateKnob, tc.policy)(okHandler())

			require.Equal(t, http.StatusTooManyRequests, serve(handler).Code)

			require.Empty(t, logger.errors)
			require.Len(t, logger.debugs, 1)
			require.Equal(t, before+1, rateLimitCount(t, RateLimitBucketIngest, metrics.RateLimitOutcomeRejected))
		})
	}
}

// A saturated limiter backend and a genuine limit hit are indistinguishable in
// the response, so the log level, the message and the counter outcome are the
// only things that tell an operator which one happened.
func TestRateLimiterHandlerBackendFailureFollowsMountPolicy(t *testing.T) {
	t.Run("fail open admits the request", func(t *testing.T) {
		logger := &recordingLogger{}
		before := rateLimitCount(t, RateLimitBucketIngest, metrics.RateLimitOutcomeBackendError)

		opts := rateLimitOpts(failingLimiter{}, logger)
		handler := RateLimiterHandler(opts, RateLimitBucketIngest, testRateKnob, FailOpen)(okHandler())

		require.Equal(t, http.StatusOK, serve(handler).Code)

		require.Len(t, logger.errors, 1)
		require.Contains(t, logger.errors[0], "fail open")
		require.Empty(t, logger.debugs)
		require.Contains(t, logger.fields, RateLimitBucketIngest)
		require.Equal(t, before+1, rateLimitCount(t, RateLimitBucketIngest, metrics.RateLimitOutcomeBackendError))
	})

	t.Run("fail closed still rejects the request", func(t *testing.T) {
		logger := &recordingLogger{}
		before := rateLimitCount(t, RateLimitBucketAPI, metrics.RateLimitOutcomeBackendError)

		opts := rateLimitOpts(failingLimiter{}, logger)
		handler := RateLimiterHandler(opts, RateLimitBucketAPI, testRateKnob, FailClosed)(okHandler())

		require.Equal(t, http.StatusTooManyRequests, serve(handler).Code)

		require.Len(t, logger.errors, 1)
		require.Contains(t, logger.errors[0], "fail closed")
		require.Empty(t, logger.debugs)
		require.Contains(t, logger.fields, RateLimitBucketAPI)
		require.Equal(t, before+1, rateLimitCount(t, RateLimitBucketAPI, metrics.RateLimitOutcomeBackendError))
	})
}
