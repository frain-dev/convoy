package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestRenderEventDeliveriesTimeout(t *testing.T) {
	t.Run("deadline exceeded is 504", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		require.True(t, renderEventDeliveriesTimeout(w, r, context.DeadlineExceeded))
		require.Equal(t, http.StatusGatewayTimeout, w.Code)
		require.Contains(t, w.Body.String(), eventDeliveriesTimeoutMsg)
	})

	t.Run("postgres query canceled is 504", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		require.True(t, renderEventDeliveriesTimeout(w, r, &pgconn.PgError{Code: "57014"}))
		require.Equal(t, http.StatusGatewayTimeout, w.Code)
	})

	t.Run("other errors stay with the caller", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		require.False(t, renderEventDeliveriesTimeout(w, r, errors.New("connection refused")))
		require.Equal(t, http.StatusOK, w.Code)
		require.Empty(t, w.Body.String())
	})

	t.Run("nil is not a timeout", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		require.False(t, renderEventDeliveriesTimeout(w, r, nil))
	})
}
