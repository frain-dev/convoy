package retrystrategies

import (
	"testing"
	"time"
)

func TestExponentialBackoffRetryStrategy(t *testing.T) {
	tests := []struct {
		name             string
		expectedDuration time.Duration
		attempts         uint64
		jitterFn         JitterFn
		millis           []uint
	}{
		{
			name:             "no-delay-for-initial-attempt",
			expectedDuration: time.Duration(0) * time.Millisecond,
			attempts:         0,
			jitterFn: func(u uint) int {
				return int(u)
			},
			millis: []uint{0, 100, 200, 1000},
		},
		{
			name:             "duration-dependent-on-attempts",
			expectedDuration: time.Duration(100) * time.Millisecond,
			attempts:         1,
			jitterFn: func(u uint) int {
				return int(u)
			},
			millis: []uint{0, 100, 200, 1000},
		},
		{
			name:             "duration-dependent-on-attempts-wraps",
			expectedDuration: time.Duration(5000) * time.Millisecond,
			attempts:         7,
			jitterFn: func(u uint) int {
				return int(u)
			},
			millis: []uint{0, 100, 200, 1000, 2000, 5000},
		},
		{
			name:             "jitterfn-affects-duration",
			expectedDuration: time.Duration(500) * time.Millisecond,
			attempts:         1,
			jitterFn: func(u uint) int {
				return 500
			},
			millis: []uint{0, 100, 200, 1000},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			retry := NewExponentialWithJitter(tc.millis, tc.jitterFn)

			got := retry.NextDuration(tc.attempts)

			if got != tc.expectedDuration {
				t.Errorf("Want duration '%v' for attempts '%d', got '%v'", tc.expectedDuration, tc.attempts, got)
			}
		})
	}
}
