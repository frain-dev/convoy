package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideBatchRetryEventDeliveryService(ctrl *gomock.Controller, f *datastore.Filter) *BatchRetryEventDeliveryService {
	return &BatchRetryEventDeliveryService{
		BatchRetryRepo:    mocks.NewMockBatchRetryRepository(ctrl),
		EventDeliveryRepo: mocks.NewMockEventDeliveryRepository(ctrl),
		Queue:             mocks.NewMockQueuer(ctrl),
		Filter:            f,
		ProjectID:         "123",
	}
}

func TestBatchRetryEventDeliveryService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name          string
		args          args
		dbFn          func(es *BatchRetryEventDeliveryService)
		wantSuccesses int
		wantFailures  int
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name: "should_batch_retry_event_deliveries",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
					Pageable: datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
					Status: []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
				},
			},
			wantSuccesses: 2,
			wantFailures:  0,
			dbFn: func(es *BatchRetryEventDeliveryService) {
				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				br, _ := es.BatchRetryRepo.(*mocks.MockBatchRetryRepository)

				br.EXPECT().FindActiveBatchRetry(gomock.Any(), gomock.Any()).
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				br.EXPECT().CreateBatchRetry(gomock.Any(), gomock.Any())

				ed.EXPECT().CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(10), nil)

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().WriteWithoutTimeout(gomock.Any(), gomock.Any(), gomock.Any())
			},
		},
		{
			name: "should_batch_retry_event_deliveries_with_one_failure",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:        &datastore.Project{UID: "123"},
					EndpointIDs:    []string{"abc"},
					SubscriptionID: "sub-1",
					EventID:        "13429",
					Pageable: datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
					Status: []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
				},
			},
			dbFn: func(es *BatchRetryEventDeliveryService) {
				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				br, _ := es.BatchRetryRepo.(*mocks.MockBatchRetryRepository)

				br.EXPECT().FindActiveBatchRetry(gomock.Any(), "123").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				ed.EXPECT().CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(10), nil)

				br.EXPECT().CreateBatchRetry(gomock.Any(), gomock.Any())

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().WriteWithoutTimeout(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantSuccesses: 1,
			wantFailures:  1,
		},
		{
			name: "should_fail_when_active_batch_retry_exists",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
				},
			},
			wantErr:    true,
			wantErrMsg: "an active batch retry already exists",
			dbFn: func(es *BatchRetryEventDeliveryService) {
				br, _ := es.BatchRetryRepo.(*mocks.MockBatchRetryRepository)
				br.EXPECT().FindActiveBatchRetry(gomock.Any(), "123").
					Return(&datastore.BatchRetry{
						ID:        "active-retry",
						ProjectID: "123",
						Status:    datastore.BatchRetryStatusProcessing,
					}, nil).Times(1)
			},
		},
		{
			name: "should_fail_when_counting_events_fails",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
				},
			},
			wantErr:    true,
			wantErrMsg: "failed to count events",
			dbFn: func(es *BatchRetryEventDeliveryService) {
				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				br, _ := es.BatchRetryRepo.(*mocks.MockBatchRetryRepository)

				br.EXPECT().FindActiveBatchRetry(gomock.Any(), "123").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				ed.EXPECT().CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(0), errors.New("failed to count events"))
			},
		},
		{
			name: "should_fail_when_creating_batch_retry_fails",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
				},
			},
			wantErr:    true,
			wantErrMsg: "failed to create batch retry",
			dbFn: func(es *BatchRetryEventDeliveryService) {
				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				br, _ := es.BatchRetryRepo.(*mocks.MockBatchRetryRepository)

				br.EXPECT().FindActiveBatchRetry(gomock.Any(), "123").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				ed.EXPECT().CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(10), nil)

				br.EXPECT().CreateBatchRetry(gomock.Any(), gomock.Any()).
					Return(errors.New("failed to create batch retry"))
			},
		},
		{
			name: "should_fail_when_queueing_batch_retry_fails",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
				},
			},
			wantErr:    true,
			wantErrMsg: "failed to queue batch retry job",
			dbFn: func(es *BatchRetryEventDeliveryService) {
				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				br, _ := es.BatchRetryRepo.(*mocks.MockBatchRetryRepository)
				q, _ := es.Queue.(*mocks.MockQueuer)

				br.EXPECT().FindActiveBatchRetry(gomock.Any(), "123").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				ed.EXPECT().CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(10), nil)

				br.EXPECT().CreateBatchRetry(gomock.Any(), gomock.Any())

				q.EXPECT().WriteWithoutTimeout(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("failed to queue batch retry job"))
			},
		},
		{
			name: "should_fail_when_finding_active_batch_retry_fails",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
				},
			},
			wantErr:    true,
			wantErrMsg: "failed to check for active batch retry",
			dbFn: func(es *BatchRetryEventDeliveryService) {
				br, _ := es.BatchRetryRepo.(*mocks.MockBatchRetryRepository)
				br.EXPECT().FindActiveBatchRetry(gomock.Any(), "123").
					Return(nil, errors.New("failed to check for active batch retry"))
			},
		},
		{
			name: "should_succeed_with_zero_events",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:     &datastore.Project{UID: "123"},
					EndpointIDs: []string{"abc"},
					EventID:     "13429",
				},
			},
			dbFn: func(es *BatchRetryEventDeliveryService) {
				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				br, _ := es.BatchRetryRepo.(*mocks.MockBatchRetryRepository)
				q, _ := es.Queue.(*mocks.MockQueuer)

				br.EXPECT().FindActiveBatchRetry(gomock.Any(), "123").
					Return(nil, datastore.ErrBatchRetryNotFound).Times(1)

				ed.EXPECT().CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(0), nil)

				br.EXPECT().CreateBatchRetry(gomock.Any(), gomock.Any())

				q.EXPECT().WriteWithoutTimeout(gomock.Any(), gomock.Any(), gomock.Any())
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideBatchRetryEventDeliveryService(ctrl, tc.args.filter)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.Run(tc.args.ctx)
			if tc.wantErr {
				require.Error(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.NoError(t, err)
		})
	}
}
