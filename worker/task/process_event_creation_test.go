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
	"github.com/frain-dev/convoy/internal/pkg/searcher"
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
	search            searcher.Searcher
	deviceRepo        datastore.DeviceRepository
}

func provideArgs(ctrl *gomock.Controller) *args {
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	cache := mocks.NewMockCache(ctrl)
	queue := mocks.NewMockQueuer(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	search := mocks.NewMockSearcher(ctrl)

	return &args{
		appRepo:           appRepo,
		eventRepo:         eventRepo,
		groupRepo:         groupRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		cache:             cache,
		eventQueue:        queue,
		subRepo:           subRepo,
		search:            search,
	}
}

func TestProcessEventCreated(t *testing.T) {
	tests := []struct {
		name       string
		event      *datastore.Event
		dbFn       func(args *args)
		wantErr    bool
		wantErrMsg string
		wantDelay  time.Duration
	}{
		{
			name: "should_process_event_for_outgoing_group",
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
					UID:  "group-id-1",
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

				mockCache.EXPECT().Get(gomock.Any(), "applications:app-id-1", gomock.Any()).Times(1).Return(nil)

				a, _ := args.appRepo.(*mocks.MockApplicationRepository)

				app := &datastore.Application{UID: "app-id-1"}
				a.EXPECT().FindApplicationByID(gomock.Any(), "app-id-1").Times(1).Return(app, nil)
				mockCache.EXPECT().Set(gomock.Any(), "applications:app-id-1", app, 10*time.Minute).Times(1).Return(nil)

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				subscriptions := []datastore.Subscription{
					{
						UID:        "456",
						AppID:      "app-id-1",
						EndpointID: "098",
						Type:       datastore.SubscriptionTypeAPI,
						Status:     datastore.ActiveSubscriptionStatus,
						FilterConfig: &datastore.FilterConfiguration{
							EventTypes: []string{"*"},
						},
					},
				}
				s.EXPECT().FindSubscriptionsByAppID(gomock.Any(), "group-id-1", "app-id-1").Times(1).Return(subscriptions, nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				a.EXPECT().FindApplicationByID(gomock.Any(), "app-id-1").Times(1).Return(app, nil)

				endpoint := &datastore.Endpoint{UID: "098", TargetURL: "https://google.com"}
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), "app-id-1", "098").
					Times(1).Return(endpoint, nil)

				ed, _ := args.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().CreateEventDelivery(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).Times(1).Return(nil)

				q.EXPECT().Write(convoy.IndexDocument, convoy.PriorityQueue, gomock.Any()).Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_process_event_for_incoming_group",
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
					UID:  "group-id-1",
					Type: datastore.IncomingGroup,
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

				a, _ := args.appRepo.(*mocks.MockApplicationRepository)
				app := &datastore.Application{UID: "app-id-1"}

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				subscriptions := []datastore.Subscription{
					{
						UID:        "456",
						AppID:      "app-id-1",
						EndpointID: "098",
						Type:       datastore.SubscriptionTypeAPI,
						Status:     datastore.ActiveSubscriptionStatus,
						FilterConfig: &datastore.FilterConfiguration{
							EventTypes: []string{"*"},
						},
					},
				}
				s.EXPECT().FindSubscriptionsBySourceIDs(gomock.Any(), "group-id-1", "source-id-1").Times(1).Return(subscriptions, nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				a.EXPECT().FindApplicationByID(gomock.Any(), "app-id-1").Times(1).Return(app, nil)

				endpoint := &datastore.Endpoint{UID: "098", TargetURL: "https://google.com"}
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), "app-id-1", "098").
					Times(1).Return(endpoint, nil)

				ed, _ := args.eventDeliveryRepo.(*mocks.MockEventDeliveryRepository)
				ed.EXPECT().CreateEventDelivery(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.EventProcessor, convoy.EventQueue, gomock.Any()).Times(1).Return(nil)

				q.EXPECT().Write(convoy.IndexDocument, convoy.PriorityQueue, gomock.Any()).Times(1).Return(nil)
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

			payload, err := json.Marshal(tt.event)
			require.NoError(t, err)

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			fn := ProcessEventCreation(args.appRepo, args.eventRepo, args.groupRepo, args.eventDeliveryRepo, args.cache, args.eventQueue, args.subRepo, args.search, args.deviceRepo)
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
