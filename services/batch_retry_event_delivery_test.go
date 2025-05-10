package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
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

				ed.EXPECT().CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any())
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

				ed.EXPECT().CountEventDeliveries(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				br.EXPECT().CreateBatchRetry(gomock.Any(), gomock.Any())

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantSuccesses: 1,
			wantFailures:  1,
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
