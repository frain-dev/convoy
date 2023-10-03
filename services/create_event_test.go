package services

import (
	"bytes"
	"context"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideCreateEventService(ctrl *gomock.Controller, event *models.CreateEvent, project *datastore.Project) *CreateEventService {
	return &CreateEventService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		EventRepo:    mocks.NewMockEventRepository(ctrl),
		Queue:        mocks.NewMockQueuer(ctrl),
		Cache:        mocks.NewMockCache(ctrl),
		NewMessage:   event,
		Project:      project,
	}
}

func TestCreateEventService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx        context.Context
		newMessage *models.CreateEvent
		g          *datastore.Project
	}
	tests := []struct {
		name       string
		dbFn       func(es *CreateEventService)
		args       args
		wantEvent  *datastore.Event
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_create_event",
			dbFn: func(es *CreateEventService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)
				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.CreateEvent{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:     &datastore.SignatureConfiguration{},
						ReplayAttacks: false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType: datastore.EventType("payment.created"),
				Raw:       `{"name":"convoy"}`,
				Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints: []string{"123"},
				ProjectID: "abc",
			},
		},

		{
			name: "should_create_event_with_exponential_backoff_strategy",
			dbFn: func(es *CreateEventService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)

				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.CreateEvent{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "exponential",
							Duration:   1000,
							RetryCount: 10,
						},
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType: datastore.EventType("payment.created"),
				Raw:       `{"name":"convoy"}`,
				Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints: []string{"123"},
				ProjectID: "abc",
			},
		},
		{
			name: "should_create_event_for_disabled_endpoint",
			dbFn: func(es *CreateEventService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)

				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.CreateEvent{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:     &datastore.SignatureConfiguration{},
						ReplayAttacks: false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType: datastore.EventType("payment.created"),
				Raw:       `{"name":"convoy"}`,
				Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints: []string{"123"},
				ProjectID: "abc",
			},
		},
		{
			name: "should_create_event_with_custom_headers",
			dbFn: func(es *CreateEventService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)

				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.CreateEvent{
					EndpointID:    "123",
					EventType:     "payment.created",
					Data:          bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
					CustomHeaders: map[string]string{"X-Test-Signature": "Test"},
				},
				g: &datastore.Project{
					UID:  "abc",
					Name: "test_project",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       "linear",
							Duration:   1000,
							RetryCount: 10,
						},
						Signature:     &datastore.SignatureConfiguration{},
						ReplayAttacks: false,
					},
				},
			},
			wantEvent: &datastore.Event{
				EventType: datastore.EventType("payment.created"),
				Raw:       `{"name":"convoy"}`,
				Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				Endpoints: []string{"123"},
				ProjectID: "abc",
				Headers:   httpheader.HTTPHeader{"X-Test-Signature": []string{"Test"}},
			},
		},
		{
			name: "should_error_for_invalid_strategy_config",
			dbFn: func(es *CreateEventService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(&datastore.Endpoint{
					Title:        "test_app",
					UID:          "123",
					ProjectID:    "abc",
					SupportEmail: "test_app@gmail.com",
				}, nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.CreateEvent{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{
					UID:    "abc",
					Name:   "test_project",
					Config: &datastore.ProjectConfig{},
				},
			},
			wantErr:    true,
			wantErrMsg: "retry strategy not defined in configuration",
		},
		{
			name: "should_error_for_empty_endpoints",
			args: args{
				ctx: ctx,
				newMessage: &models.CreateEvent{
					EndpointID: "",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{},
			},
			wantErr:    true,
			wantErrMsg: ErrInvalidEndpointID.Error(),
		},
		{
			name: "should_error_for_endpoint_not_found",
			dbFn: func(es *CreateEventService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "abc").
					Times(1).Return(nil, datastore.ErrEndpointNotFound)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.CreateEvent{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{UID: "abc"},
			},
			wantErr:    true,
			wantErrMsg: datastore.ErrEndpointNotFound.Error(),
		},

		{
			name: "should_fail_to_create_event",
			dbFn: func(es *CreateEventService) {},
			args: args{
				ctx: ctx,
				newMessage: &models.CreateEvent{
					EndpointID: "123",
					EventType:  "payment.created",
					Data:       bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while creating event - invalid project",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideCreateEventService(ctrl, tc.args.newMessage, tc.args.g)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			event, err := es.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
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
