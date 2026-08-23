package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderBatchReplayIncomplete(t *testing.T) {
	t.Run("409 after work landed", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		renderBatchReplayIncomplete(w, r, 2, 1, errors.New("failed to fetch event deliveries"))
		require.Equal(t, http.StatusConflict, w.Code)
		require.Contains(t, w.Body.String(), "2 successful, 1 failed")
		require.Contains(t, w.Body.String(), `"status":false`)
	})

	t.Run("retryable when only ownership skips counted", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		renderBatchReplayIncomplete(w, r, 0, 3, errors.New("failed to fetch event deliveries"))
		require.NotEqual(t, http.StatusConflict, w.Code)
		require.Contains(t, w.Body.String(), "failed to fetch event deliveries")
		require.Contains(t, w.Body.String(), `"status":false`)
	})

	t.Run("retryable when nothing ran", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		renderBatchReplayIncomplete(w, r, 0, 0, errors.New("failed to fetch event deliveries"))
		require.NotEqual(t, http.StatusConflict, w.Code)
	})
}
