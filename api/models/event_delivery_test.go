package models

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryListEventDeliveryTransform_LookupParams(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		wantErr       string
		wantValue     string
		wantSources   []string
		wantMinTrials int64
	}{
		{
			name:        "idempotency value with sources",
			query:       "idempotencyValue=%20abc%20&sourceId=src_1&sourceId=src_2",
			wantValue:   "abc",
			wantSources: []string{"src_1", "src_2"},
		},
		{
			name:          "min trials scoped by idempotency value",
			query:         "idempotencyValue=abc&minTrials=2",
			wantValue:     "abc",
			wantMinTrials: 2,
		},
		{
			name:          "min trials scoped by endpoint",
			query:         "endpointId=ep_1&minTrials=3",
			wantMinTrials: 3,
		},
		{
			name:          "min trials scoped by event",
			query:         "eventId=evt_1&minTrials=2",
			wantMinTrials: 2,
		},
		{name: "min trials alone", query: "minTrials=2", wantErr: "minTrials requires"},
		{name: "min trials with only idempotency key", query: "idempotencyKey=hash&minTrials=2", wantErr: "minTrials requires"},
		{name: "min trials zero", query: "eventId=evt_1&minTrials=0", wantErr: "positive integer"},
		{name: "min trials not a number", query: "eventId=evt_1&minTrials=two", wantErr: "positive integer"},
		{name: "value with key", query: "idempotencyValue=abc&idempotencyKey=hash", wantErr: "cannot be combined"},
		{name: "source without value", query: "sourceId=src_1", wantErr: "sourceId requires idempotencyValue"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/eventdeliveries?"+tc.query, nil)

			var q *QueryListEventDelivery
			got, err := q.Transform(r)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantValue, got.IdempotencyValue)
			require.Equal(t, tc.wantSources, got.SourceIDs)
			require.Equal(t, tc.wantMinTrials, got.Filter.SearchParams.MinTrials)
			require.Empty(t, got.Filter.SearchParams.EventIDs)
		})
	}
}

func TestQueryListEventTransform_RejectsValueWithKey(t *testing.T) {
	r := httptest.NewRequest("GET", "/events?idempotencyValue=abc&idempotencyKey=hash", nil)

	var q *QueryListEvent
	_, err := q.Transform(r)
	require.ErrorContains(t, err, "cannot be combined")

	r = httptest.NewRequest("GET", "/events?idempotencyValue=abc&sourceId=src_1", nil)
	got, err := q.Transform(r)
	require.NoError(t, err)
	require.Equal(t, "abc", got.IdempotencyValue)
	require.Equal(t, []string{"src_1"}, got.Filter.SourceIDs)
}

func TestQueryListEventTransform_QueryMode(t *testing.T) {
	var q *QueryListEvent
	r := httptest.NewRequest("GET", "/events?query=invoice.paid&queryMode=text", nil)
	got, err := q.Transform(r)
	require.NoError(t, err)
	require.Equal(t, "text", got.Filter.QueryMode)
	require.Equal(t, "invoice.paid", got.Filter.Query)

	r = httptest.NewRequest("GET", "/events?queryMode=unknown", nil)
	_, err = q.Transform(r)
	require.ErrorContains(t, err, "queryMode must be text or omitted")
}
