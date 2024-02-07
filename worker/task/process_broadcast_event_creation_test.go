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

				endpoint := &datastore.Endpoint{
					UID:    "endpoint-id-1",
					Title:  "testing-1",
					Status: datastore.ActiveEndpointStatus,
					Secrets: datastore.Secrets{
						{
							UID:   "secret-1",
							Value: "1234",
						},
					},
				}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", "project-id-1").Times(1).Return(endpoint, nil)

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				subscriptions := []datastore.Subscription{
					{
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
					},
				}
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(true, nil)

				s.EXPECT().LoadSubscriptionsPaged(gomock.Any(), "project-id-1", &datastore.FilterBy{}, datastore.Pageable{
					PerPage:    3500,
					Direction:  datastore.Next,
					NextCursor: "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
				}).Times(1).Return(subscriptions, datastore.PaginationData{
					PerPage:         3500,
					HasNextPage:     false,
					HasPreviousPage: false,
					PrevPageCursor:  "",
					NextPageCursor:  "",
				}, nil)

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

			payload, err := json.Marshal(tt.dynamicEvent)
			require.NoError(t, err)

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			fn := ProcessBroadcastEventCreation(args.endpointRepo, args.eventRepo, args.projectRepo, args.eventDeliveryRepo, args.eventQueue, args.subRepo, args.deviceRepo)
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
