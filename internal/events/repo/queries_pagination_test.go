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
	// page_ids selects only IDs before hydrating wide event rows.
	start := strings.Index(upper, "WITH PAGE_IDS AS (")
	require.NotEqual(t, -1, start, "inner query must page by ID CTE first")
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

func TestPreviousPageGeneratedQueriesMatchSource(t *testing.T) {
	canonical := readQueriesSQL(t)
	comments := regexp.MustCompile(`(?m)--[^\n]*`)
	parameters := regexp.MustCompile(`@[A-Za-z_][A-Za-z_0-9]*|\$[0-9]+`)
	normalize := func(query string) string {
		query = comments.ReplaceAllString(query, "")
		query = parameters.ReplaceAllString(query, "?")
		return strings.Join(strings.Fields(strings.TrimSuffix(strings.TrimSpace(query), ";")), " ")
	}
	for name, generated := range map[string]string{
		"CountPrevEvents":       countPrevEvents,
		"CountPrevEventsSearch": countPrevEventsSearch,
	} {
		t.Run(name, func(t *testing.T) {
			source := extractNamedQuery(t, canonical, name)
			// extractNamedQuery begins after the name and leaves its sqlc command.
			source = source[strings.IndexByte(source, '\n')+1:]
			require.Equal(t, normalize(source), normalize(generated))
		})
	}
}
