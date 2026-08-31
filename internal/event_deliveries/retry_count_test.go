package event_deliveries

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
)

var retryCountStatuses = []datastore.EventDeliveryStatus{
	datastore.ScheduledEventStatus,
	datastore.ProcessingEventStatus,
	datastore.RetryEventStatus,
	datastore.FailureEventStatus,
	datastore.SuccessEventStatus,
	datastore.DiscardedEventStatus,
}

func TestCountRetryCandidates_EmptyProjectIDsIsZero(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()
	fix := seedRetryCountFixture(t, db)

	d := createTestEventDelivery(t, fix.projects[0].UID, fix.events[0].UID, fix.endpoints[0].UID, fix.subs[0].UID)
	d.Status = datastore.ScheduledEventStatus
	require.NoError(t, service.CreateEventDelivery(ctx, d))

	n, err := service.CountRetryCandidates(ctx, nil, []datastore.EventDeliveryStatus{datastore.ScheduledEventStatus}, "", defaultSearchParams())
	require.NoError(t, err)
	require.Equal(t, int64(0), n)
}

func TestCountRetryCandidates_AllStatusCombos(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()
	fix := seedRetryCountFixture(t, db)

	for _, status := range retryCountStatuses {
		for i, project := range fix.projects {
			d := createTestEventDelivery(t, project.UID, fix.events[i].UID, fix.endpoints[i].UID, fix.subs[i].UID)
			d.Status = status
			require.NoError(t, service.CreateEventDelivery(ctx, d))
		}
	}

	ids := []string{fix.projects[0].UID, fix.projects[1].UID}
	params := defaultSearchParams()

	for _, combo := range statusSubsets(retryCountStatuses) {
		got, err := service.CountRetryCandidates(ctx, ids, combo, "", params)
		require.NoError(t, err, "statuses=%v", combo)
		require.Equal(t, int64(2*len(combo)), got, "statuses=%v", combo)
	}

	// Walk only singles plus the full set. Paging every subset is the same
	// walk, and a missing HasNextPage stop used to hang on a one-row page.
	walkCombos := make([][]datastore.EventDeliveryStatus, 0, len(retryCountStatuses)+1)
	for _, status := range retryCountStatuses {
		walkCombos = append(walkCombos, []datastore.EventDeliveryStatus{status})
	}
	walkCombos = append(walkCombos, append([]datastore.EventDeliveryStatus{}, retryCountStatuses...))
	for _, combo := range walkCombos {
		got, err := service.CountRetryCandidates(ctx, ids, combo, "", params)
		require.NoError(t, err, "walk statuses=%v", combo)
		walked, err := walkRetryCandidates(ctx, service, ids, combo, "", params)
		require.NoError(t, err, "walk statuses=%v", combo)
		require.Equal(t, walked, got, "count must match the retry page walk; statuses=%v", combo)
	}
}

