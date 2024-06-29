//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/pkg/httpheader"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
)

func Test_eventDeliveryRepo_CreateEventDelivery(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	ed := generateEventDelivery(project, endpoint, event, device, sub)

	edRepo := NewEventDeliveryRepo(db, nil)
	err := edRepo.CreateEventDelivery(context.Background(), ed)
	require.NoError(t, err)

	dbEventDelivery, err := edRepo.FindEventDeliveryByID(context.Background(), project.UID, ed.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbEventDelivery.CreatedAt)
	require.NotEmpty(t, dbEventDelivery.UpdatedAt)

	dbEventDelivery.CreatedAt, dbEventDelivery.UpdatedAt, dbEventDelivery.AcknowledgedAt = time.Time{}, time.Time{}, time.Time{}
	dbEventDelivery.Event, dbEventDelivery.Endpoint, dbEventDelivery.Source, dbEventDelivery.Device = nil, nil, nil, nil

	require.Equal(t, "", dbEventDelivery.Latency)

	require.Equal(t, ed.Metadata.NextSendTime.UTC(), dbEventDelivery.Metadata.NextSendTime.UTC())
	ed.Metadata.NextSendTime = time.Time{}
	dbEventDelivery.Metadata.NextSendTime = time.Time{}

	ed.CreatedAt, ed.UpdatedAt, ed.AcknowledgedAt = time.Time{}, time.Time{}, time.Time{}
	require.Equal(t, ed, dbEventDelivery)
}

