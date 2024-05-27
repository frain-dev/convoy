package task

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"

	"github.com/frain-dev/convoy/api/models"

	"github.com/frain-dev/convoy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
)

func TestProcessBroadcastEventCreation(t *testing.T) {
	tests := []struct {
		name         string
		dynamicEvent *models.BroadcastEvent
		dbFn         func(args *args)
		wantErr      bool
		wantErrMsg   string
		wantDelay    time.Duration
	}{
		{
			name: "should_create_broadcast_event",
			dynamicEvent: &models.BroadcastEvent{
				EventType:      "some.*",
				ProjectID:      "project-id-1",
				Data:           []byte(`{"name":"daniel"}`),
				CustomHeaders:  nil,
				IdempotencyKey: "idem-key-1",
			},
			dbFn: func(args *args) {
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

				a, _ := args.endpointRepo.(*mocks.MockEndpointRepository)

				st, _ := args.subTable.(*mocks.MockITable)
				st.EXPECT().GetItems("project-id-1").Return([]*memorystore.Row{
					memorystore.NewRow("sub-1", datastore.Subscription{
						UID:        "sub-1",
						Name:       "test-sub",
						Type:       datastore.SubscriptionTypeAPI,
						ProjectID:  "project-id-1",
						EndpointID: "endpoint-id-1",
						FilterConfig: &datastore.FilterConfiguration{
							EventTypes: []string{"*"},
						},
						AlertConfig:     nil,
						RetryConfig:     nil,
						RateLimitConfig: nil,
					}),
				})

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(true, nil)

				d, _ := args.db.(*mocks.MockDatabase)
				d.EXPECT().BeginTx(gomock.Any())
				d.EXPECT().Rollback(gomock.Any(), gomock.Any())

				endpoint := &datastore.Endpoint{
					UID:    "endpoint-id-1",
					Name:   "testing-1",
					Status: datastore.ActiveEndpointStatus,
					Secrets: datastore.Secrets{
						{
							UID:   "secret-1",
							Value: "1234",
						},
					},
				}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", "project-id-1").Times(1).Return(endpoint, nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), "project-id-1", "idem-key-1").Times(1).Return(nil, nil)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				ed, _ := args.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().CreateEventDelivery(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).Times(1).Return(nil)
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

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			fn := ProcessBroadcastEventCreation(args.db, args.endpointRepo,
				args.eventRepo, args.projectRepo, args.eventDeliveryRepo, args.eventQueue, args.subRepo, args.deviceRepo, args.subTable)
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
