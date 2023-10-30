package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func provideBatchRetryEventDeliveryService(ctrl *gomock.Controller, f *datastore.Filter) *BatchRetryEventDeliveryService {
	return &BatchRetryEventDeliveryService{
		EventDeliveryRepo: mocks.NewMockEventDeliveryRepository(ctrl),
		EndpointRepo:      mocks.NewMockEndpointRepository(ctrl),
		Queue:             mocks.NewMockQueuer(ctrl),
		EventRepo:         mocks.NewMockEventRepository(ctrl),
		Filter:            f,
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
				ss, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)

				ss.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "123").
					Return(&datastore.Endpoint{
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(2)

				ed.EXPECT().LoadEventDeliveriesPaged(
					gomock.Any(),
					"123",
					[]string{"abc"},
					"13429", "",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
					datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
					gomock.Any(), gomock.Any()).
					Times(1).
					Return(
						[]datastore.EventDelivery{
							{
								UID:            "ref",
								SubscriptionID: "sub-1",
							},
							{
								UID:            "oop",
								SubscriptionID: "sub-2",
								Status:         datastore.FailureEventStatus,
							},
						},
						datastore.PaginationData{},
						nil,
					)

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)
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
				ss, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)

				ss.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "123").
					Return(&datastore.Endpoint{
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(1)

				ed.EXPECT().LoadEventDeliveriesPaged(
					gomock.Any(),
					"123",
					[]string{"abc"},
					"13429", "sub-1",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
					datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
					gomock.Any(), gomock.Any()).
					Times(1).
					Return(
						[]datastore.EventDelivery{
							{
								UID:            "ref",
								SubscriptionID: "sub-1",
								Status:         datastore.SuccessEventStatus,
							},
							{
								UID:            "oop",
								SubscriptionID: "sub-2",
								Status:         datastore.FailureEventStatus,
							},
						},
						datastore.PaginationData{},
						nil,
					)

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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

			successes, failures, err := es.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantSuccesses, successes)
			require.Equal(t, tc.wantFailures, failures)
		})
	}
}
