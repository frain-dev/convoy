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
	endpointRepo      datastore.EndpointRepository
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
	cache := mocks.NewMockCache(ctrl)
	queue := mocks.NewMockQueuer(ctrl)
	search := mocks.NewMockSearcher(ctrl)
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)

	return &args{
		endpointRepo:      endpointRepo,
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
		event      *CreateEvent
		dbFn       func(args *args)
		wantErr    bool
		wantErrMsg string
		wantDelay  time.Duration
	}{
		{
			name: "should_process_event_for_outgoing_group",
			event: &CreateEvent{
				Event: datastore.Event{
					UID:       uuid.NewString(),
					EventType: "*",
					SourceID:  "source-id-1",
					GroupID:   "group-id-1",
					Endpoints: []string{"endpoint-id-1"},
					Data:      []byte(`{}`),
					CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
				},
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

				mockCache.EXPECT().Get(gomock.Any(), "endpoints:endpoint-id-1", gomock.Any()).Times(1).Return(nil)

				a, _ := args.endpointRepo.(*mocks.MockEndpointRepository)

				endpoint := &datastore.Endpoint{UID: "endpoint-id-1"}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1").Times(1).Return(endpoint, nil)
				mockCache.EXPECT().Set(gomock.Any(), "endpoints:endpoint-id-1", endpoint, 10*time.Minute).Times(1).Return(nil)

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				subscriptions := []datastore.Subscription{
					{
						UID:        "456",
						EndpointID: "endpoint-id-1",
						Type:       datastore.SubscriptionTypeAPI,
						Status:     datastore.ActiveSubscriptionStatus,
						FilterConfig: &datastore.FilterConfiguration{
							EventTypes: []string{"*"},
						},
					},
				}
				s.EXPECT().FindSubscriptionsByEndpointID(gomock.Any(), "group-id-1", "endpoint-id-1").Times(1).Return(subscriptions, nil)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(true, nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				endpoint = &datastore.Endpoint{UID: "098", TargetURL: "https://google.com"}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1").
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
			name: "should_process_event_for_outgoing_group_without_subscription",
			event: &CreateEvent{
				Event: datastore.Event{
					UID:       uuid.NewString(),
					EventType: "*",
					SourceID:  "source-id-1",
					GroupID:   "group-id-1",
					Endpoints: []string{"endpoint-id-1"},
					Data:      []byte(`{}`),
					CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
				},
				CreateSubscription: true,
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

				mockCache.EXPECT().Get(gomock.Any(), "endpoints:endpoint-id-1", gomock.Any()).Times(1).Return(nil)

				a, _ := args.endpointRepo.(*mocks.MockEndpointRepository)

				endpoint := &datastore.Endpoint{UID: "endpoint-id-1"}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1").Times(1).Return(endpoint, nil)
				mockCache.EXPECT().Set(gomock.Any(), "endpoints:endpoint-id-1", endpoint, 10*time.Minute).Times(1).Return(nil)

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				subscriptions := []datastore.Subscription{}

				s.EXPECT().FindSubscriptionsByEndpointID(gomock.Any(), "group-id-1", "endpoint-id-1").Times(1).Return(subscriptions, nil)

				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				endpoint = &datastore.Endpoint{UID: "098", TargetURL: "https://google.com"}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1").
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
			event: &CreateEvent{
				Event: datastore.Event{
					UID:       uuid.NewString(),
					EventType: "*",
					SourceID:  "source-id-1",
					GroupID:   "group-id-1",
					Endpoints: []string{"endpoint-id-1"},
					Data:      []byte(`{}`),
					CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
				},
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

				a, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				endpoint := &datastore.Endpoint{UID: "endpoint-id-1"}

				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				subscriptions := []datastore.Subscription{
					{
						UID:        "456",
						EndpointID: "endpoint-id-1",
						Type:       datastore.SubscriptionTypeAPI,
						Status:     datastore.ActiveSubscriptionStatus,
						FilterConfig: &datastore.FilterConfiguration{
							EventTypes: []string{"*"},
						},
					},
				}
				s.EXPECT().FindSubscriptionsBySourceIDs(gomock.Any(), "group-id-1", "source-id-1").Times(1).Return(subscriptions, nil)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(true, nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				endpoint = &datastore.Endpoint{UID: "098", TargetURL: "https://google.com"}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1").
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

			fn := ProcessEventCreation(args.endpointRepo, args.eventRepo, args.groupRepo, args.eventDeliveryRepo, args.cache, args.eventQueue, args.subRepo, args.search, args.deviceRepo)
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

func TestMatchSubscriptionsUsingFilter(t *testing.T) {
	tests := []struct {
		name       string
		payload    map[string]interface{}
		dbFn       func(args *args)
		inputSubs  []datastore.Subscription
		wantSubs   []datastore.Subscription
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "Match all Filter",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 10,
				},
			},
			dbFn: func(args *args) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(true, nil)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{"person.age": 10},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{},
					},
				},
			},
			wantSubs: []datastore.Subscription{
				{
					UID: "123",
				},
				{
					UID: "1234",
				},
			},
		},
		{
			name: "Equal Filter",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 10,
				},
			},
			dbFn: func(args *args) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{"person.age": 10},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{"person.age": 5},
					},
				},
			},
			wantSubs: []datastore.Subscription{
				{
					UID: "123",
				},
			},
		},
		{
			name: "Equal Filter using operator",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 10,
				},
			},
			dbFn: func(args *args) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$eq": 10,
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{"person.age": 5},
					},
				},
			},
			wantSubs: []datastore.Subscription{
				{
					UID: "123",
				},
			},
		},
		{
			name: "Not Equal Filter",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 100,
				},
			},
			dbFn: func(args *args) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{"person.age": 10},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$neq": 10,
							},
						},
					},
				},
			},
			wantSubs: []datastore.Subscription{
				{
					UID: "1234",
				},
			},
		},
		{
			name: "Greater than Filter",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 10,
				},
			},
			dbFn: func(args *args) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$gte": 10,
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$gt": 10,
							},
						},
					},
				},
			},
			wantSubs: []datastore.Subscription{
				{
					UID: "123",
				},
			},
		},
		{
			name: "Less than Filter",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 9,
				},
			},
			dbFn: func(args *args) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(true, nil)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$lte": 10,
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$lt": 10,
							},
						},
					},
				},
			},
			wantSubs: []datastore.Subscription{
				{
					UID: "123",
				},
				{
					UID: "1234",
				},
			},
		},
		{
			name: "In array Filter",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 10,
				},
			},
			dbFn: func(args *args) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(false, nil)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$in": []int{10, 1},
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$in": []int{10, 1},
							},
						},
					},
				},
				{
					UID: "12345",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"person.age": map[string]interface{}{
								"$gt": 10,
							},
						},
					},
				},
			},
			wantSubs: []datastore.Subscription{
				{
					UID: "123",
				},
			},
		},
		{
			name: "Not in array Filter",
			payload: map[string]interface{}{
				"event": map[string]interface{}{
					"action": "update",
				},
			},
			dbFn: func(args *args) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
				s.EXPECT().TestSubscriptionFilter(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(true, nil)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"event.action": map[string]interface{}{
								"$nin": []string{"update", "delete"},
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: map[string]interface{}{
							"event.action": map[string]interface{}{
								"$nin": []string{"read", "delete"},
							},
						},
					},
				},
			},
			wantSubs: []datastore.Subscription{
				{
					UID: "1234",
				},
			},
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

			payload, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			subs, err := matchSubscriptionsUsingFilter(context.Background(), datastore.Event{Data: payload}, args.subRepo, tt.inputSubs)
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
			require.Equal(t, len(tt.wantSubs), len(subs))
			for i := range tt.wantSubs {
				require.Equal(t, tt.wantSubs[i].UID, subs[i].UID)
			}
		})
	}

}
