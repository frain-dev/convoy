package services

import (
	"bytes"
	"context"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
)

func provideCreateFanoutEventService(ctrl *gomock.Controller, event *models.FanoutEvent, project *datastore.Project) *CreateFanoutEventService {
	return &CreateFanoutEventService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Queue:        mocks.NewMockQueuer(ctrl),
		NewMessage:   event,
		Project:      project,
	}
}

func TestCreateFanoutEventService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx        context.Context
		newMessage *models.FanoutEvent
		g          *datastore.Project
	}

	tests := []struct {
		name       string
		dbFn       func(es *CreateFanoutEventService)
		args       args
		wantEvent  *datastore.Event
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_create_fanout_event_for_multiple_endpoints",
			dbFn: func(es *CreateFanoutEventService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByOwnerID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{
					{
						Title:        "test_app",
						UID:          "123",
						ProjectID:    "abc",
						SupportEmail: "test_app@gmail.com",
					},

					{
						Title:        "test_app",
						UID:          "12345",
						ProjectID:    "abc",
						SupportEmail: "test_app@gmail.com",
					},
				}, nil)
				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).
					Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.FanoutEvent{
					OwnerID:   "12345",
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
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
				Endpoints: []string{"123", "12345"},
				ProjectID: "abc",
			},
		},

		{
			name: "should_error_for_empty_endpoints",
			dbFn: func(es *CreateFanoutEventService) {
				a, _ := es.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointsByOwnerID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return([]datastore.Endpoint{}, nil)
			},
			args: args{
				ctx: ctx,
				newMessage: &models.FanoutEvent{
					OwnerID:   "12345",
					EventType: "payment.created",
					Data:      bytes.NewBufferString(`{"name":"convoy"}`).Bytes(),
				},
				g: &datastore.Project{},
			},
			wantErr:    true,
			wantErrMsg: ErrNoValidOwnerIDEndpointFound.Error(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideCreateFanoutEventService(ctrl, tc.args.newMessage, tc.args.g)
			require.NoError(t, err)

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
