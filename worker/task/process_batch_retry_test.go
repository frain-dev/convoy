package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestProcessBatchRetry(t *testing.T) {
	tt := []struct {
		name          string
		expectedError error
		batchRetry    *datastore.BatchRetry
		dbFn          func(*mocks.MockBatchRetryRepository, *mocks.MockEventDeliveryRepository, *mocks.MockQueuer)
	}{
		{
			name:          "should_process_batch_retry_successfully",
			expectedError: nil,
			batchRetry: &datastore.BatchRetry{
				ID:              "batch-retry-1",
				ProjectID:       "project-1",
				Status:          datastore.BatchRetryStatusPending,
				TotalEvents:     10,
				ProcessedEvents: 0,
				FailedEvents:    0,
				Filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "project-1"},
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				},
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer) {
				// Check for active batch retry
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				// Update status to processing
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, datastore.BatchRetryStatusProcessing, retry.Status)
						return nil
					}).Times(1)

				// Load event deliveries
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(),
						gomock.Any(),
						[]string{"endpoint-1"},
						"event-1",
						"",
						nil,
						datastore.SearchParams{},
						datastore.Pageable{PerPage: 1000, Direction: datastore.Next, NextCursor: datastore.DefaultCursor},
						"",
						"",
					).
					Return([]datastore.EventDelivery{
						{UID: "delivery-1", Status: datastore.SuccessEventStatus},
						{UID: "delivery-2", Status: datastore.SuccessEventStatus},
					}, datastore.PaginationData{HasNextPage: false}, nil).Times(1)

				// Queue event deliveries
				q.EXPECT().
					Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					DoAndReturn(func(_ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
						var payload EventDelivery
						err := json.Unmarshal(job.Payload, &payload)
						require.NoError(t, err)
						assert.Contains(t, []string{"delivery-1", "delivery-2"}, payload.EventDeliveryID)
						return nil
					}).Times(2)

				// Update progress
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, 2, retry.ProcessedEvents)
						assert.Equal(t, 0, retry.FailedEvents)
						return nil
					}).Times(1)

				// Mark as completed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, datastore.BatchRetryStatusCompleted, retry.Status)
						assert.True(t, retry.CompletedAt.Valid)
						return nil
					}).Times(1)
			},
		},
		{
			name:          "should_fail_when_active_batch_retry_exists",
			expectedError: errors.New("an active batch retry already exists"),
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer) {
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(&datastore.BatchRetry{
						ID:        "active-retry",
						ProjectID: "project-1",
						Status:    datastore.BatchRetryStatusProcessing,
					}, nil).Times(1)
			},
		},
		{
			name:          "should_fail_when_updating_status_fails",
			expectedError: errors.New("batch retry not found"),
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer) {
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(datastore.ErrBatchRetryNotFound).Times(1)
			},
		},
		{
			name:          "should_fail_when_loading_deliveries_fails",
			expectedError: datastore.ErrEventDeliveryNotFound,
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
				Filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "project-1"},
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				},
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer) {
				// Check for active batch retry
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				// Update status to processing
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, datastore.BatchRetryStatusProcessing, retry.Status)
						return nil
					}).Times(1)

				// Load event deliveries - this will fail
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(),
						gomock.Any(),
						[]string{"endpoint-1"},
						"event-1",
						"",
						nil,
						datastore.SearchParams{},
						datastore.Pageable{PerPage: 1000, Direction: datastore.Next, NextCursor: datastore.DefaultCursor},
						"",
						"",
					).
					Return(nil, datastore.PaginationData{}, datastore.ErrEventDeliveryNotFound).Times(1)
			},
		},
		{
			name:          "should_handle_queue_failures",
			expectedError: nil,
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
				Filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "project-1"},
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				},
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer) {
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, datastore.BatchRetryStatusProcessing, retry.Status)
						return nil
					}).Times(1)

				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(),
						gomock.Any(),
						[]string{"endpoint-1"},
						"event-1",
						"",
						nil,
						datastore.SearchParams{},
						datastore.Pageable{PerPage: 1000, Direction: datastore.Next, NextCursor: datastore.DefaultCursor},
						"",
						"",
					).
					Return([]datastore.EventDelivery{
						{UID: "delivery-1", Status: datastore.SuccessEventStatus},
						{UID: "delivery-2", Status: datastore.SuccessEventStatus},
					}, datastore.PaginationData{HasNextPage: false}, nil).Times(1)

				q.EXPECT().
					Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Return(datastore.ErrEventDeliveryNotFound).Times(2)

				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, 0, retry.ProcessedEvents)
						assert.Equal(t, 2, retry.FailedEvents)
						return nil
					}).Times(1)

				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, datastore.BatchRetryStatusCompleted, retry.Status)
						assert.True(t, retry.CompletedAt.Valid)
						return nil
					}).Times(1)
			},
		},
		{
			name:          "should_handle_pagination",
			expectedError: nil,
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
				Filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "project-1"},
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				},
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer) {
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, datastore.BatchRetryStatusProcessing, retry.Status)
						return nil
					}).Times(1)

				// First page
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(),
						gomock.Any(),
						[]string{"endpoint-1"},
						"event-1",
						"",
						nil,
						datastore.SearchParams{},
						datastore.Pageable{PerPage: 1000, Direction: datastore.Next, NextCursor: datastore.DefaultCursor},
						"",
						"",
					).
					Return([]datastore.EventDelivery{
						{UID: "delivery-1", Status: datastore.SuccessEventStatus},
					}, datastore.PaginationData{HasNextPage: true, NextPageCursor: "next-cursor"}, nil).Times(1)

				q.EXPECT().
					Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Return(nil).Times(1)

				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, 1, retry.ProcessedEvents)
						assert.Equal(t, 0, retry.FailedEvents)
						return nil
					}).Times(1)

				// Second page
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(),
						gomock.Any(),
						[]string{"endpoint-1"},
						"event-1",
						"",
						nil,
						datastore.SearchParams{},
						datastore.Pageable{PerPage: 1000, Direction: datastore.Next, NextCursor: "next-cursor"},
						"",
						"",
					).
					Return([]datastore.EventDelivery{
						{UID: "delivery-2", Status: datastore.SuccessEventStatus},
					}, datastore.PaginationData{HasNextPage: false}, nil).Times(1)

				q.EXPECT().
					Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Return(nil).Times(1)

				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, 2, retry.ProcessedEvents)
						assert.Equal(t, 0, retry.FailedEvents)
						return nil
					}).Times(1)

				// Mark as completed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, retry *datastore.BatchRetry) error {
						assert.Equal(t, datastore.BatchRetryStatusCompleted, retry.Status)
						assert.True(t, retry.CompletedAt.Valid)
						return nil
					}).Times(1)
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			batchRetryRepo := mocks.NewMockBatchRetryRepository(ctrl)
			eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			queuer := mocks.NewMockQueuer(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(batchRetryRepo, eventDeliveryRepo, queuer)
			}

			processFn := ProcessBatchRetry(batchRetryRepo, eventDeliveryRepo, queuer)

			data, err := msgpack.EncodeMsgPack(tc.batchRetry)
			require.NoError(t, err)

			task := asynq.NewTask(string(convoy.BatchRetryProcessor), data, asynq.Queue(string(convoy.BatchRetryQueue)))

			err = processFn(context.Background(), task)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
