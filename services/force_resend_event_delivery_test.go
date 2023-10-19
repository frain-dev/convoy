package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func provideForceResendEventDeliveriesService(ctrl *gomock.Controller, ids []string, project *datastore.Project) *ForceResendEventDeliveriesService {
	return &ForceResendEventDeliveriesService{
		EventDeliveryRepo: mocks.NewMockEventDeliveryRepository(ctrl),
		EndpointRepo:      mocks.NewMockEndpointRepository(ctrl),
		Queue:             mocks.NewMockQueuer(ctrl),
		IDs:               ids,
		Project:           project,
	}
}

func TestForceResendEventDeliveriesService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		ids []string
		g   *datastore.Project
	}

	tests := []struct {
		name          string
		args          args
		dbFn          func(es *ForceResendEventDeliveriesService)
		wantSuccesses int
		wantFailures  int
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name: "should_force_resend_event_deliveries",
			args: args{
				ctx: ctx,
				ids: []string{"oop", "ref"},
				g:   &datastore.Project{UID: "123"},
			},
			dbFn: func(es *ForceResendEventDeliveriesService) {
				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().FindEventDeliveriesByIDs(
					gomock.Any(), gomock.Any(), []string{"oop", "ref"}).
					Times(1).
					Return(
						[]datastore.EventDelivery{
							{
								UID: "ref",

								Status: datastore.SuccessEventStatus,
							},
							{
								UID:    "oop",
								Status: datastore.SuccessEventStatus,
							},
						},
						nil,
					)

				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "123").
					Times(2).Return(&datastore.Endpoint{
					Status: datastore.ActiveEndpointStatus,
				}, nil)

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)
			},
			wantSuccesses: 2,
			wantFailures:  0,
		},
		{
			name: "should_fail_validation_for_resend_event_deliveries_with_one_failure",
			args: args{
				ctx: ctx,
				ids: []string{"ref", "oop"},
				g:   &datastore.Project{UID: "123"},
			},
			dbFn: func(es *ForceResendEventDeliveriesService) {
				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().FindEventDeliveriesByIDs(
					gomock.Any(), gomock.Any(), []string{"ref", "oop"}).
					Times(1).
					Return(
						[]datastore.EventDelivery{
							{
								UID:    "ref",
								Status: datastore.SuccessEventStatus,
							},
							{
								UID:    "oop",
								Status: datastore.FailureEventStatus,
							},
						},
						nil,
					)
			},
			wantErr:    true,
			wantErrMsg: ErrInvalidEventDeliveryStatus.Error(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideForceResendEventDeliveriesService(ctrl, tc.args.ids, tc.args.g)

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

func TestEventService_forceResendEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Project
	}
	tests := []struct {
		name       string
		dbFn       func(es *ForceResendEventDeliveriesService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_force_resend_event_delivery",
			dbFn: func(es *ForceResendEventDeliveriesService) {
				s, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "test_project").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.ActiveEndpointStatus,
				}, nil)

				ed, _ := es.EventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "test_project"},
			},
		},
		{
			name: "should_fail_to_find_endpoint",
			dbFn: func(es *ForceResendEventDeliveriesService) {
				s, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "test_project").
					Times(1).Return(nil, errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "test_project"},
			},
			wantErr:    true,
			wantErrMsg: "endpoint not found",
		},
		{
			name: "should_error_not_active_subscription",
			dbFn: func(es *ForceResendEventDeliveriesService) {
				s, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				s.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "test_project").
					Times(1).Return(&datastore.Endpoint{
					Status: datastore.InactiveEndpointStatus,
				}, nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Project{UID: "test_project"},
			},
			wantErr:    true,
			wantErrMsg: "force resend to an inactive or pending endpoint is not allowed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideForceResendEventDeliveriesService(ctrl, nil, nil)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.forceResendEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_requeueEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx               context.Context
		eventDelivery     *datastore.EventDelivery
		g                 *datastore.Project
		eventDeliveryRepo datastore.EventDeliveryRepository
		queuer            queue.Queuer
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(es *args)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_requeue_event_delivery",
			args: args{
				ctx:           ctx,
				eventDelivery: &datastore.EventDelivery{UID: "123"},
				g:             &datastore.Project{Name: "test_project"},
			},
			dbFn: func(es *args) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				eq, _ := es.queuer.(*mocks.MockQueuer)
				eq.EXPECT().Write(gomock.Any(), convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_fail_update_event_delivery_status",
			args: args{
				ctx:           ctx,
				eventDelivery: &datastore.EventDelivery{UID: "123"},
				g:             &datastore.Project{Name: "test_project"},
			},
			dbFn: func(es *args) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while trying to resend event",
		},
		{
			name: "should_fail_to_write_event_delivery_to_queue",
			args: args{
				ctx:           ctx,
				eventDelivery: &datastore.EventDelivery{UID: "123"},
				g:             &datastore.Project{Name: "test_project"},
			},
			dbFn: func(es *args) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				eq, _ := es.queuer.(*mocks.MockQueuer)
				eq.EXPECT().Write(gomock.Any(), convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "error occurred re-enqueing old event - 123",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			tc.args.eventDeliveryRepo = mocks.NewMockEventDeliveryRepository(ctrl)
			tc.args.queuer = mocks.NewMockQueuer(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(&tc.args)
			}

			err = requeueEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g, tc.args.eventDeliveryRepo, tc.args.queuer)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
