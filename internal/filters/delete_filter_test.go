package filters

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

// TestDeleteFilter_Success tests deleting a filter
func TestDeleteFilter_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, subscription := seedTestData(t, db)
	seedEventType(t, db, project.UID, "test.event")

	service := createFilterService(t, db)

	// Create a filter
	filter := &datastore.EventTypeFilter{
		SubscriptionID: subscription.UID,
		EventType:      "test.event",
		Headers:        datastore.M{},
		Body:           datastore.M{},
		RawHeaders:     datastore.M{},
		RawBody:        datastore.M{},
	}
	err := service.CreateFilter(ctx, filter)
	require.NoError(t, err)

	// Delete the filter
	err = service.DeleteFilter(ctx, filter.UID)

	require.NoError(t, err)

	// Verify deletion
	_, err = service.FindFilterByID(ctx, filter.UID)
	require.Error(t, err)
	require.Equal(t, datastore.ErrFilterNotFound, err)
}

// TestDeleteFilter_NotFound tests deleting non-existent filter
func TestDeleteFilter_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createFilterService(t, db)

	err := service.DeleteFilter(ctx, "non-existent-id")

	require.Error(t, err)
	require.Equal(t, datastore.ErrFilterNotFound, err)
}
