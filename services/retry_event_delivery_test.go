package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func provideRetryEventDeliveryService(ctrl *gomock.Controller, eventDelivery *datastore.EventDelivery, project *datastore.Project) *RetryEventDeliveryService {
	return &RetryEventDeliveryService{
		EventDeliveryRepo: mocks.NewMockEventDeliveryRepository(ctrl),
		EndpointRepo:      mocks.NewMockEndpointRepository(ctrl),
		Queue:             mocks.NewMockQueuer(ctrl),
		EventDelivery:     eventDelivery,
		Project:           project,
	}
}

func TestRetryEventDeliveryService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Project
	}
	tests := []struct {
		name       string
		dbFn       func(es *RetryEventDeliveryService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_retry_event_delivery",
			dbFn: func(es *RetryEventDeliveryService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{Status: datastore.ActiveEndpointStatus}, nil)

				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
		},
		{
			name: "should_error_for_success_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "event already sent",
		},
		{
			name: "should_retry_event_delivery",
			dbFn: func(es *RetryEventDeliveryService) {
				er, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				er.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{Status: datastore.ActiveEndpointStatus}, nil)

				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
		},
		{
			name: "should_error_for_success_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "event already sent",
		},
		{
			name: "should_error_for_retry_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.RetryEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "cannot resend event that did not fail previously",
		},
		{
			name: "should_error_for_processing_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.ProcessingEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "cannot resend event that did not fail previously",
		},
		{
			name: "should_error_for_scheduled_status",
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.ScheduledEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "cannot resend event that did not fail previously",
		},
		{
			name: "should_fail_to_find_subscription",
			dbFn: func(es *RetryEventDeliveryService) {
				s, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(nil, datastore.ErrEndpointNotFound)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "endpoint not found",
		},
		{
			name: "should_error_for_pending_subscription_status",
			dbFn: func(es *RetryEventDeliveryService) {
				s, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.PendingEndpointStatus,
				}, nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "endpoint is being re-activated",
		},
		{
			name: "should_retry_event_delivery_with_inactive_subscription",
			dbFn: func(es *RetryEventDeliveryService) {
				s, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.InactiveEndpointStatus,
				}, nil)

				s.EXPECT().UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.PendingEndpointStatus).
					Times(1).Return(nil)

				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
		},
		{
			name: "should_fail_to_retry_event_delivery_with_inactive_subscription",
			dbFn: func(es *RetryEventDeliveryService) {
				s, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.InactiveEndpointStatus,
				}, nil)

				s.EXPECT().UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.PendingEndpointStatus).
					Times(1).Return(errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "failed to update endpoint status",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideRetryEventDeliveryService(ctrl, tc.args.eventDelivery, tc.args.g)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
