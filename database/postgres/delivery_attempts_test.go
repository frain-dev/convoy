//go:build integration
// +build integration

package postgres

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"testing"
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

	edRepo := NewEventDeliveryRepo(db, nil)
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

	edRepo := NewEventDeliveryRepo(db, nil)
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
