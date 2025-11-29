package task

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
)

func TestProcessBatchRetry(t *testing.T) {
	tt := []struct {
		name          string
		expectedError error
		batchRetry    *datastore.BatchRetry
		dbFn          func(*mocks.MockBatchRetryRepository, *mocks.MockEventDeliveryRepository, *mocks.MockQueuer, *datastore.BatchRetry)
	}{
		{
			name:          "should_process_batch_retry_successfully",
			expectedError: nil, // Should succeed
			batchRetry: &datastore.BatchRetry{
				ID:              "batch-retry-1",
				ProjectID:       "project-1",
				Status:          datastore.BatchRetryStatusPending,
				TotalEvents:     10,
				ProcessedEvents: 0,
				FailedEvents:    0,
				Filter: datastore.FromFilterStruct(datastore.Filter{
					ProjectID:   "project-1",
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Status:      []datastore.EventDeliveryStatus{},
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				}),
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, retry *datastore.BatchRetry) {
				// Check for active batch retry - none found, will use the provided one
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				// Update status to processing - should succeed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				// Find active batch retry again in the loop
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(retry, nil).Times(1)

				// Load event deliveries - return empty list to exit loop
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(), "project-1", []string{"endpoint-1"}, "event-1", "", []datastore.EventDeliveryStatus{}, gomock.Any(), gomock.Any(), "", "", "",
					).
					Return([]datastore.EventDelivery{}, datastore.PaginationData{HasNextPage: false}, nil).Times(1)

				// Mark batch retry as completed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
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
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, retry *datastore.BatchRetry) {
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
				Filter: datastore.FromFilterStruct(datastore.Filter{
					ProjectID:   "project-1",
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Status:      []datastore.EventDeliveryStatus{},
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				}),
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, retry *datastore.BatchRetry) {
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				// Update status to processing - this will fail
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(datastore.ErrBatchRetryNotFound).Times(1)
			},
		},
		{
			name:          "should_fail_when_filter_is_invalid",
			expectedError: fmt.Errorf("batch retry has no filter"),
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
				// No Filter field - this should cause the function to fail early
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, retry *datastore.BatchRetry) {
				// Check for active batch retry - none found, will use the provided one
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)
			},
		},
		{
			name:          "should_fail_when_loading_deliveries_fails",
			expectedError: errors.New("failed to load deliveries"),
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
				Filter: datastore.FromFilterStruct(datastore.Filter{
					ProjectID:   "project-1",
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Status:      []datastore.EventDeliveryStatus{},
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				}),
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, retry *datastore.BatchRetry) {
				// Check for active batch retry - none found, will use the provided one
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				// Update status to processing - should succeed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				// Find active batch retry again in the loop
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(retry, nil).Times(1)

				// Load event deliveries - this will fail
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(), "project-1", []string{"endpoint-1"}, "event-1", "", []datastore.EventDeliveryStatus{}, gomock.Any(), gomock.Any(), "", "", "",
					).
					Return(nil, datastore.PaginationData{}, errors.New("failed to load deliveries")).Times(1)

				// Update batch retry to failed status
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
		},
		{
			name:          "should_handle_queue_failures",
			expectedError: nil, // Should succeed even with some queue failures
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
				Filter: datastore.FromFilterStruct(datastore.Filter{
					ProjectID:   "project-1",
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Status:      []datastore.EventDeliveryStatus{},
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				}),
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, retry *datastore.BatchRetry) {
				// Check for active batch retry - none found, will use the provided one
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				// Update status to processing - should succeed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				// Find active batch retry again in the loop
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(retry, nil).Times(1)

				// Load event deliveries - return some deliveries
				deliveries := []datastore.EventDelivery{
					{UID: "delivery-1"},
					{UID: "delivery-2"},
				}
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(), "project-1", []string{"endpoint-1"}, "event-1", "", []datastore.EventDeliveryStatus{}, gomock.Any(), gomock.Any(), "", "", "",
					).
					Return(deliveries, datastore.PaginationData{HasNextPage: false}, nil).Times(1)

				// Queue the first delivery successfully
				q.EXPECT().
					Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Return(nil).Times(1)

				// Queue the second delivery - this will fail
				q.EXPECT().
					Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Return(errors.New("queue failed")).Times(1)

				// Update batch retry progress
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				// Mark batch retry as completed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
		},
		{
			name:          "should_handle_pagination",
			expectedError: nil, // Should succeed with pagination
			batchRetry: &datastore.BatchRetry{
				ID:        "batch-retry-1",
				ProjectID: "project-1",
				Status:    datastore.BatchRetryStatusPending,
				Filter: datastore.FromFilterStruct(datastore.Filter{
					ProjectID:   "project-1",
					EndpointIDs: []string{"endpoint-1"},
					EventID:     "event-1",
					Status:      []datastore.EventDeliveryStatus{},
					Pageable: datastore.Pageable{
						PerPage:    1000,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
				}),
			},
			dbFn: func(br *mocks.MockBatchRetryRepository, ed *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, retry *datastore.BatchRetry) {
				// Check for active batch retry - none found, will use the provided one
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				// Update status to processing - should succeed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				// First iteration - find active batch retry
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(retry, nil).Times(1)

				// First batch of deliveries
				deliveries1 := []datastore.EventDelivery{
					{UID: "delivery-1"},
					{UID: "delivery-2"},
				}
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(), "project-1", []string{"endpoint-1"}, "event-1", "", []datastore.EventDeliveryStatus{}, gomock.Any(), gomock.Any(), "", "", "",
					).
					Return(deliveries1, datastore.PaginationData{HasNextPage: true, NextPageCursor: "cursor-2"}, nil).Times(1)

				// Queue first batch successfully
				q.EXPECT().
					Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Return(nil).Times(2)

				// Update batch retry progress after first batch
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				// Second iteration - find active batch retry again
				br.EXPECT().
					FindActiveBatchRetry(gomock.Any(), "project-1").
					Return(retry, nil).Times(1)

				// Second batch of deliveries (empty, so exit loop)
				ed.EXPECT().
					LoadEventDeliveriesPaged(
						gomock.Any(), "project-1", []string{"endpoint-1"}, "event-1", "", []datastore.EventDeliveryStatus{}, gomock.Any(), gomock.Any(), "", "", "",
					).
					Return([]datastore.EventDelivery{}, datastore.PaginationData{HasNextPage: false}, nil).Times(1)

				// Mark batch retry as completed
				br.EXPECT().
					UpdateBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
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
			logger := log.NewLogger(os.Stdout)

			if tc.dbFn != nil {
				tc.dbFn(batchRetryRepo, eventDeliveryRepo, queuer, tc.batchRetry)
			}

			processFn := ProcessBatchRetry(batchRetryRepo, eventDeliveryRepo, queuer, logger)

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
