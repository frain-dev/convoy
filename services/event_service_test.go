package services

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideEventService(ctrl *gomock.Controller) *EventService {
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	queue := mocks.NewMockQueuer(ctrl)
	cache := mocks.NewMockCache(ctrl)
	searcher := mocks.NewMockSearcher(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	deviceRepo := mocks.NewMockDeviceRepository(ctrl)
	return NewEventService(endpointRepo, eventRepo, eventDeliveryRepo, queue, cache, searcher, subRepo, sourceRepo, deviceRepo)
}

func TestEventService_CreateEvent(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx        context.Context
		newMessage *models.Event
		g          *datastore.Group
	}
	tests := []struct {
		name        string
		dbFn        func(es *EventService)
		args        args
		wantEvent   *datastore.Event
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_event",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					GroupID:      "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)
				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoint:  "123",
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Group{
					UID:  "abc",
					Name: "test_group",
					Config: &datastore.GroupConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:       &datastore.SignatureConfiguration{},
						DisableEndpoint: false,
						ReplayAttacks:   false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123"},
				GroupID:          "abc",
				DocumentStatus:   datastore.ActiveDocumentStatus,
			},
		},

		{
			name: "should_create_event_for_multiple_endpoints",
			dbFn: func(es *EventService) {

				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByID(gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{
					{
						Title:        "test_app",
						UID:          "123",
						GroupID:      "abc",
						SupportEmail: "test_app@gmail.com",
					},

					{
						Title:        "test_app",
						UID:          "12345",
						GroupID:      "abc",
						SupportEmail: "test_app@gmail.com",
					},
				}, nil)
				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoints: []string{"123", "12345"},
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Group{
					UID:  "abc",
					Name: "test_group",
					Config: &datastore.GroupConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:       &datastore.SignatureConfiguration{},
						DisableEndpoint: false,
						ReplayAttacks:   false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123", "12345"},
				GroupID:          "abc",
				DocumentStatus:   datastore.ActiveDocumentStatus,
			},
		},

		{
			name: "should_create_event_with_exponential_backoff_strategy",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByID(gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{
					{
						Title:        "test_app",
						UID:          "123",
						GroupID:      "abc",
						SupportEmail: "test_app@gmail.com",
					},
				}, nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoints: []string{"123"},
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Group{
					UID:  "abc",
					Name: "test_group",
					Config: &datastore.GroupConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "exponential",
							Duration:   1000,
							RetryCount: 10,
						},
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123"},
				GroupID:          "abc",
				DocumentStatus:   datastore.ActiveDocumentStatus,
			},
		},
		{
			name: "should_create_event_for_disabled_endpoint",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByID(gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{
					{
						Title:        "test_app",
						UID:          "123",
						GroupID:      "abc",
						SupportEmail: "test_app@gmail.com",
					},
				}, nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoints: []string{"123"},
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Group{
					UID:  "abc",
					Name: "test_group",
					Config: &datastore.GroupConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:       &datastore.SignatureConfiguration{},
						DisableEndpoint: false,
						ReplayAttacks:   false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123"},
				GroupID:          "abc",
				DocumentStatus:   datastore.ActiveDocumentStatus,
			},
		},
		{
			name: "should_create_event_with_custom_headers",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByID(gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{
					{
						Title:        "test_app",
						UID:          "123",
						GroupID:      "abc",
						SupportEmail: "test_app@gmail.com",
					},
				}, nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoints:     []string{"123"},
					EventType:     "payment.created",
					Data:          bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
					CustomHeaders: map[string]string{"X-Test-Signature": "Test"},
				},
				g: &datastore.Group{
					UID:  "abc",
					Name: "test_group",
					Config: &datastore.GroupConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:       &datastore.SignatureConfiguration{},
						DisableEndpoint: false,
						ReplayAttacks:   false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType:        datastore.EventType("payment.created"),
				MatchedEndpoints: 0,
				Data:             bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints:        []string{"123"},
				GroupID:          "abc",
				DocumentStatus:   datastore.ActiveDocumentStatus,
				Headers:          httpheader.HTTPHeader{"X-Test-Signature": []string{"Test"}},
			},
		},
		{
			name: "should_error_for_invalid_strategy_config",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByID(gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{
					{
						Title:        "test_app",
						UID:          "123",
						GroupID:      "abc",
						SupportEmail: "test_app@gmail.com",
					},
				}, nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoints: []string{"123"},
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Group{
					UID:    "abc",
					Name:   "test_group",
					Config: &datastore.GroupConfig{},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "retry strategy not defined in configuration",
		},
		{
			name: "should_error_for_empty_endpoints",
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoints: []string{},
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Group{},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  ErrInvalidEndpointID.Error(),
		},
		{
			name: "should_error_for_endpoint_not_found",
			dbFn: func(es *EventService) {
				a, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByID(gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{}, nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoints: []string{"123"},
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Group{},
			},
			wantErr:     true,
			wantErrCode: http.StatusNotFound,
			wantErrMsg:  ErrNoValidEndpointFound.Error(),
		},

		{
			name: "should_fail_to_create_event",
			dbFn: func(es *EventService) {},
			args: args{
				ctx: ctx,
				newMessage: &models.Event{
					Endpoints: []string{"123"},
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while creating event - invalid group",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			event, err := es.CreateEvent(tc.args.ctx, tc.args.newMessage, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, event.UID)
			require.NotEmpty(t, event.CreatedAt)
			require.NotEmpty(t, event.UpdatedAt)
			require.Empty(t, event.DeletedAt)

			stripVariableFields(t, "event", event)

			m1 := tc.wantEvent.Endpoints[0]
			m2 := event.Endpoints[0]

			tc.wantEvent.Endpoints[0], event.Endpoints[0] = "", ""
			require.Equal(t, tc.wantEvent, event)
			require.Equal(t, m1, m2)
		})
	}
}

func TestEventService_GetEvent(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		id  string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(es *EventService)
		wantEvent   *datastore.Event
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_get_app_event",
			args: args{
				ctx: ctx,
				id:  "123",
			},
			dbFn: func(es *EventService) {
				e, _ := es.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventByID(gomock.Any(), "123").
					Times(1).Return(&datastore.Event{UID: "123"}, nil)
			},
			wantEvent: &datastore.Event{UID: "123"},
		},
		{
			name: "should_fail_to_get_app_event",
			args: args{
				ctx: ctx,
				id:  "123",
			},
			dbFn: func(es *EventService) {
				e, _ := es.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventByID(gomock.Any(), "123").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find event by id",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			event, err := es.GetEvent(tc.args.ctx, tc.args.id)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantEvent, event)
		})
	}
}

func TestEventService_ReplayAppEvent(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx   context.Context
		event *datastore.Event
		g     *datastore.Group
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(es *EventService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_replay_app_event",
			args: args{
				ctx:   ctx,
				event: &datastore.Event{UID: "123"},
				g:     &datastore.Group{UID: "123", Name: "test_group"},
			},
			dbFn: func(es *EventService) {
				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_replay_app_event",
			args: args{
				ctx:   ctx,
				event: &datastore.Event{UID: "123"},
				g:     &datastore.Group{UID: "123", Name: "test_group"},
			},
			dbFn: func(es *EventService) {
				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to write event to queue",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err := es.ReplayEvent(tc.args.ctx, tc.args.event, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_GetEventDelivery(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		id  string
	}
	tests := []struct {
		name              string
		args              args
		dbFn              func(es *EventService)
		wantEventDelivery *datastore.EventDelivery
		wantErr           bool
		wantErrCode       int
		wantErrMsg        string
	}{
		{
			name: "should_get_event_delivery",
			args: args{
				ctx: ctx,
				id:  "123",
			},
			dbFn: func(es *EventService) {
				e, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().FindEventDeliveryByID(gomock.Any(), "123").
					Times(1).Return(&datastore.EventDelivery{UID: "123"}, nil)
			},
			wantEventDelivery: &datastore.EventDelivery{UID: "123"},
		},
		{
			name: "should_fail_to_get_event_delivery",
			args: args{
				ctx: ctx,
				id:  "123",
			},
			dbFn: func(es *EventService) {
				e, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				e.EXPECT().FindEventDeliveryByID(gomock.Any(), "123").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find event delivery by id",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			eventDelivery, err := es.GetEventDelivery(tc.args.ctx, tc.args.id)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantEventDelivery, eventDelivery)
		})
	}
}

func TestEventService_BatchRetryEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name          string
		args          args
		dbFn          func(es *EventService)
		wantSuccesses int
		wantFailures  int
		wantErr       bool
		wantErrCode   int
		wantErrMsg    string
	}{
		{
			name: "should_batch_retry_event_deliveries",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "13429",
					Pageable: datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
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
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ss, _ := es.subRepo.(*mocks.MockSubscriptionRepository)

				ss.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil).Times(2)

				ed.EXPECT().LoadEventDeliveriesPaged(
					gomock.Any(),
					"123",
					"abc",
					"13429",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
					datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
					}).
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

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)
			},
		},
		{
			name: "should_batch_retry_event_deliveries_with_one_failure",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "13429",
					Pageable: datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
					},
					Status: []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ss, _ := es.subRepo.(*mocks.MockSubscriptionRepository)

				ss.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil).Times(1)

				ed.EXPECT().LoadEventDeliveriesPaged(
					gomock.Any(),
					"123",
					"abc",
					"13429",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.RetryEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 1342,
						CreatedAtEnd:   1332,
					},
					datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
					}).
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

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				q, _ := es.queue.(*mocks.MockQueuer)
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
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			successes, failures, err := es.BatchRetryEventDelivery(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantSuccesses, successes)
			require.Equal(t, tc.wantFailures, failures)
		})
	}
}

