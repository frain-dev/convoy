package datastore

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromFilterStruct_PersistsSearchQuery(t *testing.T) {
	filter := FromFilterStruct(Filter{
		ProjectID: "proj-1",
		Query:     "invoice.paid",
		SearchParams: SearchParams{
			CreatedAtStart: 1704067200,
			CreatedAtEnd:   1704153600,
			Query:          "invoice.paid",
		},
	})

	search, ok := filter["SearchParams"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "invoice.paid", search["query"])
	require.Equal(t, "invoice.paid", filter["Query"])

	br := &BatchRetry{Filter: filter}
	got, err := br.GetFilter()
	require.NoError(t, err)
	require.Equal(t, "invoice.paid", got.SearchParams.Query)
	require.Equal(t, "invoice.paid", got.Query)
}

func TestFromFilterStruct_PersistsNarrowingSearchParams(t *testing.T) {
	filter := FromFilterStruct(Filter{
		ProjectID: "proj-1",
		SearchParams: SearchParams{
			EventIDs:  []string{"evt-1", "evt-2"},
			MinTrials: 2,
		},
	})

	// Round-trip through JSON the way the filter column is stored and read back.
	stored, err := json.Marshal(filter)
	require.NoError(t, err)
	var scanned RetryFilter
	require.NoError(t, scanned.Scan(stored))

	got, err := (&BatchRetry{Filter: scanned}).GetFilter()
	require.NoError(t, err)
	require.Equal(t, []string{"evt-1", "evt-2"}, got.SearchParams.EventIDs)
	require.Equal(t, int64(2), got.SearchParams.MinTrials)
}

func TestFromFilterStruct_SearchQueryWithoutDates(t *testing.T) {
	filter := FromFilterStruct(Filter{
		SearchParams: SearchParams{Query: "01ABC"},
	})

	search, ok := filter["SearchParams"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "01ABC", search["query"])
}
