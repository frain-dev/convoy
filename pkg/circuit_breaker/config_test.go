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
	}{
		{
			name: "Valid Config",
			config: CircuitBreakerConfig{
				SampleRate:                  1,
				ErrorTimeout:                30,
				FailureThreshold:            0.5,
				FailureCount:                5,
				SuccessThreshold:            2,
				ObservabilityWindow:         5,
				NotificationThresholds:      []uint64{10, 20, 30},
				ConsecutiveFailureThreshold: 3,
			},
			wantErr: false,
		},
		{
			name: "Invalid SampleRate",
			config: CircuitBreakerConfig{
				SampleRate: 0,
			},
			wantErr: true,
		},
		{
			name: "Invalid ErrorTimeout",
			config: CircuitBreakerConfig{
				SampleRate:   1,
				ErrorTimeout: 0,
			},
			wantErr: true,
		},
		{
			name: "Invalid FailureThreshold",
			config: CircuitBreakerConfig{
				SampleRate:       1,
				ErrorTimeout:     30,
				FailureThreshold: 1.5,
			},
			wantErr: true,
		},
		{
			name: "Invalid FailureCount",
			config: CircuitBreakerConfig{
				SampleRate:       1,
				ErrorTimeout:     30,
				FailureThreshold: 0.5,
				FailureCount:     0,
			},
			wantErr: true,
		},
		{
			name: "Invalid SuccessThreshold",
			config: CircuitBreakerConfig{
				SampleRate:       1,
				ErrorTimeout:     30,
				FailureThreshold: 0.5,
				FailureCount:     5,
				SuccessThreshold: 0,
			},
			wantErr: true,
		},
		{
			name: "Invalid ObservabilityWindow",
			config: CircuitBreakerConfig{
				SampleRate:             1,
				ErrorTimeout:           30,
				FailureThreshold:       0.5,
				FailureCount:           5,
				SuccessThreshold:       2,
				ObservabilityWindow:    0,
				NotificationThresholds: []uint64{10, 20, 30},
			},
			wantErr: true,
		},
		{
			name: "Invalid NotificationThresholds",
			config: CircuitBreakerConfig{
				SampleRate:             1,
				ErrorTimeout:           30,
				FailureThreshold:       0.5,
				FailureCount:           5,
				SuccessThreshold:       2,
				ObservabilityWindow:    5,
				NotificationThresholds: []uint64{},
			},
			wantErr: true,
		},
		{
			name: "Invalid ConsecutiveFailureThreshold",
			config: CircuitBreakerConfig{
				SampleRate:                  1,
				ErrorTimeout:                30,
				FailureThreshold:            0.5,
				FailureCount:                5,
				SuccessThreshold:            2,
				ObservabilityWindow:         5,
				NotificationThresholds:      []uint64{10, 20, 30},
				ConsecutiveFailureThreshold: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
