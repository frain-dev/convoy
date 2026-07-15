//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestCreateDeliveryAttempt(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	attemptsRepo := NewDeliveryAttemptRepo(db)
	ctx := context.Background()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)
	ed := generateEventDelivery(project, endpoint, event, device, sub)

	uid := ulid.Make().String()

	edRepo := NewEventDeliveryRepo(db)
	err := edRepo.CreateEventDelivery(ctx, ed)

	attempt := &datastore.DeliveryAttempt{
		UID:              uid,
		EventDeliveryId:  ed.UID,
		URL:              "https://example.com",
		Method:           "POST",
		ProjectId:        project.UID,
		EndpointID:       endpoint.UID,
		APIVersion:       "2024-01-01",
		IPAddress:        "192.0.0.1",
		RequestHeader:    map[string]string{"Content-Type": "application/json"},
		ResponseHeader:   map[string]string{"Content-Type": "application/json"},
		HttpResponseCode: "200",
		ResponseData:     []byte("{\"status\":\"ok\"}"),
		Status:           true,
	}

	err = attemptsRepo.CreateDeliveryAttempt(ctx, attempt)
	require.NoError(t, err)

	att, err := attemptsRepo.FindDeliveryAttemptById(ctx, ed.UID, uid)
	require.NoError(t, err)

	require.Equal(t, att.ResponseData, attempt.ResponseData)
}

func TestFindDeliveryAttempts(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	attemptsRepo := NewDeliveryAttemptRepo(db)
	ctx := context.Background()

	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)
	ed := generateEventDelivery(project, endpoint, event, device, sub)

	edRepo := NewEventDeliveryRepo(db)
	err := edRepo.CreateEventDelivery(ctx, ed)

	attempts := []datastore.DeliveryAttempt{
		{
			UID:              ulid.Make().String(),
			EventDeliveryId:  ed.UID,
			URL:              "https://example.com",
			Method:           "POST",
			EndpointID:       endpoint.UID,
			ProjectId:        project.UID,
			APIVersion:       "2024-01-01",
			IPAddress:        "192.168.0.1",
			RequestHeader:    map[string]string{"Content-Type": "application/json"},
			ResponseHeader:   map[string]string{"Content-Type": "application/json"},
			HttpResponseCode: "200",
			ResponseData:     []byte("{\"status\":\"ok\"}"),
			Status:           true,
		},
		{
			UID:              ulid.Make().String(),
			EventDeliveryId:  ed.UID,
			URL:              "https://main.com",
			Method:           "POST",
			EndpointID:       endpoint.UID,
			ProjectId:        project.UID,
			APIVersion:       "2024-04-04",
			IPAddress:        "127.0.0.1",
			RequestHeader:    map[string]string{"Content-Type": "application/json"},
			ResponseHeader:   map[string]string{"Content-Type": "application/json"},
			HttpResponseCode: "400",
			ResponseData:     []byte("{\"status\":\"Not Found\"}"),
			Error:            "",
			Status:           false,
		},
	}

	for _, a := range attempts {
		err = attemptsRepo.CreateDeliveryAttempt(ctx, &a)
		require.NoError(t, err)
	}

	atts, err := attemptsRepo.FindDeliveryAttempts(ctx, ed.UID)
	require.NoError(t, err)

	require.Equal(t, atts[0].ResponseData, attempts[0].ResponseData)
	require.Equal(t, atts[1].HttpResponseCode, attempts[1].HttpResponseCode)
}

// Old delivery + newer retry attempt: hard-delete attempts by delivery created_at,
// then hard-delete deliveries. Reproduces the Spruce retention FK failure mode;
// must succeed without delivery_attempts_event_delivery_id_fkey.
func TestDeleteProjectDeliveriesAttempts_HardDeleteByDeliveryCutoffClearsRetryAttempts(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	ctx := context.Background()
	source := seedSource(t, db)
	project := seedProject(t, db)
	device := seedDevice(t, db)
	endpoint := seedEndpoint(t, db)
	event := seedEvent(t, db, project)
	sub := seedSubscription(t, db, project, source, endpoint, device)
	ed := generateEventDelivery(project, endpoint, event, device, sub)

	edRepo := NewEventDeliveryRepo(db)
	attemptsRepo := NewDeliveryAttemptRepo(db)

	require.NoError(t, edRepo.CreateEventDelivery(ctx, ed))

	attemptUID := ulid.Make().String()
	require.NoError(t, attemptsRepo.CreateDeliveryAttempt(ctx, &datastore.DeliveryAttempt{
		UID:              attemptUID,
		EventDeliveryId:  ed.UID,
		URL:              "https://example.com",
		Method:           "POST",
		ProjectId:        project.UID,
		EndpointID:       endpoint.UID,
		APIVersion:       "2024-01-01",
		IPAddress:        "192.0.0.1",
		RequestHeader:    map[string]string{"Content-Type": "application/json"},
		ResponseHeader:   map[string]string{"Content-Type": "application/json"},
		HttpResponseCode: "500",
		ResponseData:     []byte(`{"status":"error"}`),
		Status:           false,
	}))

	policy := 7 * 24 * time.Hour
	cutoff := time.Now().Add(-policy)
	_, err := db.GetDB().ExecContext(ctx,
		`UPDATE convoy.event_deliveries SET created_at = $1 WHERE id = $2 AND project_id = $3`,
		time.Now().Add(-10*24*time.Hour), ed.UID, project.UID)
	require.NoError(t, err)
	_, err = db.GetDB().ExecContext(ctx,
		`UPDATE convoy.delivery_attempts SET created_at = $1 WHERE id = $2 AND project_id = $3`,
		time.Now().Add(-24*time.Hour), attemptUID, project.UID)
	require.NoError(t, err)

	filter := &datastore.DeliveryAttemptsFilter{
		CreatedAtStart: 0,
		CreatedAtEnd:   cutoff.Unix(),
	}
	err = attemptsRepo.DeleteProjectDeliveriesAttempts(ctx, project.UID, filter, true)
	require.NoError(t, err)

	err = edRepo.DeleteProjectEventDeliveries(ctx, project.UID, &datastore.EventDeliveryFilter{
		CreatedAtStart: 0,
		CreatedAtEnd:   cutoff.Unix(),
	}, true)
	require.NoError(t, err)

	_, err = attemptsRepo.FindDeliveryAttemptById(ctx, ed.UID, attemptUID)
	require.ErrorIs(t, err, datastore.ErrDeliveryAttemptNotFound)
}
