package datastore

import (
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

func TestFromFilterStruct_SearchQueryWithoutDates(t *testing.T) {
	filter := FromFilterStruct(Filter{
		SearchParams: SearchParams{Query: "01ABC"},
	})

	search, ok := filter["SearchParams"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "01ABC", search["query"])
}
