package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestApplyEventListSearch(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	pickerStart := now.Add(-7 * 24 * time.Hour).Unix()
	pickerEnd := now.Unix()

	t.Run("empty search is a no-op", func(t *testing.T) {
		filter := &datastore.Filter{SearchParams: datastore.SearchParams{CreatedAtStart: pickerStart, CreatedAtEnd: pickerEnd}}
		require.NoError(t, ApplyEventListSearch(filter, nil, true, now))
		require.Equal(t, pickerStart, filter.SearchParams.CreatedAtStart)
	})

	t.Run("metadata query is allowed when licensed", func(t *testing.T) {
		filter := &datastore.Filter{
			Query:        "01HXYZ",
			SearchParams: datastore.SearchParams{CreatedAtStart: pickerStart, CreatedAtEnd: pickerEnd},
		}
		require.NoError(t, ApplyEventListSearch(filter, nil, true, now))
		require.Equal(t, pickerStart, filter.SearchParams.CreatedAtStart)
		require.True(t, filter.EventSearchLicensed)
	})

	t.Run("unlicensed search is rejected", func(t *testing.T) {
		filter := &datastore.Filter{Query: "payment"}
		err := ApplyEventListSearch(filter, nil, false, now)
		require.ErrorIs(t, err, ErrSearchUnlicensed)
	})

	t.Run("payload body allowed when licensed without search period", func(t *testing.T) {
		filter := &datastore.Filter{Body: json.RawMessage(`{"status":"paid"}`)}
		require.NoError(t, ApplyEventListSearch(filter, &datastore.Project{Config: &datastore.ProjectConfig{}}, true, now))
	})

	t.Run("invalid json body is rejected", func(t *testing.T) {
		filter := &datastore.Filter{Body: json.RawMessage(`not-json`)}
		err := ApplyEventListSearch(filter, nil, true, now)
		require.ErrorIs(t, err, ErrInvalidPayloadBody)
	})

	t.Run("json object query is promoted to body", func(t *testing.T) {
		filter := &datastore.Filter{Query: `{"status":"paid"}`}
		require.NoError(t, ApplyEventListSearch(filter, nil, true, now))
		require.Empty(t, filter.Query)
		require.JSONEq(t, `{"status":"paid"}`, string(filter.Body))
	})

	t.Run("json object query allowed when licensed without search period", func(t *testing.T) {
		filter := &datastore.Filter{Query: `{"status":"paid"}`}
		require.NoError(t, ApplyEventListSearch(filter, &datastore.Project{Config: &datastore.ProjectConfig{}}, true, now))
		require.JSONEq(t, `{"status":"paid"}`, string(filter.Body))
	})

	t.Run("mixed query splits leftover text and json object", func(t *testing.T) {
		filter := &datastore.Filter{Query: `invoice.paid {"status":"paid"}`}
		require.NoError(t, ApplyEventListSearch(filter, nil, true, now))
		require.Equal(t, "invoice.paid", filter.Query)
		require.JSONEq(t, `{"status":"paid"}`, string(filter.Body))
	})

	t.Run("query and body together stay as and inputs", func(t *testing.T) {
		filter := &datastore.Filter{
			Query: "invoice.paid",
			Body:  json.RawMessage(`{"status":"paid"}`),
		}
		require.NoError(t, ApplyEventListSearch(filter, nil, true, now))
		require.Equal(t, "invoice.paid", filter.Query)
		require.JSONEq(t, `{"status":"paid"}`, string(filter.Body))
	})

	t.Run("search window follows date filter only", func(t *testing.T) {
		filter := &datastore.Filter{
			Body:         json.RawMessage(`{"status":"paid"}`),
			SearchParams: datastore.SearchParams{CreatedAtStart: pickerStart, CreatedAtEnd: pickerEnd},
		}
		require.NoError(t, ApplyEventListSearch(filter, nil, true, now))
		require.Equal(t, pickerStart, filter.SearchParams.CreatedAtStart)
		require.Equal(t, pickerEnd, filter.SearchParams.CreatedAtEnd)
	})
}

func TestListSearchSQLFromFilter(t *testing.T) {
	t.Run("metadata query without payload scan", func(t *testing.T) {
		sql := ListSearchSQLFromFilter(&datastore.Filter{
			Query:               "payment",
			EventSearchLicensed: true,
		}, nil)
		require.True(t, sql.HasSearch)
		require.True(t, sql.HasQuery)
		require.False(t, sql.HasBody)
	})

	t.Run("body search", func(t *testing.T) {
		sql := ListSearchSQLFromFilter(&datastore.Filter{
			Body:                json.RawMessage(`{"status":"paid"}`),
			EventSearchLicensed: true,
		}, nil)
		require.True(t, sql.HasBody)
		require.JSONEq(t, `{"status":"paid"}`, string(sql.Body))
	})

	t.Run("json object query uses body containment only", func(t *testing.T) {
		filter := &datastore.Filter{
			Body:                json.RawMessage(`{"status":"paid"}`),
			EventSearchLicensed: true,
		}
		sql := ListSearchSQLFromFilter(filter, nil)
		require.True(t, sql.HasBody)
		require.False(t, sql.HasQuery)
		require.JSONEq(t, `{"status":"paid"}`, string(sql.Body))
	})

	t.Run("query and body both set", func(t *testing.T) {
		sql := ListSearchSQLFromFilter(&datastore.Filter{
			Query:               "invoice.paid",
			Body:                json.RawMessage(`{"status":"paid"}`),
			EventSearchLicensed: true,
		}, nil)
		require.True(t, sql.HasSearch)
		require.True(t, sql.HasQuery)
		require.True(t, sql.HasBody)
		require.Equal(t, "invoice.paid%", sql.SearchIDPrefix)
		require.JSONEq(t, `{"status":"paid"}`, string(sql.Body))
	})
}

func TestNeedsSearchTimeout(t *testing.T) {
	require.False(t, NeedsSearchTimeout(&datastore.Filter{
		Query:               "01HXYZ",
		EventSearchLicensed: true,
	}, nil))
	require.True(t, NeedsSearchTimeout(&datastore.Filter{
		Body:                json.RawMessage(`{"status":"paid"}`),
		EventSearchLicensed: true,
	}, nil))
	require.True(t, NeedsSearchTimeout(&datastore.Filter{
		Query:               "invoice.paid",
		Body:                json.RawMessage(`{"status":"paid"}`),
		EventSearchLicensed: true,
	}, nil))
}

func TestIsSearchTimeout(t *testing.T) {
	require.False(t, IsSearchTimeout(nil))
	require.True(t, IsSearchTimeout(context.DeadlineExceeded))
	require.True(t, IsSearchTimeout(&pgconn.PgError{Code: "57014"}))
	require.False(t, IsSearchTimeout(&pgconn.PgError{Code: "42P01"}))
}
