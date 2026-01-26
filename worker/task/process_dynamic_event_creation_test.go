package task

import (
	"context"
	"testing"
	"time"

	jobenvelope "github.com/olamilekan000/surge/surge/job"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
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
				JobID:          "123:1234567890",
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

				mockTracer, _ := args.tracer.(*mocks.MockBackend)
				mockTracer.EXPECT().Capture(gomock.Any(), "dynamic.event.creation.success", gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr: false,
		},
		{
			name: "should_create_new_endpoint_and_subscription_for_dynamic_event",
			dynamicEvent: &models.DynamicEvent{
				JobID:     "123:1234567890",
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

				mockTracer, _ := args.tracer.(*mocks.MockBackend)
				mockTracer.EXPECT().Capture(gomock.Any(), "dynamic.event.creation.success", gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
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

			payload, err := msgpack.EncodeMsgPack(tt.dynamicEvent)
			require.NoError(t, err)

			job := &queue.Job{
				Payload: payload,
			}

			jobEnvelope := &jobenvelope.JobEnvelope{
				ID:        "",
				Topic:     string(convoy.EventProcessor),
				Args:      job.Payload,
				Namespace: "default",
				Queue:     string(convoy.EventQueue),
				State:     jobenvelope.StatePending,
				CreatedAt: time.Now(),
			}

			deps := EventProcessorDeps{
				EndpointRepo:       args.endpointRepo,
				EventRepo:          args.eventRepo,
				ProjectRepo:        args.projectRepo,
				EventQueue:         args.eventQueue,
				SubRepo:            args.subRepo,
				FilterRepo:         args.filterRepo,
				Licenser:           args.licenser,
				TracerBackend:      args.tracer,
				OAuth2TokenService: args.oauth2TokenService,
			}
			fn := ProcessDynamicEventCreation(deps)
			err = fn(context.Background(), jobEnvelope)
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
