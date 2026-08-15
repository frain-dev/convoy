package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/pkg/limiter"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// The middleware turns a per-second knob into a per-minute window, so a knob of
// 1 yields 60 tokens per bucket.
const (
	testRateKnob    = 1
	testWindowGrant = testRateKnob * 60
)

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

// eventsStack mirrors the events write path, where the projects router charges
// the API bucket and the write group charges the ingest bucket.
func eventsStack(rate limiter.RateLimiter) http.Handler {
	return RateLimiterHandler(rate, RateLimitBucketAPI, testRateKnob, &recordingLogger{})(
		RateLimiterHandler(rate, RateLimitBucketIngest, testRateKnob, &recordingLogger{})(okHandler()),
	)
}

func serve(handler http.Handler) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/events", nil))

	return w
}

func TestRateLimiterHandlerChargesEachBucketOnce(t *testing.T) {
	rate := newCountingLimiter()

	require.Equal(t, http.StatusOK, serve(eventsStack(rate)).Code)

	require.Equal(t, 1, rate.charged(RateLimitBucketAPI))
	require.Equal(t, 1, rate.charged(RateLimitBucketIngest))
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
			exhausted := RateLimiterHandler(rate, tc.exhausted, testRateKnob, &recordingLogger{})(okHandler())
			unaffected := RateLimiterHandler(rate, tc.unaffected, testRateKnob, &recordingLogger{})(okHandler())

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
	stack := eventsStack(rate)

	for i := 0; i < testWindowGrant; i++ {
		require.Equal(t, http.StatusOK, serve(stack).Code, "request %d rejected before the configured limit", i+1)
	}

	require.Equal(t, http.StatusTooManyRequests, serve(stack).Code)
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
			handler := RateLimiterHandler(rejectingLimiter{delay: tc.delay}, RateLimitBucketAPI, testRateKnob, &recordingLogger{})(okHandler())

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

// A saturated limiter backend and a genuine limit hit both return 429, so the
// log is the only thing that tells an operator which one happened.
func TestRateLimiterHandlerDistinguishesBackendFailureFromLimitHit(t *testing.T) {
	t.Run("backend failure logs an error", func(t *testing.T) {
		logger := &recordingLogger{}
		handler := RateLimiterHandler(failingLimiter{}, RateLimitBucketIngest, testRateKnob, logger)(okHandler())

		// Failure policy stays fail closed: an unreachable limiter still 429s.
		require.Equal(t, http.StatusTooManyRequests, serve(handler).Code)

		require.Len(t, logger.errors, 1)
		require.Empty(t, logger.debugs)
		require.Contains(t, logger.fields, RateLimitBucketIngest)
	})

	t.Run("limit hit does not log an error", func(t *testing.T) {
		logger := &recordingLogger{}
		handler := RateLimiterHandler(rejectingLimiter{delay: time.Second}, RateLimitBucketIngest, testRateKnob, logger)(okHandler())

		require.Equal(t, http.StatusTooManyRequests, serve(handler).Code)

		require.Empty(t, logger.errors)
		require.Len(t, logger.debugs, 1)
	})
}
