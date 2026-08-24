package event_deliveries

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testListFilter() listFilter {
	return listFilter{
		ProjectID: "proj_1",
		Start:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		End:       time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
	}
}

func TestListFilterOmitsOptionalPredicates(t *testing.T) {
	sql, args := testListFilter().pageSQL(21, true)
	require.NotContains(t, sql, "CASE")
	require.NotContains(t, sql, "ed.status")
	require.NotContains(t, sql, "(ed.created_at, ed.id)")
	require.Contains(t, sql, "ORDER BY ed.created_at DESC, ed.id DESC")
	require.Len(t, args, 4) // project, start, end, limit
}

func TestListFilterEmitsRealStatusAndKeyset(t *testing.T) {
	f := testListFilter()
	f.Statuses = []string{"Retry"}
	f.HasKeyset = true
	f.KeysetAt = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	f.KeysetID = "del_1"
	f.KeysetOp = "<="

	sql, args := f.pageSQL(21, true)
	require.NotContains(t, sql, "CASE")
	require.Contains(t, sql, "unnest($1::text[])")
	require.Contains(t, sql, "JOIN LATERAL")
	require.Contains(t, sql, "ed.status = s.status")
	require.NotContains(t, sql, "ed.status = ANY($")
	require.Contains(t, sql, "(ed.created_at, ed.id) <= ($")
	require.Equal(t, []string{"Retry"}, args[0])
	require.Equal(t, "del_1", args[5])
}

func TestListFilterSearchIsExactIDOrTypeOrEndpoint(t *testing.T) {
	idFilter := testListFilter()
	idFilter.DeliveryID = "del_exact"
	sql, args := idFilter.pageSQL(21, true)
	require.Contains(t, sql, "ed.id = $")
	require.NotContains(t, sql, "ILIKE")
	require.Equal(t, "del_exact", args[3])

	orFilter := testListFilter()
	orFilter.TypePrefix = "pay%"
	orFilter.SearchEndpoints = []string{"ep_1"}
	sql, args = orFilter.pageSQL(21, false)
	require.Contains(t, sql, "ORDER BY ed.created_at ASC, ed.id ASC")
	require.Contains(t, sql, "ed.event_type ILIKE $")
	require.Contains(t, sql, "ev.event_type ILIKE $")
	require.Contains(t, sql, "ILIKE $")
	require.Contains(t, sql, "ed.endpoint_id = ANY($")
	require.True(t, strings.Contains(sql, " OR "))
	require.Equal(t, "pay%", args[3])
	require.Equal(t, []string{"ep_1"}, args[4])
}

func TestListFilterEventTypeUsesDisplayExpr(t *testing.T) {
	f := testListFilter()
	f.EventType = "pde996.bench"
	sql, args := f.pageSQL(21, true)
	require.Contains(t, sql, "EXISTS (SELECT 1 FROM convoy.events ev")
	require.Contains(t, sql, "ev.event_type = $")
	require.Equal(t, "pde996.bench", args[3])
}

func TestListFilterCountUsesSameWhere(t *testing.T) {
	f := testListFilter()
	f.Statuses = []string{"Success"}
	sql, args := f.countSQL()
	require.Contains(t, strings.ToUpper(sql), "SELECT COUNT(*)")
	require.Contains(t, sql, "ed.status = ANY($")
	require.NotContains(t, sql, "LIMIT")
	require.Equal(t, []string{"Success"}, args[3])
}

func TestListFilterExistsUsesPerStatusPage(t *testing.T) {
	f := testListFilter()
	f.Statuses = []string{"Failure", "Retry"}
	f.HasKeyset = true
	f.KeysetAt = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	f.KeysetID = "del_1"
	f.KeysetOp = ">"

	sql, args := f.existsSQL()
	require.Contains(t, strings.ToUpper(sql), "SELECT EXISTS")
	require.Contains(t, sql, "unnest($1::text[])")
	require.Contains(t, sql, "JOIN LATERAL")
	require.NotContains(t, sql, "COUNT(*)")
	require.Equal(t, []string{"Failure", "Retry"}, args[0])
	require.Equal(t, 1, args[len(args)-1])
}

func TestLooksLikeID(t *testing.T) {
	require.True(t, looksLikeID("01ARZ3NDEKTSV4RRFFQ69G5FAV"))
	require.True(t, looksLikeID("FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"))
	require.False(t, looksLikeID("payment.created"))
	require.False(t, looksLikeID("Invoice endpoint"))
	require.False(t, looksLikeID("short"))
}

func TestEscapeLike(t *testing.T) {
	require.Equal(t, `100\%`, escapeLike(`100%`))
	require.Equal(t, `a\_b`, escapeLike(`a_b`))
	require.Equal(t, `foo\\bar`, escapeLike(`foo\bar`))
}
