package repo

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func readQueriesSQL(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "queries.sql")
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func extractNamedQuery(t *testing.T, sql, name string) string {
	t.Helper()
	marker := "-- name: " + name
	i := strings.Index(sql, marker)
	require.NotEqual(t, -1, i, "missing query %s", name)
	rest := sql[i+len(marker):]
	j := strings.Index(rest, "\n-- name:")
	if j == -1 {
		return rest
	}
	return rest[:j]
}

func innerCTEOrderBy(t *testing.T, query string) string {
	t.Helper()
	upper := strings.ToUpper(query)
	start := strings.Index(upper, "WITH FILTERED_EVENTS AS (")
	require.NotEqual(t, -1, start)
	limitIdx := strings.Index(upper[start:], "LIMIT")
	require.NotEqual(t, -1, limitIdx)
	cte := upper[start : start+limitIdx]
	orderIdx := strings.LastIndex(cte, "ORDER BY")
	require.NotEqual(t, -1, orderIdx)
	return strings.TrimSpace(cte[orderIdx:])
}

func TestLoadEventsPagedExistsInnerQueriesUsePlainOrderBy(t *testing.T) {
	sql := readQueriesSQL(t)

	desc := extractNamedQuery(t, sql, "LoadEventsPagedExistsInnerDesc")
	asc := extractNamedQuery(t, sql, "LoadEventsPagedExistsInnerAsc")

	descInner := innerCTEOrderBy(t, desc)
	ascInner := innerCTEOrderBy(t, asc)
	require.Equal(t, "ORDER BY EV.ID DESC", descInner)
	require.Equal(t, "ORDER BY EV.ID ASC", ascInner)

	caseOrder := regexp.MustCompile(`ORDER BY\s*\n\s*CASE\s+WHEN`)
	require.False(t, caseOrder.MatchString(descInner), "inner DESC scan must not use CASE ORDER BY")
	require.False(t, caseOrder.MatchString(ascInner), "inner ASC scan must not use CASE ORDER BY")

	require.NotContains(t, sql, "-- name: LoadEventsPagedExists :many")
}