func generateEventDelivery(project *datastore.Project, endpoint *datastore.Endpoint, event *datastore.Event, device *datastore.Device, sub *datastore.Subscription) *datastore.EventDelivery {
	e := &datastore.EventDelivery{
		UID:            ulid.Make().String(),
		ProjectID:      project.UID,
		EventID:        event.UID,
		EndpointID:     endpoint.UID,
		DeviceID:       device.UID,
		SubscriptionID: sub.UID,
		EventType:      event.EventType,
		Headers:        httpheader.HTTPHeader{"X-sig": []string{"3787 fmmfbf"}},
		DeliveryAttempts: []datastore.DeliveryAttempt{
			{UID: ulid.Make().String()},
		},
		URLQueryParams: "name=ref&category=food",
		Status:         datastore.SuccessEventStatus,
		Metadata: &datastore.Metadata{
			Data:            []byte(`{"name": "10x"}`),
			Raw:             `{"name": "10x"}`,
			Strategy:        datastore.ExponentialStrategyProvider,
			NextSendTime:    time.Now().Add(time.Hour),
			NumTrials:       1,
			IntervalSeconds: 10,
			RetryLimit:      20,
		},
		CLIMetadata: &datastore.CLIMetadata{},
		Description: "test",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return e
}

func Test_eventDeliveryRepo_FindEventDeliveriesByIDs(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	edRepo := NewEventDeliveryRepo(db, nil)
	edMap := map[string]*datastore.EventDelivery{}
	ids := []string{}
	for i := 0; i < 8; i++ {
		ed := generateEventDelivery(project, endpoint, event, device, sub)
		ed.Headers["uid"] = []string{ulid.Make().String()}
		if i == 0 || i == 1 || i == 5 {
			edMap[ed.UID] = ed
			ids = append(ids, ed.UID)
		}

		err := edRepo.CreateEventDelivery(context.Background(), ed)
		require.NoError(t, err)
	}

	dbEventDeliveries, err := edRepo.FindEventDeliveriesByIDs(context.Background(), project.UID, ids)
	require.NoError(t, err)
	require.Equal(t, 3, len(dbEventDeliveries))

	for i := range dbEventDeliveries {

		dbEventDelivery := &dbEventDeliveries[i]
		ed := edMap[dbEventDelivery.UID]

		require.NotEmpty(t, dbEventDelivery.CreatedAt)
		require.NotEmpty(t, dbEventDelivery.UpdatedAt)

		dbEventDelivery.CreatedAt, dbEventDelivery.UpdatedAt, dbEventDelivery.AcknowledgedAt = time.Time{}, time.Time{}, time.Time{}
		dbEventDelivery.Event, dbEventDelivery.Endpoint, dbEventDelivery.Source = nil, nil, nil

		require.Equal(t, ed.Metadata.NextSendTime.UTC(), dbEventDelivery.Metadata.NextSendTime.UTC())
		ed.Metadata.NextSendTime = time.Time{}
		dbEventDelivery.Metadata.NextSendTime = time.Time{}

		ed.CreatedAt, ed.UpdatedAt, ed.AcknowledgedAt = time.Time{}, time.Time{}, time.Time{}

		require.Equal(t, ed.Headers, dbEventDelivery.Headers)
		require.Equal(t, ed, dbEventDelivery)
	}
}

func Test_eventDeliveryRepo_FindEventDeliveriesByEventID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	edRepo := NewEventDeliveryRepo(db, nil)
	edMap := map[string]*datastore.EventDelivery{}

	mainEvent := seedEvent(t, db, project)
	for i := 0; i < 8; i++ {

		ed := generateEventDelivery(project, endpoint, seedEvent(t, db, project), device, sub)
		if i == 1 || i == 4 || i == 5 {
			ed.EventID = mainEvent.UID
			edMap[ed.UID] = ed
		}

		err := edRepo.CreateEventDelivery(context.Background(), ed)
		require.NoError(t, err)
	}

	dbEventDeliveries, err := edRepo.FindEventDeliveriesByEventID(context.Background(), project.UID, mainEvent.UID)
	require.NoError(t, err)
	require.Equal(t, 3, len(dbEventDeliveries))

	for i := range dbEventDeliveries {

		dbEventDelivery := &dbEventDeliveries[i]

		ed, ok := edMap[dbEventDelivery.UID]

		require.True(t, ok)

		require.NotEmpty(t, dbEventDelivery.CreatedAt)
		require.NotEmpty(t, dbEventDelivery.UpdatedAt)

		dbEventDelivery.CreatedAt, dbEventDelivery.UpdatedAt, dbEventDelivery.AcknowledgedAt = time.Time{}, time.Time{}, time.Time{}
		dbEventDelivery.Event, dbEventDelivery.Endpoint, dbEventDelivery.Source = nil, nil, nil

		require.Equal(t, ed.Metadata.NextSendTime.UTC(), dbEventDelivery.Metadata.NextSendTime.UTC())
		ed.Metadata.NextSendTime = time.Time{}
		dbEventDelivery.Metadata.NextSendTime = time.Time{}

		ed.CreatedAt, ed.UpdatedAt, ed.AcknowledgedAt = time.Time{}, time.Time{}, time.Time{}
		require.Equal(t, ed, dbEventDelivery)
	}
}

