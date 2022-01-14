package bolt

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Test_EventDeliveryRepo_CreateEventDelivery(t *testing.T) {
	database, err := New(config.Configuration{})
	if err != nil {
		require.NoError(t, err)
		return
	}

	defer database.Disconnect(context.Background())

	e := database.EventDeliveryRepo()

	type args struct {
		ctx      context.Context
		delivery *datastore.EventDelivery
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "should_create_event_delivery_successfully",
			args: args{
				ctx: context.Background(),
				delivery: &datastore.EventDelivery{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID:       uuid.NewString(),
						EventType: "",
					},
					EndpointMetadata: &datastore.EndpointMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID: uuid.NewString(),
					},
					Metadata: &datastore.Metadata{
						IntervalSeconds: 30,
						RetryLimit:      5,
					},
					CreatedAt:      0,
					UpdatedAt:      0,
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.CreateEventDelivery(tt.args.ctx, tt.args.delivery)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
		})
	}
}

func Test_eventDeliveryRepo_FindEventDeliveryByID(t *testing.T) {
	database, err := New(config.Configuration{})
	if err != nil {
		require.NoError(t, err)
		return
	}

	defer database.Disconnect(context.Background())

	e := database.EventDeliveryRepo()

	type args struct {
		ctx context.Context
		uid func(*datastore.EventDelivery) string
	}
	tests := []struct {
		name    string
		args    args
		want    *datastore.EventDelivery
		wantErr bool
	}{
		{
			name: "should_find_event_delivery_successfully",
			args: args{
				ctx: context.Background(),
				uid: func(d *datastore.EventDelivery) string {
					return d.UID
				},
			},
			want: &datastore.EventDelivery{
				UID:              uuid.NewString(),
				EventMetadata:    &datastore.EventMetadata{UID: uuid.NewString()},
				EndpointMetadata: nil,
				AppMetadata:      nil,
				Metadata:         nil,
				Description:      "abc",
				Status:           datastore.ProcessingEventStatus,
				DeliveryAttempts: nil,
				CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e.CreateEventDelivery(tt.args.ctx, tt.want)
			if err != nil {
				require.NoError(t, err)
				return
			}

			got, err := e.FindEventDeliveryByID(tt.args.ctx, tt.args.uid(tt.want))
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_eventDeliveryRepo_FindEventDeliveriesByIDs(t *testing.T) {
	database, err := New(config.Configuration{})
	if err != nil {
		require.NoError(t, err)
		return
	}

	defer database.Disconnect(context.Background())

	e := database.EventDeliveryRepo()

	delivery1 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err = e.CreateEventDelivery(context.Background(), &delivery1)
	if err != nil {
		require.NoError(t, err)
		return
	}

	delivery2 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err = e.CreateEventDelivery(context.Background(), &delivery2)
	if err != nil {
		require.NoError(t, err)
		return
	}

	got, err := e.FindEventDeliveriesByIDs(context.Background(), []string{delivery1.UID, delivery2.UID})
	if err != nil {
		require.NoError(t, err)
		return
	}

	require.Equal(t, 2, len(got))
	require.Equal(t, delivery1, got[0])
	require.Equal(t, delivery2, got[1])
}

func Test_eventDeliveryRepo_FindEventDeliveriesByEventID(t *testing.T) {
	database, err := New(config.Configuration{})
	if err != nil {
		require.NoError(t, err)
		return
	}

	defer database.Disconnect(context.Background())

	e := database.EventDeliveryRepo()

	eventID := uuid.NewString()

	delivery1 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       eventID,
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err = e.CreateEventDelivery(context.Background(), &delivery1)
	if err != nil {
		require.NoError(t, err)
		return
	}

	delivery2 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       eventID,
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err = e.CreateEventDelivery(context.Background(), &delivery2)
	if err != nil {
		require.NoError(t, err)
		return
	}

	got, err := e.FindEventDeliveriesByEventID(context.Background(), eventID)
	if err != nil {
		require.NoError(t, err)
		return
	}

	require.Equal(t, 2, len(got))
	require.Equal(t, delivery1, got[0])
	require.Equal(t, delivery2, got[1])
}

func Test_eventDeliveryRepo_UpdateStatusOfEventDelivery(t *testing.T) {
	database, err := New(config.Configuration{})
	if err != nil {
		require.NoError(t, err)
		return
	}

	defer database.Disconnect(context.Background())

	e := database.EventDeliveryRepo()

	delivery := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
		Status:      datastore.ScheduledEventStatus,
	}

	err = e.CreateEventDelivery(context.Background(), &delivery)
	if err != nil {
		require.NoError(t, err)
		return
	}

	status := datastore.SuccessEventStatus
	err = e.UpdateStatusOfEventDelivery(context.Background(), delivery, status)
	if err != nil {
		require.NoError(t, err)
		return
	}

	d, err := e.FindEventDeliveryByID(context.Background(), delivery.UID)
	if err != nil {
		require.NoError(t, err)
		return
	}

	require.Equal(t, status, d.Status)
}

func Test_eventDeliveryRepo_LoadEventDeliveriesPaged(t *testing.T) {
	database, err := New(config.Configuration{})
	if err != nil {
		require.NoError(t, err)
		return
	}

	defer database.Disconnect(context.Background())

	e := database.EventDeliveryRepo()

	delivery1 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err = e.CreateEventDelivery(context.Background(), &delivery1)
	if err != nil {
		require.NoError(t, err)
		return
	}

	delivery2 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err = e.CreateEventDelivery(context.Background(), &delivery2)
	if err != nil {
		require.NoError(t, err)
		return
	}

	deliveries, _, err := e.LoadEventDeliveriesPaged(context.Background(), "", "", "", nil, models.SearchParams{}, models.Pageable{PerPage: 10, Page: 3})
	if err != nil {
		require.NoError(t, err)
		return
	}
	require.LessOrEqual(t, 2, len(deliveries))
}
