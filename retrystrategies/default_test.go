package retrystrategies

import (
	"testing"
	"time"
)

func TestDefaultRetryStrategy(t *testing.T) {
	tests := []struct {
		name             string
		expectedDuration time.Duration
		attempts         uint64
		interval         uint64
	}{
		{
			name:             "duration-as-seconds",
			expectedDuration: time.Duration(1) * time.Second,
			attempts:         0,
			interval:         1,
		},
		{
			name:             "duration-dependent-on-interval",
			expectedDuration: time.Duration(5) * time.Second,
			attempts:         0,
			interval:         5,
		},
		{
			name:             "duration-not-attempt-dependent",
			expectedDuration: time.Duration(5) * time.Second,
			attempts:         200,
			interval:         5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			retry := NewDefault(tc.interval)

			got := retry.NextDuration(tc.attempts)

			if got != tc.expectedDuration {
				t.Errorf("Want duration '%v' for attempts '%d', got '%v'", tc.expectedDuration, tc.attempts, got)
			}
		})
	}
}
