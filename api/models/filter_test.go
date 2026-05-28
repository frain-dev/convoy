package models

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func TestOptionalTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		wantSet  bool
		wantTime bool
	}{
		{
			name:     "omitted field is not set",
			payload:  `{}`,
			wantSet:  false,
			wantTime: false,
		},
		{
			name:     "null field is set without time",
			payload:  `{"enabled_at":null}`,
			wantSet:  true,
			wantTime: false,
		},
		{
			name:     "timestamp field is set with time",
			payload:  `{"enabled_at":"2026-05-28T00:00:00Z"}`,
			wantSet:  true,
			wantTime: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req UpdateFilterRequest

			err := json.Unmarshal([]byte(tt.payload), &req)

			require.NoError(t, err)
			require.Equal(t, tt.wantSet, req.EnabledAt.Set)
			require.Equal(t, tt.wantTime, req.EnabledAt.Time != nil)
		})
	}
}

func TestOptionalTimeMarshalJSON(t *testing.T) {
	t.Run("unset enabled_at is omitted from requests", func(t *testing.T) {
		req := CreateFilterRequest{EventType: "user.created"}

		payload, err := json.Marshal(req)

		require.NoError(t, err)
		require.NotContains(t, string(payload), "enabled_at")
	})

	t.Run("explicit null enabled_at is preserved", func(t *testing.T) {
		req := UpdateFilterRequest{EnabledAt: OptionalTime{Set: true}}

		payload, err := json.Marshal(req)

		require.NoError(t, err)
		require.Contains(t, string(payload), `"enabled_at":null`)
	})

	t.Run("timestamp enabled_at is encoded as json time", func(t *testing.T) {
		enabledAt := time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC)
		req := BulkUpdateFilterRequest{
			UID:       "filter-id",
			EnabledAt: OptionalTime{Set: true, Time: &enabledAt},
		}

		payload, err := json.Marshal(req)

		require.NoError(t, err)
		require.Contains(t, string(payload), `"enabled_at":"2026-05-28T01:02:03Z"`)
	})
}

func TestTestFilterRequestTransformAllowsScopedRequestsWithoutPayload(t *testing.T) {
	payloadField, ok := reflect.TypeOf(TestFilterRequest{}).FieldByName("Payload")
	require.True(t, ok)
	require.NotContains(t, payloadField.Tag.Get("validate"), "required")

	req := TestFilterRequest{
		Request: TestFilterRequestScopes{
			Headers: datastore.M{"x-event": "user.created"},
			Query:   datastore.M{"source": "partner"},
			Path:    datastore.M{"org": "acme"},
		},
	}

	got := req.Transform()

	require.Nil(t, got.Body)
	require.Equal(t, datastore.M{"x-event": "user.created"}, got.Headers)
	require.Equal(t, datastore.M{"source": "partner"}, got.Query)
	require.Equal(t, datastore.M{"org": "acme"}, got.Path)
}
