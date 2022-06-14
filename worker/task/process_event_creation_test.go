package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

type args struct {
	appRepo           datastore.ApplicationRepository
	eventRepo         datastore.EventRepository
	groupRepo         datastore.GroupRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	cache             cache.Cache
	eventQueue        queue.Queuer
	subRepo           datastore.SubscriptionRepository
}

func provideArgs(ctrl *gomock.Controller) *args {
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	cache := mocks.NewMockCache(ctrl)
	queue := mocks.NewMockQueuer(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)

	return &args{
		appRepo:           appRepo,
		eventRepo:         eventRepo,
		groupRepo:         groupRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		cache:             cache,
		eventQueue:        queue,
		subRepo:           subRepo,
	}
}

func TestProcessEventCreated(t *testing.T) {
	tests := []struct {
		name  string
		event *datastore.Event
		dbFn  func(args *args)
		want  func(context.Context, *asynq.Task) error
	}{
		{
			name: "should_process_event",
			event: &datastore.Event{
				UID:        uuid.NewString(),
				EventType:  "*",
				ProviderID: uuid.NewString(),
				SourceID:   "source-id-1",
				GroupID:    "group-id-1",
				AppID:      "app-id-1",
				Data:       []byte(`{}`),
				CreatedAt:  primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:  primitive.NewDateTimeFromTime(time.Now()),
			},
			dbFn: func(args *args) {
				mockCache, _ := args.cache.(*mocks.MockCache)
				var gr *datastore.Group
				mockCache.EXPECT().Get(gomock.Any(), "groups:group-id-1", &gr).Times(1).Return(nil)

				group := &datastore.Group{
					UID:  "123",
					Type: datastore.OutgoingGroup,
					Config: &datastore.GroupConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       datastore.LinearStrategyProvider,
							Duration:   10,
							RetryCount: 3,
						},
					},
				}

				g, _ := args.groupRepo.(*mocks.MockGroupRepository)
				g.EXPECT().FetchGroupByID(gomock.Any(), "group-id-1").Times(1).Return(
					group,
					nil,
				)
				mockCache.EXPECT().Set(gomock.Any(), "groups:group-id-1", group, 10*time.Minute).Times(1).Return(nil)

				var app *datastore.Application
				mockCache.EXPECT().Get(gomock.Any(), "applications:app-id-1", &app).Times(1).Return(nil)

				a, _ := args.appRepo.(*mocks.MockApplicationRepository)

				app = &datastore.Application{UID: "abc"}
				a.EXPECT().FindApplicationByID(gomock.Any(), app).Times(1).Return(app, nil)
				mockCache.EXPECT().Set(gomock.Any(), "applications:app-id-1", &app, 10*time.Minute).Times(1).Return(nil)

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				subscriptions := []datastore.Subscription{
					{UID: "456"},
				}
				s.EXPECT().FindSubscriptionsByAppID(gomock.Any(), "group-id-1", "app-id-1").Times(1).Return(subscriptions, nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				event := &datastore.Event{
					UID:            uuid.NewString(),
					EventType:      "*",
					ProviderID:     uuid.NewString(),
					SourceID:       "source-id-1",
					GroupID:        "group-id-1",
					AppID:          "app-id-1",
					Data:           []byte(`{}`),
					DocumentStatus: datastore.ActiveDocumentStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				}
				e.EXPECT().CreateEvent(gomock.Any(), event).Times(1).Return(nil)

			},
			want: nil,
		},
	}
	for _, tt := range tests {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		args := provideArgs(ctrl)

		t.Run(tt.name, func(t *testing.T) {
			if tt.dbFn != nil {
				tt.dbFn(args)
			}

			payload, err := json.Marshal(tt.event)
			require.NoError(t, err)

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			fn := ProcessEventCreated(args.appRepo, args.eventRepo, args.groupRepo, args.eventDeliveryRepo, args.cache, args.eventQueue, args.subRepo)
			err = fn(context.Background(), task)

		})
	}
}
