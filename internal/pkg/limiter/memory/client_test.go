package mlimiter

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAllow(t *testing.T) {
	rl := NewMemoryRateLimiter()
	key := "test-key"
	rate := 10
	duration := 10

	for i := 0; i < 11; i++ {
		err := rl.Allow(context.Background(), key, rate, duration)
		time.Sleep(time.Second)
		require.NoError(t, err)
	}

	err := rl.Allow(context.Background(), key, rate, duration)
	require.Error(t, err)
}
