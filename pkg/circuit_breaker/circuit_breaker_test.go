package circuit_breaker

import (
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
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
		result := cb.String()

		require.NotEmpty(t, result)

		// Decode the result back to a CircuitBreaker
		var decodedCB CircuitBreaker
		err := msgpack.DecodeMsgPack([]byte(result), &decodedCB)
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
	cb.trip(resetTime)

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

	cb.Reset(time.Now())

	require.Equal(t, StateClosed, cb.State)
	require.Equal(t, uint64(0), cb.ConsecutiveFailures)
}

func TestNewCircuitBreakerFromStore(t *testing.T) {
	createValidMsgpack := func() []byte {
		cb := &CircuitBreaker{
			Key:                 "test-key",
			TenantId:            "tenant-1",
			State:               StateClosed,
			Requests:            10,
			FailureRate:         0.2,
			SuccessRate:         0.8,
			WillResetAt:         time.Now().Add(time.Hour),
			TotalFailures:       2,
			TotalSuccesses:      8,
			ConsecutiveFailures: 1,
			NotificationsSent:   1,
		}
		data, err := msgpack.EncodeMsgPack(cb)
		if err != nil {
			t.Fatalf("Failed to create test data: %v", err)
		}
		return data
	}

	logger := log.NewLogger(os.Stdout)

	tests := []struct {
		name        string
		input       []byte
		logger      *log.Logger
		wantErr     bool
		errContains string
		validate    func(*testing.T, *CircuitBreaker)
	}{
		{
			name:        "empty input",
			input:       []byte{},
			logger:      logger,
			wantErr:     true,
			errContains: "EOF",
		},
		{
			name:        "invalid CircuitBreaker",
			input:       []byte{0x1, 0x2, 0x3},
			logger:      logger,
			wantErr:     true,
			errContains: "decoding map length",
		},
		{
			name:    "valid CircuitBreaker with logger",
			input:   createValidMsgpack(),
			logger:  logger,
			wantErr: false,
			validate: func(t *testing.T, cb *CircuitBreaker) {
				assert.Equal(t, "test-key", cb.Key)
				assert.Equal(t, "tenant-1", cb.TenantId)
				assert.Equal(t, StateClosed, cb.State)
				assert.Equal(t, uint64(10), cb.Requests)
				assert.Equal(t, 0.2, cb.FailureRate)
				assert.Equal(t, 0.8, cb.SuccessRate)
				assert.Equal(t, uint64(2), cb.TotalFailures)
				assert.Equal(t, uint64(8), cb.TotalSuccesses)
				assert.Equal(t, uint64(1), cb.ConsecutiveFailures)
				assert.Equal(t, uint64(1), cb.NotificationsSent)
				assert.NotNil(t, cb.logger)
			},
		},
		{
			name:    "valid CircuitBreaker without logger",
			input:   createValidMsgpack(),
			logger:  nil,
			wantErr: false,
			validate: func(t *testing.T, cb *CircuitBreaker) {
				assert.Equal(t, "test-key", cb.Key)
				assert.Nil(t, cb.logger)
			},
		},
		{
			name: "CircuitBreaker with different state",
			input: func() []byte {
				cb := &CircuitBreaker{
					Key:   "test-key",
					State: StateOpen,
				}
				data, _ := msgpack.EncodeMsgPack(cb)
				return data
			}(),
			logger:  logger,
			wantErr: false,
			validate: func(t *testing.T, cb *CircuitBreaker) {
				assert.Equal(t, StateOpen, cb.State)
			},
		},
		{
			name: "large numbers test",
			input: func() []byte {
				cb := &CircuitBreaker{
					Key:                 "test-key",
					Requests:            18446744073709551615, // max uint64
					TotalSuccesses:      18446744073709551615, // max uint64
					ConsecutiveFailures: 18446744073709551615, // max uint64
				}
				data, _ := msgpack.EncodeMsgPack(cb)
				return data
			}(),
			logger:  logger,
			wantErr: false,
			validate: func(t *testing.T, cb *CircuitBreaker) {
				assert.Equal(t, uint64(18446744073709551615), cb.Requests)
				assert.Equal(t, uint64(18446744073709551615), cb.TotalSuccesses)
				assert.Equal(t, uint64(18446744073709551615), cb.ConsecutiveFailures)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCircuitBreakerFromStore(tt.input, tt.logger)

			if tt.wantErr {
				assert.Error(t, err)
				if len(tt.errContains) > 0 {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, got)

			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}
