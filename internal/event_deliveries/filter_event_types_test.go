package event_deliveries

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

func TestCatalogFilterNamesOmitsWildcardDeprecatedAndEmpty(t *testing.T) {
	names := CatalogFilterNames([]datastore.ProjectEventType{
		{Name: "*"},
		{Name: ""},
		{Name: "  "},
		{Name: "invoice.paid"},
		{Name: " order.created "},
		{Name: "legacy.event", DeprecatedAt: null.TimeFrom(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))},
		{Name: "invoice.paid"},
	})
	require.Equal(t, []string{"invoice.paid", "order.created"}, names)
}

func TestGroupFilterEventTypesPutsOverlapInCatalogOnly(t *testing.T) {
	catalog, observed := GroupFilterEventTypes(
		[]datastore.ProjectEventType{
			{Name: "invoice.paid"},
			{Name: "*"},
			{Name: "order.created"},
			{Name: " invoice.paid "},
		},
		[]string{"invoice.paid", "canary.heartbeat", "*", "", "canary.heartbeat"},
	)
	require.Equal(t, []string{"invoice.paid", "order.created"}, catalog)
	require.Equal(t, []string{"canary.heartbeat"}, observed)
}

func TestGroupFilterEventTypesKeepsDeprecatedOutOfObserved(t *testing.T) {
	catalog, observed := GroupFilterEventTypes(
		[]datastore.ProjectEventType{
			{Name: "invoice.paid"},
			{Name: "legacy.event", DeprecatedAt: null.TimeFrom(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))},
		},
		[]string{"legacy.event", "canary.heartbeat"},
	)
	require.Equal(t, []string{"invoice.paid"}, catalog)
	require.Equal(t, []string{"canary.heartbeat"}, observed)
}

func TestGroupFilterEventTypesEmptyInputsAreEmptySlices(t *testing.T) {
	catalog, observed := GroupFilterEventTypes(nil, nil)
	require.NotNil(t, catalog)
	require.NotNil(t, observed)
	require.Empty(t, catalog)
	require.Empty(t, observed)
}

func TestObservedEventTypesSQLOmitsEndpointPredicateWhenUnscoped(t *testing.T) {
	sql, args := observedEventTypesSQL("proj_1", time.Unix(1, 0), time.Unix(2, 0), nil)
	require.NotContains(t, sql, "CASE")
	require.NotContains(t, sql, "endpoint_id")
	require.Contains(t, sql, "ed.event_type <> '*'")
	require.Contains(t, sql, "LIMIT 200")
	require.Equal(t, []any{"proj_1", time.Unix(1, 0), time.Unix(2, 0)}, args)
}

func TestObservedEventTypesSQLUsesRealEndpointAny(t *testing.T) {
	sql, args := observedEventTypesSQL("proj_1", time.Unix(1, 0), time.Unix(2, 0), []string{"ep_1"})
	require.NotContains(t, sql, "CASE")
	require.Contains(t, sql, "ed.endpoint_id = ANY($4::TEXT[])")
	require.Equal(t, []string{"ep_1"}, args[3])
}
