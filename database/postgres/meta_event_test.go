//go:build integration
// +build integration

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func Test_CreateMetaEvent(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	metaEventRepo := NewMetaEventRepo(db, nil)
	metaEvent := generateMetaEvent(t, db)
	ctx := context.Background()

	require.NoError(t, metaEventRepo.CreateMetaEvent(ctx, metaEvent))

	newMetaEvent, err := metaEventRepo.FindMetaEventByID(ctx, metaEvent.ProjectID, metaEvent.UID)
	require.NoError(t, err)

	newMetaEvent.CreatedAt, newMetaEvent.UpdatedAt, newMetaEvent.Metadata.NextSendTime = time.Time{}, time.Time{}, time.Time{}
	metaEvent.CreatedAt, metaEvent.UpdatedAt, metaEvent.Metadata.NextSendTime = time.Time{}, time.Time{}, time.Time{}

	require.Equal(t, metaEvent, newMetaEvent)
}

func Test_FindMetaEventByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	metaEventRepo := NewMetaEventRepo(db, nil)
	metaEvent := generateMetaEvent(t, db)
	ctx := context.Background()

	_, err := metaEventRepo.FindMetaEventByID(ctx, metaEvent.ProjectID, metaEvent.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrMetaEventNotFound))

	require.NoError(t, metaEventRepo.CreateMetaEvent(ctx, metaEvent))

	newMetaEvent, err := metaEventRepo.FindMetaEventByID(ctx, metaEvent.ProjectID, metaEvent.UID)
	require.NoError(t, err)

	newMetaEvent.CreatedAt, newMetaEvent.UpdatedAt, newMetaEvent.Metadata.NextSendTime = time.Time{}, time.Time{}, time.Time{}
	metaEvent.CreatedAt, metaEvent.UpdatedAt, metaEvent.Metadata.NextSendTime = time.Time{}, time.Time{}, time.Time{}

	require.Equal(t, metaEvent, newMetaEvent)
}

func Test_UpdateMetaEvent(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	metaEventRepo := NewMetaEventRepo(db, nil)
	metaEvent := generateMetaEvent(t, db)
	ctx := context.Background()

	require.NoError(t, metaEventRepo.CreateMetaEvent(ctx, metaEvent))

	data := json.RawMessage([]byte(`{"event_type": "endpoint.updated"}`))

	metaEvent.Status = datastore.SuccessEventStatus
	metaEvent.EventType = string(datastore.EndpointUpdated)
	metaEvent.Metadata = &datastore.Metadata{
		Data: data,
		Raw:  string(data),
	}
	err := metaEventRepo.UpdateMetaEvent(ctx, metaEvent.ProjectID, metaEvent)
	require.NoError(t, err)

	newMetaEvent, err := metaEventRepo.FindMetaEventByID(ctx, metaEvent.ProjectID, metaEvent.UID)
	require.NoError(t, err)

	newMetaEvent.CreatedAt, newMetaEvent.UpdatedAt, newMetaEvent.Metadata.NextSendTime = time.Time{}, time.Time{}, time.Time{}
	metaEvent.CreatedAt, metaEvent.UpdatedAt, metaEvent.Metadata.NextSendTime = time.Time{}, time.Time{}, time.Time{}

	require.Equal(t, metaEvent, newMetaEvent)
}

func Test_LoadMetaEventsPaged(t *testing.T) {
	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name       string
		pageData   datastore.Pageable
		count      int
		endpointID string
		expected   Expected
	}{
		{
			name: "Load Meta Events Paged - 10 records",
			pageData: datastore.Pageable{
				PerPage:    3,
				Direction:  datastore.Next,
				NextCursor: datastore.DefaultCursor,
			},
			count: 10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},

		{
			name: "Load Meta Events Paged - 12 records",
			pageData: datastore.Pageable{
				PerPage:    4,
				Direction:  datastore.Next,
				NextCursor: datastore.DefaultCursor,
			},
			count: 12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 4,
				},
			},
		},

		{
			name: "Load Meta Events Paged - 5 records",
			pageData: datastore.Pageable{
				PerPage:    3,
				Direction:  datastore.Next,
				NextCursor: datastore.DefaultCursor,
			},
			count: 5,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			project := seedProject(t, db)
			metaEventRepo := NewMetaEventRepo(db, nil)

			for i := 0; i < tc.count; i++ {
				metaEvent := &datastore.MetaEvent{
					UID:       ulid.Make().String(),
					Status:    datastore.ScheduledEventStatus,
					EventType: string(datastore.EndpointCreated),
					ProjectID: project.UID,
					Metadata:  &datastore.Metadata{},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}

				err := metaEventRepo.CreateMetaEvent(context.Background(), metaEvent)
				require.NoError(t, err)
			}

			_, pageable, err := metaEventRepo.LoadMetaEventsPaged(context.Background(), project.UID, &datastore.Filter{
				SearchParams: datastore.SearchParams{
					CreatedAtStart: time.Now().Add(-time.Hour).Unix(),
					CreatedAtEnd:   time.Now().Add(5 * time.Minute).Unix(),
				},
				Pageable: tc.pageData,
			})

			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
		})
	}
}

func generateMetaEvent(t *testing.T, db database.Database) *datastore.MetaEvent {
	project := seedProject(t, db)

	return &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		Status:    datastore.ScheduledEventStatus,
		EventType: string(datastore.EndpointCreated),
		ProjectID: project.UID,
		Metadata: &datastore.Metadata{
			Data:            []byte(`{"name": "10x"}`),
			Raw:             `{"name": "10x"}`,
			Strategy:        datastore.ExponentialStrategyProvider,
			NextSendTime:    time.Now().Add(time.Hour),
			NumTrials:       1,
			IntervalSeconds: 10,
			RetryLimit:      20,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