func TestEventService_CountAffectedEventDeliveries(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(es *EventService)
		wantCount   int64
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_count_affected_event_deliveries",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "ref",
					Status:     []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().CountEventDeliveries(
					gomock.Any(),
					"123",
					"abc",
					"ref",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					}).Times(1).Return(int64(1234), nil)
			},
			wantCount: 1234,
		},
		{
			name: "should_fail_to_count_affected_event_deliveries",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "ref",
					Status:     []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().CountEventDeliveries(
					gomock.Any(),
					"123",
					"abc",
					"ref",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					}).Times(1).Return(int64(0), errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching event deliveries",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			count, err := es.CountAffectedEventDeliveries(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantCount, count)
		})
	}
}

func TestEventService_ForceResendEventDeliveries(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx context.Context
		ids []string
		g   *datastore.Group
	}
	tests := []struct {
		name          string
		args          args
		dbFn          func(es *EventService)
		wantSuccesses int
		wantFailures  int
		wantErr       bool
		wantErrCode   int
		wantErrMsg    string
	}{
		{
			name: "should_force_resend_event_deliveries",
			args: args{
				ctx: ctx,
				ids: []string{"oop", "ref"},
				g:   &datastore.Group{UID: "123"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().FindEventDeliveriesByIDs(
					gomock.Any(), []string{"oop", "ref"}).
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

				a, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				a.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(&datastore.Subscription{
					Status: datastore.ActiveSubscriptionStatus,
				}, nil)

				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(2).Return(nil)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
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
				g:   &datastore.Group{UID: "123"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().FindEventDeliveriesByIDs(
					gomock.Any(), []string{"ref", "oop"}).
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
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  ErrInvalidEventDeliveryStatus.Error(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			successes, failures, err := es.ForceResendEventDeliveries(tc.args.ctx, tc.args.ids, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantSuccesses, successes)
			require.Equal(t, tc.wantFailures, failures)
		})
	}
}

func TestEventService_GetEventsPaged(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name               string
		args               args
		dbFn               func(es *EventService)
		wantEvents         []datastore.Event
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_get_event_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					SourceID:   "bcv",
					EndpointID: "abc",
					Pageable: datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
					},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventRepo.(*mocks.MockEventRepository)
				f := &datastore.Filter{
					Query:      "",
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "",
					SourceID:   "bcv",
					Pageable: datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
					},
					Status: nil,
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				}
				ed.EXPECT().LoadEventsPaged(gomock.Any(), f).
					Times(1).
					Return([]datastore.Event{{UID: "1234", Endpoints: []string{"abc"}}}, datastore.PaginationData{
						Total:     1,
						Page:      1,
						PerPage:   2,
						Prev:      1,
						Next:      3,
						TotalPage: 2,
					}, nil)

				ap, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				ap.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).Return(&datastore.Endpoint{
					UID:          "abc",
					Title:        "Title",
					GroupID:      "123",
					SupportEmail: "SupportEmail",
				}, nil).Times(1)
			},
			wantEvents: []datastore.Event{
				{
					UID:       "1234",
					Endpoints: []string{"abc"},
					EndpointMetadata: []*datastore.Endpoint{{
						UID:          "abc",
						Title:        "Title",
						GroupID:      "123",
						SupportEmail: "SupportEmail",
					}},
				},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     1,
				Page:      1,
				PerPage:   2,
				Prev:      1,
				Next:      3,
				TotalPage: 2,
			},
		},
		{
			name: "should_fail_to_get_events_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "ref",
					Status:     []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventRepo.(*mocks.MockEventRepository)
				ed.EXPECT().
					LoadEventsPaged(gomock.Any(), gomock.Any()).
					Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching events",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			events, paginationData, err := es.GetEventsPaged(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantEvents, events)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}

func TestEventService_SearchEvents(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name               string
		args               args
		dbFn               func(es *EventService)
		wantEvents         []datastore.Event
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_get_event_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					Pageable: datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
					},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				se, _ := es.searcher.(*mocks.MockSearcher)
				se.EXPECT().Search(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]string{"1234"}, datastore.PaginationData{
						Total:     1,
						Page:      1,
						PerPage:   2,
						Prev:      1,
						Next:      3,
						TotalPage: 2,
					}, nil)

				ed, _ := es.eventRepo.(*mocks.MockEventRepository)
				ed.EXPECT().FindEventsByIDs(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]datastore.Event{{UID: "1234"}}, nil)
			},
			wantEvents: []datastore.Event{
				{UID: "1234"},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     1,
				Page:      1,
				PerPage:   2,
				Prev:      1,
				Next:      3,
				TotalPage: 2,
			},
		},
		{
			name: "should_fail_to_get_events_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "ref",
					Status:     []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.searcher.(*mocks.MockSearcher)
				ed.EXPECT().
					Search(gomock.Any(), gomock.Any()).
					Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			events, paginationData, err := es.Search(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantEvents, events)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}

