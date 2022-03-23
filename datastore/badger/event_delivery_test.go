//go:build integration
// +build integration

package badger

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Test_EventDeliveryRepo_CreateEventDelivery(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	e := NewEventDeliveryRepository(db)

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
	db, closeFn := getDB(t)
	defer closeFn()

	e := NewEventDeliveryRepository(db)

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
	db, closeFn := getDB(t)
	defer closeFn()

	e := NewEventDeliveryRepository(db)

	delivery1 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err := e.CreateEventDelivery(context.Background(), &delivery1)
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
	require.Contains(t, got, delivery1)
	require.Contains(t, got, delivery2)
}

func Test_eventDeliveryRepo_FindEventDeliveriesByEventID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	e := NewEventDeliveryRepository(db)

	eventID := uuid.NewString()

	delivery1 := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       eventID,
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
	}

	err := e.CreateEventDelivery(context.Background(), &delivery1)
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
	for _, delivery := range got {
		require.Equal(t, delivery.EventMetadata.UID, eventID)
	}
}

func Test_eventDeliveryRepo_UpdateStatusOfEventDelivery(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	e := NewEventDeliveryRepository(db)

	delivery := datastore.EventDelivery{
		UID: uuid.NewString(),
		EventMetadata: &datastore.EventMetadata{
			UID:       uuid.NewString(),
			EventType: "*",
		},
		AppMetadata: &datastore.AppMetadata{UID: uuid.NewString()},
		Status:      datastore.ScheduledEventStatus,
	}

	err := e.CreateEventDelivery(context.Background(), &delivery)
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
	ctx := context.Background()

	type args struct {
		groupID      string
		appID        string
		eventID      string
		status       []datastore.EventDeliveryStatus
		searchParams datastore.SearchParams
		pageable     datastore.Pageable
	}

	tests := []struct {
		name               string
		args               args
		eventDeliveries    []datastore.EventDelivery
		wantCount          int
		wantPaginationData datastore.PaginationData
		wantErr            bool
	}{
		{
			name: "should_filter_event_deliveries_by_app_id_successfully",
			args: args{
				groupID: "",
				appID:   "123",
				eventID: "",
				status:  nil,
				searchParams: datastore.SearchParams{
					CreatedAtStart: 0,
					CreatedAtEnd:   0,
				},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    0,
				},
			},
			eventDeliveries: []datastore.EventDelivery{
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: "abcd",
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     "123",
						GroupID: "abc",
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: "daniel",
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     "123",
						GroupID: "junior",
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			wantCount: 2,
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 1,
			},
			wantErr: false,
		},
		{
			name: "should_filter_event_deliveries_by_group_id_successfully",
			args: args{
				groupID: "group2",
				appID:   "",
				eventID: "",
				status:  nil,
				searchParams: datastore.SearchParams{
					CreatedAtStart: 0,
					CreatedAtEnd:   0,
				},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    0,
				},
			},
			eventDeliveries: []datastore.EventDelivery{
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: "group2",
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: "group2",
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			wantCount: 2,
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 1,
			},
			wantErr: false,
		},
		{
			name: "should_filter_event_deliveries_by_event_id_successfully",
			args: args{
				groupID: "",
				appID:   "",
				eventID: "event3",
				status:  nil,
				searchParams: datastore.SearchParams{
					CreatedAtStart: 0,
					CreatedAtEnd:   0,
				},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    0,
				},
			},
			eventDeliveries: []datastore.EventDelivery{
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: "event3",
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: "event3",
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			wantCount: 2,
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 1,
			},
			wantErr: false,
		},
		{
			name: "should_filter_event_deliveries_by_status_successfully",
			args: args{
				groupID: "",
				appID:   "",
				eventID: "",
				status:  []datastore.EventDeliveryStatus{datastore.ProcessingEventStatus, datastore.ScheduledEventStatus},
				searchParams: datastore.SearchParams{
					CreatedAtStart: 0,
					CreatedAtEnd:   0,
				},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    0,
				},
			},
			eventDeliveries: []datastore.EventDelivery{
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ProcessingEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.RetryEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.DiscardedEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.FailureEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			wantCount: 2,
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 1,
			},
			wantErr: false,
		},
		{
			name: "should_filter_event_deliveries_by_status_successfully",
			args: args{
				groupID: "",
				appID:   "",
				eventID: "",
				status:  []datastore.EventDeliveryStatus{datastore.ProcessingEventStatus, datastore.ScheduledEventStatus},
				searchParams: datastore.SearchParams{
					CreatedAtStart: 0,
					CreatedAtEnd:   0,
				},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    0,
				},
			},
			eventDeliveries: []datastore.EventDelivery{
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ProcessingEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.RetryEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.DiscardedEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.FailureEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			wantCount: 2,
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 1,
			},
			wantErr: false,
		},
		{
			name: "should_filter_event_deliveries_by_status_successfully",
			args: args{
				groupID: "",
				appID:   "",
				eventID: "",
				status:  nil,
				searchParams: datastore.SearchParams{
					CreatedAtStart: time.Date(2021, time.January, 1, 0, 0, 0, 0, time.Local).Unix(),
					CreatedAtEnd:   time.Date(2021, time.August, 1, 0, 0, 0, 0, time.Local).Unix(),
				},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    0,
				},
			},
			eventDeliveries: []datastore.EventDelivery{
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.February, 1, 0, 0, 0, 0, time.Local)),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.February, 1, 0, 0, 0, 0, time.Local)),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ProcessingEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.March, 1, 0, 0, 0, 0, time.Local)),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.March, 1, 0, 0, 0, 0, time.Local)),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.RetryEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local)),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local)),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.DiscardedEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.FailureEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			wantCount: 3,
			wantPaginationData: datastore.PaginationData{
				Total:     3,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 1,
			},
			wantErr: false,
		},
		{
			name: "should_fetch_event_deliveries_with_correct_pagination_data",
			args: args{
				groupID:      "",
				appID:        "",
				eventID:      "",
				status:       nil,
				searchParams: datastore.SearchParams{},
				pageable: datastore.Pageable{
					Page:    4,
					PerPage: 1,
					Sort:    0,
				},
			},
			eventDeliveries: []datastore.EventDelivery{
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ScheduledEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.February, 1, 0, 0, 0, 0, time.Local)),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.February, 1, 0, 0, 0, 0, time.Local)),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.ProcessingEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.March, 1, 0, 0, 0, 0, time.Local)),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.March, 1, 0, 0, 0, 0, time.Local)),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.RetryEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local)),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.June, 1, 0, 0, 0, 0, time.Local)),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: "uuid.NewString()",
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.DiscardedEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					ID:  primitive.NewObjectID(),
					UID: uuid.NewString(),
					EventMetadata: &datastore.EventMetadata{
						UID: uuid.NewString(),
					},
					AppMetadata: &datastore.AppMetadata{
						UID:     uuid.NewString(),
						GroupID: uuid.NewString(),
					},
					Status:         datastore.FailureEventStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			wantCount: 1,
			wantPaginationData: datastore.PaginationData{
				Total:     5,
				Page:      4,
				PerPage:   1,
				Prev:      3,
				Next:      5,
				TotalPage: 5,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			e := NewEventDeliveryRepository(db)

			var err error
			for _, delivery := range tt.eventDeliveries {
				err = e.CreateEventDelivery(ctx, &delivery)
				require.NoError(t, err)
			}

			eventDeliveries, paginationData, err := e.LoadEventDeliveriesPaged(ctx, tt.args.groupID, tt.args.appID, tt.args.eventID, tt.args.status, tt.args.searchParams, tt.args.pageable)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.Equal(t, tt.wantCount, len(eventDeliveries))
			require.NoError(t, err)
			require.Equal(t, tt.wantPaginationData, paginationData)
		})
	}
}