func TestCountRetryCandidates_Lookback(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()
	fix := seedRetryCountFixture(t, db)
	now := time.Now()

	inside := createTestEventDelivery(t, fix.projects[0].UID, fix.events[0].UID, fix.endpoints[0].UID, fix.subs[0].UID)
	inside.Status = datastore.ScheduledEventStatus
	require.NoError(t, service.CreateEventDelivery(ctx, inside))
	setDeliveryCreatedAt(t, db, inside.UID, now.Add(-30*time.Minute))

	mid := createTestEventDelivery(t, fix.projects[1].UID, fix.events[1].UID, fix.endpoints[1].UID, fix.subs[1].UID)
	mid.Status = datastore.ScheduledEventStatus
	require.NoError(t, service.CreateEventDelivery(ctx, mid))
	setDeliveryCreatedAt(t, db, mid.UID, now.Add(-2*time.Hour))

	old := createTestEventDelivery(t, fix.projects[0].UID, fix.events[0].UID, fix.endpoints[0].UID, fix.subs[0].UID)
	old.Status = datastore.ScheduledEventStatus
	require.NoError(t, service.CreateEventDelivery(ctx, old))
	setDeliveryCreatedAt(t, db, old.UID, now.Add(-25*time.Hour))

	ids := []string{fix.projects[0].UID, fix.projects[1].UID}
	statuses := []datastore.EventDeliveryStatus{datastore.ScheduledEventStatus}

	oneHour, err := service.CountRetryCandidates(ctx, ids, statuses, "", lookbackParams(now, time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(1), oneHour)

	fiveHours, err := service.CountRetryCandidates(ctx, ids, statuses, "", lookbackParams(now, 5*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(2), fiveHours)

	day, err := service.CountRetryCandidates(ctx, ids, statuses, "", lookbackParams(now, 24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(2), day)

	twoDays, err := service.CountRetryCandidates(ctx, ids, statuses, "", lookbackParams(now, 48*time.Hour))
	require.NoError(t, err)
	require.Equal(t, int64(3), twoDays)
}

func TestCountRetryCandidates_EventIDFilter(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()
	fix := seedRetryCountFixture(t, db)

	wanted := createTestEventDelivery(t, fix.projects[0].UID, fix.events[0].UID, fix.endpoints[0].UID, fix.subs[0].UID)
	wanted.Status = datastore.FailureEventStatus
	require.NoError(t, service.CreateEventDelivery(ctx, wanted))

	other := createTestEventDelivery(t, fix.projects[1].UID, fix.events[1].UID, fix.endpoints[1].UID, fix.subs[1].UID)
	other.Status = datastore.FailureEventStatus
	require.NoError(t, service.CreateEventDelivery(ctx, other))

	ids := []string{fix.projects[0].UID, fix.projects[1].UID}
	statuses := []datastore.EventDeliveryStatus{datastore.FailureEventStatus}

	all, err := service.CountRetryCandidates(ctx, ids, statuses, "", defaultSearchParams())
	require.NoError(t, err)
	require.Equal(t, int64(2), all)

	filtered, err := service.CountRetryCandidates(ctx, ids, statuses, fix.events[0].UID, defaultSearchParams())
	require.NoError(t, err)
	require.Equal(t, int64(1), filtered)

	walked, err := walkRetryCandidates(ctx, service, ids, statuses, fix.events[0].UID, defaultSearchParams())
	require.NoError(t, err)
	require.Equal(t, walked, filtered)

	none, err := service.CountRetryCandidates(ctx, ids, statuses, "evt_missing", defaultSearchParams())
	require.NoError(t, err)
	require.Equal(t, int64(0), none)
}

func TestCountRetryCandidates_SingleRowWalkTerminates(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()
	fix := seedRetryCountFixture(t, db)

	d := createTestEventDelivery(t, fix.projects[0].UID, fix.events[0].UID, fix.endpoints[0].UID, fix.subs[0].UID)
	d.Status = datastore.ScheduledEventStatus
	require.NoError(t, service.CreateEventDelivery(ctx, d))

	ids := []string{fix.projects[0].UID}
	statuses := []datastore.EventDeliveryStatus{datastore.ScheduledEventStatus}
	done := make(chan struct{})
	var walked int64
	var walkErr error
	go func() {
		defer close(done)
		walked, walkErr = walkRetryCandidates(ctx, service, ids, statuses, "", defaultSearchParams())
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("walkRetryCandidates hung on a single-row page")
	}
	require.NoError(t, walkErr)
	require.Equal(t, int64(1), walked)
}

func TestCountRetryCandidates_OneProjectScope(t *testing.T) {
	service, db := setupTestDB(t)
	ctx := context.Background()
	fix := seedRetryCountFixture(t, db)

	for i, project := range fix.projects {
		d := createTestEventDelivery(t, project.UID, fix.events[i].UID, fix.endpoints[i].UID, fix.subs[i].UID)
		d.Status = datastore.RetryEventStatus
		require.NoError(t, service.CreateEventDelivery(ctx, d))
	}

	statuses := []datastore.EventDeliveryStatus{datastore.RetryEventStatus}
	one, err := service.CountRetryCandidates(ctx, []string{fix.projects[0].UID}, statuses, "", defaultSearchParams())
	require.NoError(t, err)
	require.Equal(t, int64(1), one)
}

type retryCountFixture struct {
	projects  []*datastore.Project
	endpoints []*datastore.Endpoint
	subs      []*datastore.Subscription
	events    []*datastore.Event
}

func seedRetryCountFixture(t *testing.T, db database.Database) retryCountFixture {
	t.Helper()
	var fix retryCountFixture
	for i := 0; i < 2; i++ {
		project := seedTestProject(t, db)
		endpoint := seedTestEndpoint(t, db, project.UID)
		source := seedTestSource(t, db, project.UID)
		sub := seedSubscription(t, db, project.UID, endpoint.UID, source.UID)
		event := seedEvent(t, db, project.UID, endpoint.UID, source.UID)
		fix.projects = append(fix.projects, project)
		fix.endpoints = append(fix.endpoints, endpoint)
		fix.subs = append(fix.subs, sub)
		fix.events = append(fix.events, event)
	}
	return fix
}

func lookbackParams(now time.Time, window time.Duration) datastore.SearchParams {
	return datastore.SearchParams{
		CreatedAtStart: now.Add(-window).Unix(),
		CreatedAtEnd:   now.Unix(),
	}
}

func setDeliveryCreatedAt(t *testing.T, db database.Database, id string, at time.Time) {
	t.Helper()
	_, err := db.GetDB().ExecContext(context.Background(),
		"UPDATE convoy.event_deliveries SET created_at=$1, updated_at=$1 WHERE id=$2", at, id)
	require.NoError(t, err)
}

func statusSubsets(all []datastore.EventDeliveryStatus) [][]datastore.EventDeliveryStatus {
	out := make([][]datastore.EventDeliveryStatus, 0, 1<<len(all)-1)
	for mask := 1; mask < 1<<len(all); mask++ {
		var combo []datastore.EventDeliveryStatus
		for i, status := range all {
			if mask&(1<<i) != 0 {
				combo = append(combo, status)
			}
		}
		out = append(out, combo)
	}
	return out
}

func walkRetryCandidates(ctx context.Context, service *Service, projectIDs []string, statuses []datastore.EventDeliveryStatus, eventID string, params datastore.SearchParams) (int64, error) {
	var total int64
	for _, status := range statuses {
		for _, projectID := range projectIDs {
			pageable := datastore.Pageable{
				Direction: datastore.Next,
				PerPage:   1000,
			}
			for {
				deliveries, pagination, err := service.LoadEventDeliveriesPaged(ctx, projectID, nil, eventID, "", []datastore.EventDeliveryStatus{status}, params, pageable, "", "", "")
				if err != nil {
					return 0, err
				}
				if len(deliveries) == 0 {
					break
				}
				total += int64(len(deliveries))
				if !pagination.HasNextPage {
					break
				}
				pageable.NextCursor = pagination.NextPageCursor
			}
		}
	}
	return total, nil
}
