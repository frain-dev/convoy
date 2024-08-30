package circuit_breaker

import (
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCircuitBreaker_String(t *testing.T) {
	cb := &CircuitBreaker{
		Key:                 "test",
		State:               StateClosed,
		Requests:            100,
		FailureRate:         0.1,
		WillResetAt:         time.Now(),
		TotalFailures:       10,
		TotalSuccesses:      90,
		ConsecutiveFailures: 2,
	}

	t.Run("Success", func(t *testing.T) {
		result, err := cb.String()

		require.NoError(t, err)
		require.NotEmpty(t, result)

		// Decode the result back to a CircuitBreaker
		var decodedCB CircuitBreaker
		err = msgpack.DecodeMsgPack([]byte(result), &decodedCB)
		require.NoError(t, err)

		// Compare the decoded CircuitBreaker with the original
		require.Equal(t, cb.Key, decodedCB.Key)
		require.Equal(t, cb.State, decodedCB.State)
		require.Equal(t, cb.Requests, decodedCB.Requests)
		require.Equal(t, cb.FailureRate, decodedCB.FailureRate)
		require.Equal(t, cb.WillResetAt.Unix(), decodedCB.WillResetAt.Unix())
		require.Equal(t, cb.TotalFailures, decodedCB.TotalFailures)
		require.Equal(t, cb.TotalSuccesses, decodedCB.TotalSuccesses)
		require.Equal(t, cb.ConsecutiveFailures, decodedCB.ConsecutiveFailures)
	})
}

func TestCircuitBreaker_tripCircuitBreaker(t *testing.T) {
	cb := &CircuitBreaker{
		State:               StateClosed,
		ConsecutiveFailures: 0,
	}

	resetTime := time.Now().Add(30 * time.Second)
	cb.tripCircuitBreaker(resetTime)

	require.Equal(t, StateOpen, cb.State)
	require.Equal(t, resetTime, cb.WillResetAt)
	require.Equal(t, uint64(1), cb.ConsecutiveFailures)
}

func TestCircuitBreaker_toHalfOpen(t *testing.T) {
	cb := &CircuitBreaker{
		State: StateOpen,
	}

	cb.toHalfOpen()

	require.Equal(t, StateHalfOpen, cb.State)
}

func TestCircuitBreaker_resetCircuitBreaker(t *testing.T) {
	cb := &CircuitBreaker{
		State:               StateOpen,
		ConsecutiveFailures: 5,
	}

	cb.resetCircuitBreaker()

	require.Equal(t, StateClosed, cb.State)
	require.Equal(t, uint64(0), cb.ConsecutiveFailures)
}
