package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"

	"github.com/frain-dev/convoy/api/models"

	"github.com/frain-dev/convoy"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
)

func TestProcessDynamicEventCreation(t *testing.T) {
	tests := []struct {
		name         string
		dynamicEvent *models.DynamicEvent
		dbFn         func(args *testArgs)
		wantErr      bool
		wantErrMsg   string
		wantDelay    time.Duration
	}{
		{
			name: "should_create_dynamic_event",
			dynamicEvent: &models.DynamicEvent{
				URL:            "https://google.com",
				Secret:         "1234",
				EventTypes:     []string{"*"},
				Data:           []byte(`{"name":"daniel"}`),
				ProjectID:      "project-id-1",
				EventType:      "*",
				IdempotencyKey: "idem-key-1",
			},
			dbFn: func(args *testArgs) {
				project := &datastore.Project{
					UID:  "project-id-1",
					Type: datastore.OutgoingProject,
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       datastore.LinearStrategyProvider,
							Duration:   10,
							RetryCount: 3,
						},
					},
				}

				g, _ := args.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "project-id-1").Times(1).Return(
					project,
					nil,
				)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), "project-id-1", "idem-key-1").Times(1).Return(nil, nil)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_create_new_endpoint_and_subscription_for_dynamic_event",
			dynamicEvent: &models.DynamicEvent{
				URL:       "https://google.com",
				Secret:    "1234",
				Data:      []byte(`{"name":"daniel"}`),
				ProjectID: "project-id-1",
				EventType: "*",
			},
			dbFn: func(args *testArgs) {
				project := &datastore.Project{
					UID:  "project-id-1",
					Type: datastore.OutgoingProject,
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       datastore.LinearStrategyProvider,
							Duration:   10,
							RetryCount: 3,
						},
					},
				}

				g, _ := args.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "project-id-1").Times(1).Return(
					project,
					nil,
				)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			args := provideArgs(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(args)
			}

			payload, err := json.Marshal(tt.dynamicEvent)
			require.NoError(t, err)

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			fn := ProcessDynamicEventCreation(NewDynamicEventChannel(), args.endpointRepo, args.eventRepo, args.projectRepo, args.eventDeliveryRepo, args.eventQueue, args.subRepo, args.deviceRepo, args.licenser, args.tracer)
			err = fn(context.Background(), task)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*EndpointError).Error())
				require.Equal(t, tt.wantDelay, err.(*EndpointError).Delay())
				return
			}

			require.Nil(t, err)
		})
	}
}
