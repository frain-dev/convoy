package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue/inventory"
	"github.com/stretchr/testify/require"
)

type inventoryReader struct {
	err   error
	calls int
}

func (r *inventoryReader) Inspect(context.Context) (inventory.Snapshot, error) {
	r.calls++
	return inventory.Snapshot{ObservedAt: time.Now()}, r.err
}

func TestQueueInventoryResponseKeepsFailedPreviousStoreVisible(t *testing.T) {
	active, previous := &inventoryReader{}, &inventoryReader{err: errors.New("postgres://private-credential@host/customer")}
	inv, err := inventory.New([]inventory.Registration{
		{ID: "active-store", Provider: "postgres", Role: "active", Reader: active},
		{ID: "previous-store", Provider: "redis", Role: "previous", Reader: previous},
	})
	require.NoError(t, err)
	h := newQueueHandler(t, nil, adminOnce(1))
	h.A.QueueInventory = inv
	recorder := httptest.NewRecorder()
	h.GetQueueStores(recorder, queueRequest("/ui/admin/queue/stores", nil, &datastore.User{UID: "user-1"}))
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	body := recorder.Body.String()
	require.Contains(t, body, `"connection":"unknown"`)
	require.Contains(t, body, `"snapshot":null`)
	require.Contains(t, body, `"inspection_failed"`)
	require.NotContains(t, body, "private-credential")
	require.Equal(t, 1, active.calls)
	require.Equal(t, 1, previous.calls)
}

func TestQueueInventoryDoesNotReadBeforeAdminAuthorization(t *testing.T) {
	reader := &inventoryReader{}
	inv, err := inventory.New([]inventory.Registration{{ID: "active", Provider: "redis", Role: "active", Reader: reader}})
	require.NoError(t, err)
	h := newQueueHandler(t, nil, nil)
	h.A.QueueInventory = inv
	recorder := httptest.NewRecorder()
	h.GetQueueStores(recorder, queueRequest("/ui/admin/queue/stores", nil, nil))
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Zero(t, reader.calls)
}
