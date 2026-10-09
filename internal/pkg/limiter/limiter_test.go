package limiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRateLimitErrorContract(t *testing.T) {
	delay := 3 * time.Second
	err := NewRateLimitExceeded(delay)

	require.Equal(t, ErrRateLimitExceeded, GetRawError(err))
	require.Equal(t, delay, GetRetryAfter(err))
	require.Zero(t, GetRetryAfter(ErrRateLimitExceeded))
	require.Nil(t, GetRawError(ErrRateLimitExceeded))
}