func Test_eventDeliveryRepo_CountDeliveriesByStatus(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	edRepo := NewEventDeliveryRepo(db, nil)

	status := datastore.FailureEventStatus
	for i := 0; i < 8; i++ {

		ed := generateEventDelivery(project, endpoint, event, device, sub)
		if i == 1 || i == 4 || i == 5 {
			ed.Status = status
		}

		err := edRepo.CreateEventDelivery(context.Background(), ed)
		require.NoError(t, err)
	}

	count, err := edRepo.CountDeliveriesByStatus(context.Background(), project.UID, status, datastore.SearchParams{
		CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
		CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
	})

	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func Test_eventDeliveryRepo_UpdateStatusOfEventDelivery(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	ed := generateEventDelivery(project, endpoint, event, device, sub)

	edRepo := NewEventDeliveryRepo(db, nil)
	err := edRepo.CreateEventDelivery(context.Background(), ed)
	require.NoError(t, err)

	err = edRepo.UpdateStatusOfEventDelivery(context.Background(), project.UID, *ed, datastore.RetryEventStatus)
	require.NoError(t, err)

	dbEventDelivery, err := edRepo.FindEventDeliveryByID(context.Background(), project.UID, ed.UID)
	require.NoError(t, err)

	require.Equal(t, datastore.RetryEventStatus, dbEventDelivery.Status)
}

func Test_eventDeliveryRepo_UpdateStatusOfEventDeliveries(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	ed1 := generateEventDelivery(project, endpoint, event, device, sub)
	ed2 := generateEventDelivery(project, endpoint, event, device, sub)

	edRepo := NewEventDeliveryRepo(db, nil)
	err := edRepo.CreateEventDelivery(context.Background(), ed1)
	require.NoError(t, err)

	err = edRepo.CreateEventDelivery(context.Background(), ed2)
	require.NoError(t, err)

	err = edRepo.UpdateStatusOfEventDeliveries(context.Background(), project.UID, []string{ed1.UID, ed2.UID}, datastore.RetryEventStatus)
	require.NoError(t, err)

	dbEventDeliveries, err := edRepo.FindEventDeliveriesByIDs(context.Background(), project.UID, []string{ed1.UID, ed2.UID})
	require.NoError(t, err)

	for _, d := range dbEventDeliveries {
		require.Equal(t, datastore.RetryEventStatus, d.Status)
	}
}

func Test_eventDeliveryRepo_FindDiscardedEventDeliveries(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	edRepo := NewEventDeliveryRepo(db, nil)

	status := datastore.DiscardedEventStatus
	for i := 0; i < 8; i++ {

		ed := generateEventDelivery(project, endpoint, event, device, sub)
		if i == 1 || i == 4 || i == 5 {
			ed.Status = status
		}

		err := edRepo.CreateEventDelivery(context.Background(), ed)
		require.NoError(t, err)
	}

	dbEventDeliveries, err := edRepo.FindDiscardedEventDeliveries(context.Background(), project.UID, device.UID, datastore.SearchParams{
		CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
		CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	for _, d := range dbEventDeliveries {
		require.Equal(t, datastore.DiscardedEventStatus, d.Status)
	}
}

func Test_eventDeliveryRepo_UpdateEventDeliveryWithAttempt(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	ed := generateEventDelivery(project, endpoint, event, device, sub)

	edRepo := NewEventDeliveryRepo(db, nil)
	err := edRepo.CreateEventDelivery(context.Background(), ed)
	require.NoError(t, err)

	newAttempt := datastore.DeliveryAttempt{
		UID: ulid.Make().String(),
	}

	latency := "1h2m"

	ed.Latency = latency

	err = edRepo.UpdateEventDeliveryWithAttempt(context.Background(), project.UID, *ed, newAttempt)
	require.NoError(t, err)

	dbEventDelivery, err := edRepo.FindEventDeliveryByID(context.Background(), project.UID, ed.UID)
	require.NoError(t, err)

	require.Equal(t, ed.DeliveryAttempts[0], dbEventDelivery.DeliveryAttempts[0])
	require.Equal(t, newAttempt, dbEventDelivery.DeliveryAttempts[1])
	require.Equal(t, latency, dbEventDelivery.Latency)
}

func Test_eventDeliveryRepo_CountEventDeliveries(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	ed1 := generateEventDelivery(project, endpoint, event, device, sub)
	ed2 := generateEventDelivery(project, endpoint, event, device, sub)

	edRepo := NewEventDeliveryRepo(db, nil)
	err := edRepo.CreateEventDelivery(context.Background(), ed1)
	require.NoError(t, err)

	err = edRepo.CreateEventDelivery(context.Background(), ed2)
	require.NoError(t, err)

	c, err := edRepo.CountEventDeliveries(context.Background(), project.UID, []string{ed1.EndpointID, ed2.EndpointID}, event.UID, []datastore.EventDeliveryStatus{datastore.SuccessEventStatus}, datastore.SearchParams{
		CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
		CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	require.Equal(t, int64(2), c)
}

func Test_eventDeliveryRepo_DeleteProjectEventDeliveries(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	// soft delete
	ed1 := generateEventDelivery(project, endpoint, event, device, sub)
	ed2 := generateEventDelivery(project, endpoint, event, device, sub)

	edRepo := NewEventDeliveryRepo(db, nil)
	err := edRepo.CreateEventDelivery(context.Background(), ed1)
	require.NoError(t, err)

	err = edRepo.CreateEventDelivery(context.Background(), ed2)
	require.NoError(t, err)

	err = edRepo.DeleteProjectEventDeliveries(context.Background(), project.UID, &datastore.EventDeliveryFilter{
		CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
		CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
	}, false)

	require.NoError(t, err)

	// hard delete

	ed1 = generateEventDelivery(project, endpoint, event, device, sub)
	ed2 = generateEventDelivery(project, endpoint, event, device, sub)

	err = edRepo.CreateEventDelivery(context.Background(), ed1)
	require.NoError(t, err)

	err = edRepo.CreateEventDelivery(context.Background(), ed2)
	require.NoError(t, err)

	err = edRepo.DeleteProjectEventDeliveries(context.Background(), project.UID, &datastore.EventDeliveryFilter{
		CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
		CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
	}, true)

	require.NoError(t, err)
}

func Test_eventDeliveryRepo_LoadEventDeliveriesPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)

	edRepo := NewEventDeliveryRepo(db, nil)
	edMap := map[string]*datastore.EventDelivery{}
	for i := 0; i < 8; i++ {
		ed := generateEventDelivery(project, endpoint, event, device, sub)
		edMap[ed.UID] = ed

		err := edRepo.CreateEventDelivery(context.Background(), ed)
		require.NoError(t, err)
	}

	dbEventDeliveries, _, err := edRepo.LoadEventDeliveriesPaged(
		context.Background(), project.UID, []string{endpoint.UID}, event.UID, sub.UID,
		[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus},
		datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
		datastore.Pageable{
			PerPage: 10,
		},
		"", "",
	)

	require.NoError(t, err)
	require.Equal(t, 8, len(dbEventDeliveries))

	for i := range dbEventDeliveries {

		dbEventDelivery := &dbEventDeliveries[i]
		ed := edMap[dbEventDelivery.UID]

		require.NotEmpty(t, dbEventDelivery.CreatedAt)
		require.NotEmpty(t, dbEventDelivery.UpdatedAt)

		dbEventDelivery.CreatedAt, dbEventDelivery.UpdatedAt, dbEventDelivery.AcknowledgedAt = time.Time{}, time.Time{}, time.Time{}

		require.Equal(t, event.EventType, dbEventDelivery.Event.EventType)
		require.Equal(t, endpoint.UID, dbEventDelivery.Endpoint.UID)
		dbEventDelivery.Event, dbEventDelivery.Endpoint, dbEventDelivery.Source, dbEventDelivery.Device = nil, nil, nil, nil

		require.Equal(t, ed.Metadata.NextSendTime.UTC(), dbEventDelivery.Metadata.NextSendTime.UTC())
		ed.Metadata.NextSendTime = time.Time{}
		dbEventDelivery.Metadata.NextSendTime = time.Time{}

		ed.CreatedAt, ed.UpdatedAt, ed.AcknowledgedAt = time.Time{}, time.Time{}, time.Time{}

		require.Equal(t, ed, dbEventDelivery)
	}

	evType := "file"
	event = seedEventWithEventType(t, db, project, evType)

	ed := generateEventDelivery(project, endpoint, event, device, sub)

	err = edRepo.CreateEventDeliveries(context.Background(), []*datastore.EventDelivery{ed})
	require.NoError(t, err)

	filteredDeliveries, _, err := edRepo.LoadEventDeliveriesPaged(
		context.Background(), project.UID, []string{endpoint.UID}, event.UID, sub.UID,
		[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus},
		datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Add(time.Hour).Unix(),
		},
		datastore.Pageable{
			PerPage: 10,
		},
		"", evType,
	)

	require.NoError(t, err)
	require.Equal(t, 1, len(filteredDeliveries))
	require.Equal(t, ed.UID, filteredDeliveries[0].UID)
}
