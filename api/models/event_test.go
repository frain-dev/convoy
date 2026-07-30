package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateEventValidateRequiresDeliveryTarget(t *testing.T) {
	tests := []struct {
		name       string
		event      CreateEvent
		wantErrMsg string
	}{
		{
			name: "should_reject_event_with_no_endpoint_id_and_no_app_id",
			event: CreateEvent{
				EventType: "invoice.paid",
				Data:      json.RawMessage(`{"level":"test"}`),
			},
			wantErrMsg: "please provide an endpoint ID",
		},
		{
			name: "should_accept_event_with_endpoint_id",
			event: CreateEvent{
				EndpointID: "endpoint-id-1",
				EventType:  "invoice.paid",
				Data:       json.RawMessage(`{"level":"test"}`),
			},
		},
		{
			name: "should_accept_event_with_deprecated_app_id",
			event: CreateEvent{
				AppID:     "app-id-1",
				EventType: "invoice.paid",
				Data:      json.RawMessage(`{"level":"test"}`),
			},
		},
		{
			name: "should_still_reject_event_without_data",
			event: CreateEvent{
				EndpointID: "endpoint-id-1",
				EventType:  "invoice.paid",
			},
			wantErrMsg: "please provide your data",
		},
		{
			name: "should_still_reject_event_without_event_type",
			event: CreateEvent{
				EndpointID: "endpoint-id-1",
				Data:       json.RawMessage(`{"level":"test"}`),
			},
			wantErrMsg: "please provide an event type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.event.Validate()

			if tc.wantErrMsg != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErrMsg)
				return
			}

			require.NoError(t, err)
		})
	}
}
