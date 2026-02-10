package meta_events

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

// ============================================================================
// LoadMetaEventsPaged Tests
// ============================================================================

func TestLoadMetaEventsPaged_ForwardPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create multiple meta events
	_ = seedMultipleMetaEvents(t, db, project, 10)

	// Load first page
	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	metaEvents, pagination, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents, 5)
	require.True(t, pagination.HasNextPage)

	// Load second page
	filter.Pageable.NextCursor = pagination.NextPageCursor
	metaEvents2, pagination2, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents2, 5)
	require.True(t, pagination2.HasPreviousPage)
}

func TestLoadMetaEventsPaged_BackwardPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create multiple meta events
	_ = seedMultipleMetaEvents(t, db, project, 10)

	// Load first page (forward)
	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	metaEvents, pagination, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents, 5)

	// Load second page (forward)
	filter.Pageable.NextCursor = pagination.NextPageCursor
	_, pagination2, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)

	// Load previous page (backward)
	filter.Pageable.Direction = datastore.Prev
	filter.Pageable.PrevCursor = pagination2.PrevPageCursor
	metaEvents3, _, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents3, 5)
}

func TestLoadMetaEventsPaged_DateFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create meta events
	_ = seedMultipleMetaEvents(t, db, project, 5)

	// Load with date filter that includes all events
	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	metaEvents, _, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents, 5)

	// Load with date filter that excludes all events (future dates)
	filter.SearchParams.CreatedAtStart = time.Now().Add(time.Hour).Unix()
	filter.SearchParams.CreatedAtEnd = time.Now().Add(2 * time.Hour).Unix()

	metaEvents2, _, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents2, 0)
}

func TestLoadMetaEventsPaged_EmptyResults(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Load without creating any meta events
	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	metaEvents, pagination, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents, 0)
	require.False(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
}

func TestLoadMetaEventsPaged_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create meta events for the project
	_ = seedMultipleMetaEvents(t, db, project, 5)

	// Load with a different project ID
	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	metaEvents, _, err := service.LoadMetaEventsPaged(ctx, ulid.Make().String(), filter)
	require.NoError(t, err)
	require.Len(t, metaEvents, 0)
}

func TestLoadMetaEventsPaged_SingleResult(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create a single meta event
	_ = seedMetaEvent(t, db, project)

	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	metaEvents, pagination, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents, 1)
	require.False(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
}

func TestLoadMetaEventsPaged_ExactPageSize(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create exactly 5 meta events
	_ = seedMultipleMetaEvents(t, db, project, 5)

	// Load with page size of 5
	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	metaEvents, pagination, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents, 5)
	require.False(t, pagination.HasNextPage)
}

func TestLoadMetaEventsPaged_VerifyOrder(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create multiple meta events
	created := seedMultipleMetaEvents(t, db, project, 5)

	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   10,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	metaEvents, _, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)
	require.Len(t, metaEvents, 5)

	// Verify DESC order (newest first based on ID, since ULID is time-ordered)
	// The last created event should have the highest ID
	require.Equal(t, created[len(created)-1].UID, metaEvents[0].UID)
}

func TestLoadMetaEventsPaged_PaginationMetadata(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createMetaEventService(t, db)

	// Create multiple meta events
	_ = seedMultipleMetaEvents(t, db, project, 15)

	// Load first page
	filter := &datastore.Filter{
		Pageable: datastore.Pageable{
			PerPage:   5,
			Direction: datastore.Next,
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: 0,
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
	}
	filter.Pageable.SetCursors()

	_, pagination, err := service.LoadMetaEventsPaged(ctx, project.UID, filter)
	require.NoError(t, err)

	require.Equal(t, int64(5), pagination.PerPage)
	require.True(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
	require.NotEmpty(t, pagination.NextPageCursor)
}
