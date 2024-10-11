package circuit_breaker

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCircuitBreakerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  CircuitBreakerConfig
		wantErr bool
		err     string
	}{
		{
			name: "Valid Config",
			config: CircuitBreakerConfig{
				SampleRate:                  1,
				BreakerTimeout:              30,
				FailureThreshold:            50,
				SuccessThreshold:            2,
				ObservabilityWindow:         5,
				ConsecutiveFailureThreshold: 3,
				MinimumRequestCount:         10,
			},
			wantErr: false,
		},
		{
			name: "Invalid SampleRate",
			config: CircuitBreakerConfig{
				SampleRate: 0,
			},
			wantErr: true,
			err:     "SampleRate must be greater than 0",
		},
		{
			name: "Invalid ErrorTimeout",
			config: CircuitBreakerConfig{
				SampleRate:     1,
				BreakerTimeout: 0,
			},
			wantErr: true,
			err:     "BreakerTimeout must be greater than 0",
		},
		{
			name: "Invalid FailureThreshold",
			config: CircuitBreakerConfig{
				SampleRate:       1,
				BreakerTimeout:   30,
				FailureThreshold: 150,
			},
			wantErr: true,
			err:     "FailureThreshold must be between 1 and 100",
		},
		{
			name: "Invalid SuccessThreshold",
			config: CircuitBreakerConfig{
				SampleRate:       1,
				BreakerTimeout:   30,
				FailureThreshold: 5,
				SuccessThreshold: 150,
			},
			wantErr: true,
			err:     "SuccessThreshold must be between 1 and 100",
		},
		{
			name: "Invalid ObservabilityWindow",
			config: CircuitBreakerConfig{
				SampleRate:          1,
				BreakerTimeout:      30,
				FailureThreshold:    50,
				SuccessThreshold:    2,
				ObservabilityWindow: 0,
			},
			wantErr: true,
			err:     "ObservabilityWindow must be greater than 0",
		},
		{
			name: "ObservabilityWindow should be greater than sample rate",
			config: CircuitBreakerConfig{
				SampleRate:          200,
				BreakerTimeout:      30,
				FailureThreshold:    50,
				SuccessThreshold:    2,
				ObservabilityWindow: 1,
			},
			wantErr: true,
			err:     "ObservabilityWindow must be greater than the SampleRate",
		},
		{
			name: "Invalid ConsecutiveFailureThreshold",
			config: CircuitBreakerConfig{
				SampleRate:                  1,
				BreakerTimeout:              30,
				FailureThreshold:            50,
				SuccessThreshold:            2,
				ObservabilityWindow:         5,
				ConsecutiveFailureThreshold: 0,
			},
			wantErr: true,
			err:     "ConsecutiveFailureThreshold must be greater than 0",
		},
		{
			name: "Invalid MinimumRequestCount",
			config: CircuitBreakerConfig{
				SampleRate:                  1,
				BreakerTimeout:              30,
				FailureThreshold:            30,
				SuccessThreshold:            2,
				ObservabilityWindow:         5,
				MinimumRequestCount:         5,
				ConsecutiveFailureThreshold: 1,
			},
			wantErr: true,
			err:     "MinimumRequestCount must be greater than 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.NotEmpty(t, tt.err)
				require.Contains(t, err.Error(), tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
