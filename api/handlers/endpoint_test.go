package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestApplyPeriodFailureRates(t *testing.T) {
	tests := []struct {
		name        string
		counts      []datastore.EndpointStatusDeliveryCount
		wantRate    *float64
		wantSuccess *int64
		wantFailure *int64
		wantRetry   *int64
	}{
		{
			// No deliveries in range: rate and counts stay nil so the UI renders a
			// dash, distinct from a genuine 0%.
			name:   "no_counts_leaves_rate_nil",
			counts: nil,
		},
		{
			name: "success_only_yields_zero_rate",
			counts: []datastore.EndpointStatusDeliveryCount{
				{EndpointID: "ep1", Status: datastore.SuccessEventStatus, Count: 10},
			},
			wantRate:    f64(0),
			wantSuccess: i64(10),
			wantFailure: i64(0),
			wantRetry:   i64(0),
		},
		{
			name: "terminal_failures_counted",
			counts: []datastore.EndpointStatusDeliveryCount{
				{EndpointID: "ep1", Status: datastore.SuccessEventStatus, Count: 3},
				{EndpointID: "ep1", Status: datastore.FailureEventStatus, Count: 1},
			},
			wantRate:    f64(0.25),
			wantSuccess: i64(3),
			wantFailure: i64(1),
			wantRetry:   i64(0),
		},
		{
			// Regression: in-flight Retry deliveries have failed at least once, so
			// they count toward the rate instead of hiding an ongoing outage behind
			// a dash until retries exhaust.
			name: "retry_deliveries_count_as_failures",
			counts: []datastore.EndpointStatusDeliveryCount{
				{EndpointID: "ep1", Status: datastore.SuccessEventStatus, Count: 2},
				{EndpointID: "ep1", Status: datastore.FailureEventStatus, Count: 1},
				{EndpointID: "ep1", Status: datastore.RetryEventStatus, Count: 1},
			},
			wantRate:    f64(0.5),
			wantSuccess: i64(2),
			wantFailure: i64(1),
			wantRetry:   i64(1),
		},
		{
			name: "retry_only_yields_full_rate",
			counts: []datastore.EndpointStatusDeliveryCount{
				{EndpointID: "ep1", Status: datastore.RetryEventStatus, Count: 4},
			},
			wantRate:    f64(1),
			wantSuccess: i64(0),
			wantFailure: i64(0),
			wantRetry:   i64(4),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoints := []datastore.Endpoint{{UID: "ep1"}}

			applyPeriodFailureRates(endpoints, tt.counts)

			ep := endpoints[0]
			if tt.wantRate == nil {
				require.Nil(t, ep.PeriodFailureRate)
				require.Nil(t, ep.SuccessCount)
				require.Nil(t, ep.FailureCount)
				require.Nil(t, ep.RetryCount)
				return
			}

			require.NotNil(t, ep.PeriodFailureRate)
			require.InDelta(t, *tt.wantRate, *ep.PeriodFailureRate, 1e-9)
			require.Equal(t, tt.wantSuccess, ep.SuccessCount)
			require.Equal(t, tt.wantFailure, ep.FailureCount)
			require.Equal(t, tt.wantRetry, ep.RetryCount)
		})
	}
}

func TestApplyPeriodFailureRates_MultipleEndpoints(t *testing.T) {
	endpoints := []datastore.Endpoint{{UID: "ep1"}, {UID: "ep2"}, {UID: "ep3"}}
	counts := []datastore.EndpointStatusDeliveryCount{
		{EndpointID: "ep1", Status: datastore.SuccessEventStatus, Count: 9},
		{EndpointID: "ep1", Status: datastore.FailureEventStatus, Count: 1},
		{EndpointID: "ep2", Status: datastore.RetryEventStatus, Count: 2},
	}

	applyPeriodFailureRates(endpoints, counts)

	require.NotNil(t, endpoints[0].PeriodFailureRate)
	require.InDelta(t, 0.1, *endpoints[0].PeriodFailureRate, 1e-9)

	require.NotNil(t, endpoints[1].PeriodFailureRate)
	require.InDelta(t, 1.0, *endpoints[1].PeriodFailureRate, 1e-9)

	// ep3 had no deliveries; it keeps nil rate/counts.
	require.Nil(t, endpoints[2].PeriodFailureRate)
	require.Nil(t, endpoints[2].SuccessCount)
}

func f64(v float64) *float64 { return &v }

func i64(v int64) *int64 { return &v }