func TestEventService_GetEventDeliveriesPaged(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name                string
		args                args
		dbFn                func(es *EventService)
		wantEventDeliveries []datastore.EventDelivery
		wantPaginationData  datastore.PaginationData
		wantErr             bool
		wantErrCode         int
		wantErrMsg          string
	}{
		{
			name: "should_get_event_deliveries_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "123",
					Pageable: datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
					},
					Status: []datastore.EventDeliveryStatus{datastore.SuccessEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().LoadEventDeliveriesPaged(
					gomock.Any(),
					"123",
					"abc",
					"123",
					[]datastore.EventDeliveryStatus{datastore.SuccessEventStatus},
					datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
					datastore.Pageable{
						Page:    1,
						PerPage: 1,
						Sort:    1,
					}).
					Times(1).
					Return([]datastore.EventDelivery{{
						UID:        "1234",
						EndpointID: "12345",
					}}, datastore.PaginationData{
						Total:     1,
						Page:      1,
						PerPage:   2,
						Prev:      1,
						Next:      3,
						TotalPage: 2,
					}, nil)

				ev, _ := es.eventRepo.(*mocks.MockEventRepository)
				ev.EXPECT().FindEventByID(gomock.Any(), gomock.Any()).Return(&datastore.Event{
					UID:       "123",
					EventType: "incoming",
				}, nil)

				en, _ := es.endpointRepo.(*mocks.MockEndpointRepository)
				en.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).Return(&datastore.Endpoint{
					UID:            "1234",
					Title:          "Title",
					GroupID:        "123",
					SupportEmail:   "SupportEmail",
					TargetURL:      "http://localhost.com",
					DocumentStatus: "Active",
					Secrets: []datastore.Secret{
						{
							UID:   "abc",
							Value: "Secret",
						},
					},
					HttpTimeout:       "30s",
					RateLimit:         10,
					RateLimitDuration: "1h",
				}, nil)
			},
			wantEventDeliveries: []datastore.EventDelivery{
				{
					UID:        "1234",
					EndpointID: "12345",
					Event: &datastore.Event{
						UID:       "123",
						EventType: "incoming",
					},
					Endpoint: &datastore.Endpoint{
						UID:            "1234",
						Title:          "Title",
						GroupID:        "123",
						SupportEmail:   "SupportEmail",
						TargetURL:      "http://localhost.com",
						DocumentStatus: "Active",
						Secrets: []datastore.Secret{
							{
								UID:   "abc",
								Value: "Secret",
							},
						},
						HttpTimeout:       "30s",
						RateLimit:         10,
						RateLimitDuration: "1h",
					},
				},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     1,
				Page:      1,
				PerPage:   2,
				Prev:      1,
				Next:      3,
				TotalPage: 2,
			},
		},
		{
			name: "should_fail_to_get_events_deliveries_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Group:      &datastore.Group{UID: "123"},
					EndpointID: "abc",
					EventID:    "ref",
					Status:     []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().
					LoadEventDeliveriesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching event deliveries",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			eventDeliveries, paginationData, err := es.GetEventDeliveriesPaged(tc.args.ctx, tc.args.filter)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantEventDeliveries, eventDeliveries)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}

func TestEventService_ResendEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Group
	}
	tests := []struct {
		name       string
		dbFn       func(es *EventService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_retry_event_delivery",
			dbFn: func(es *EventService) {
				a, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				a.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{Status: datastore.ActiveSubscriptionStatus}, nil)

				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Group{UID: "abc"},
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
				g: &datastore.Group{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "event already sent",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err := es.ResendEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_RetryEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Group
	}
	tests := []struct {
		name       string
		dbFn       func(es *EventService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_retry_event_delivery",
			dbFn: func(es *EventService) {
				s, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{}, nil)

				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Group{UID: "abc"},
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
				g: &datastore.Group{UID: "abc"},
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
				g: &datastore.Group{UID: "abc"},
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
				g: &datastore.Group{UID: "abc"},
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
				g: &datastore.Group{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "cannot resend event that did not fail previously",
		},
		{
			name: "should_fail_to_find_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil, ErrSubscriptionNotFound)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Group{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "subscription not found",
		},
		{
			name: "should_error_for_pending_subscription_status",
			dbFn: func(es *EventService) {
				s, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{
					Status: datastore.PendingSubscriptionStatus,
				}, nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Group{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "subscription is being re-activated",
		},
		{
			name: "should_retry_event_delivery_with_inactive_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{
					Status: datastore.InactiveSubscriptionStatus,
				}, nil)

				s.EXPECT().UpdateSubscriptionStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.PendingSubscriptionStatus).
					Times(1).Return(nil)

				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Group{UID: "abc"},
			},
		},
		{
			name: "should_fail_to_retry_event_delivery_with_inactive_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.subRepo.(*mocks.MockSubscriptionRepository)

				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{Status: datastore.InactiveSubscriptionStatus}, nil)

				s.EXPECT().UpdateSubscriptionStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.PendingSubscriptionStatus).
					Times(1).Return(errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.FailureEventStatus,
				},
				g: &datastore.Group{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: "failed to update subscription status",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err := es.RetryEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestEventService_forceResendEventDelivery(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Group
	}
	tests := []struct {
		name       string
		dbFn       func(es *EventService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_force_resend_event_delivery",
			dbFn: func(es *EventService) {
				s, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{
					Status: datastore.ActiveSubscriptionStatus,
				}, nil)

				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				q, _ := es.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Group{Name: "test_group"},
			},
		},
		{
			name: "should_fail_to_find_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil, errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Group{Name: "test_group"},
			},
			wantErr:    true,
			wantErrMsg: "subscription not found",
		},
		{
			name: "should_error_not_active_subscription",
			dbFn: func(es *EventService) {
				s, _ := es.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{
					Status: datastore.PendingSubscriptionStatus,
				}, nil)
			},
			args: args{
				ctx: ctx,
				eventDelivery: &datastore.EventDelivery{
					UID:    "123",
					Status: datastore.SuccessEventStatus,
				},
				g: &datastore.Group{Name: "test_group"},
			},
			wantErr:    true,
			wantErrMsg: "force resend to an inactive or pending endpoint is not allowed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err := es.forceResendEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
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
		ctx           context.Context
		eventDelivery *datastore.EventDelivery
		g             *datastore.Group
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(es *EventService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_requeue_event_delivery",
			args: args{
				ctx:           ctx,
				eventDelivery: &datastore.EventDelivery{UID: "123"},
				g:             &datastore.Group{Name: "test_group"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
		},
		{
			name: "should_fail_update_event_delivery_status",
			args: args{
				ctx:           ctx,
				eventDelivery: &datastore.EventDelivery{UID: "123"},
				g:             &datastore.Group{Name: "test_group"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
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
				g:             &datastore.Group{Name: "test_group"},
			},
			dbFn: func(es *EventService) {
				ed, _ := es.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), datastore.ScheduledEventStatus).
					Times(1).Return(nil)

				eq, _ := es.queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "error occurred re-enqueing old event - 123: failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			es := provideEventService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err := es.requeueEventDelivery(tc.args.ctx, tc.args.eventDelivery, tc.args.g)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
