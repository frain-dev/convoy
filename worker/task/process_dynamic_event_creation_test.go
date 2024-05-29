package task

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"

	"github.com/frain-dev/convoy/api/models"

	"github.com/frain-dev/convoy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
)

func TestProcessDynamicEventCreation(t *testing.T) {
	tests := []struct {
		name         string
		dynamicEvent *models.DynamicEvent
		dbFn         func(args *args)
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
				a.EXPECT().FindEndpointByTargetURL(gomock.Any(), "project-id-1", "https://google.com").Times(1).Return(endpoint, nil)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", "project-id-1").Times(1).Return(endpoint, nil)

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				subscriptions := []datastore.Subscription{
					{
						UID:             "sub-1",
						Name:            "test-sub",
						Type:            datastore.SubscriptionTypeAPI,
						ProjectID:       "project-id-1",
						EndpointID:      "endpoint-id-1",
						AlertConfig:     nil,
						RetryConfig:     nil,
						RateLimitConfig: nil,
					},
				}

				s.EXPECT().UpdateSubscription(gomock.Any(), "project-id-1", gomock.Any()).Times(1).Return(nil)

				s.EXPECT().FindSubscriptionsByEndpointID(gomock.Any(), "project-id-1", "endpoint-id-1").Times(1).Return(subscriptions, nil)

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
		{
			name: "should_create_new_endpoint_and_subscription_for_dynamic_event",
			dynamicEvent: &models.DynamicEvent{
				URL:       "https://google.com",
				Secret:    "1234",
				Data:      []byte(`{"name":"daniel"}`),
				ProjectID: "project-id-1",
				EventType: "*",
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

				a.EXPECT().FindEndpointByTargetURL(gomock.Any(), "project-id-1", "https://google.com").Times(1).Return(nil, datastore.ErrEndpointNotFound)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "project-id-1").Times(1).Return(&datastore.Endpoint{
					UID:                "endpoint-id-1",
					ProjectID:          project.UID,
					Name:               fmt.Sprintf("endpoint-%s", ulid.Make().String()),
					Url:                "https:/google.com",
					HttpTimeout:        convoy.HTTP_TIMEOUT,
					AdvancedSignatures: true,
					Status:             datastore.ActiveEndpointStatus,
					CreatedAt:          time.Now(),
					UpdatedAt:          time.Now(),
				}, nil)

				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), "project-id-1").
					Times(1).Return(nil)

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)

				s.EXPECT().FindSubscriptionsByEndpointID(gomock.Any(), "project-id-1", gomock.Any()).Times(1).Return(nil, datastore.ErrSubscriptionNotFound)

				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
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

			payload, err := json.Marshal(tt.dynamicEvent)
			require.NoError(t, err)

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			fn := ProcessDynamicEventCreation(args.endpointRepo, args.eventRepo, args.projectRepo, args.eventDeliveryRepo, args.eventQueue, args.subRepo, args.deviceRepo)
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
