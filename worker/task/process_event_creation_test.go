package task

import (
	"context"
	"encoding/json"
	"github.com/oklog/ulid/v2"
	"testing"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type testArgs struct {
	endpointRepo      datastore.EndpointRepository
	eventRepo         datastore.EventRepository
	projectRepo       datastore.ProjectRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	db                database.Database
	cache             cache.Cache
	eventQueue        queue.Queuer
	subRepo           datastore.SubscriptionRepository
	deviceRepo        datastore.DeviceRepository
	subTable          memorystore.ITable
	licenser          license.Licenser
	tracer            tracer.Backend
}

func provideArgs(ctrl *gomock.Controller) *testArgs {
	mockCache := mocks.NewMockCache(ctrl)
	mockQueuer := mocks.NewMockQueuer(ctrl)
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	deviceRepo := mocks.NewMockDeviceRepository(ctrl)
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	db := mocks.NewMockDatabase(ctrl)
	subTable := mocks.NewMockITable(ctrl)
	mockTracer := mocks.NewMockBackend(ctrl)

	return &testArgs{
		endpointRepo:      endpointRepo,
		deviceRepo:        deviceRepo,
		eventRepo:         eventRepo,
		projectRepo:       projectRepo,
		db:                db,
		eventDeliveryRepo: eventDeliveryRepo,
		cache:             mockCache,
		eventQueue:        mockQueuer,
		subRepo:           subRepo,
		subTable:          subTable,
		licenser:          mocks.NewMockLicenser(ctrl),
		tracer:            mockTracer,
	}
}

func TestProcessEventCreated(t *testing.T) {
	tests := []struct {
		name        string
		createEvent *CreateEvent
		dbFn        func(args *testArgs)
		wantErr     bool
		wantErrMsg  string
		wantDelay   time.Duration
	}{
		{
			name: "should_process_event_for_outgoing_project",
			createEvent: &CreateEvent{
				JobID: "123",
				Params: CreateEventTaskParams{
					UID:            "01JMJ3WTZGP411PY39KSY8AFQF",
					ProjectID:      "project-id-1",
					OwnerID:        "owner-id-1",
					AppID:          "app-id-1",
					EndpointID:     "endpoint-id-1",
					SourceID:       "source-id-1",
					Data:           []byte(`{"name":"daniel"}`),
					EventType:      "*",
					IdempotencyKey: "idem-key-1",
				},
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

				a, _ := args.endpointRepo.(*mocks.MockEndpointRepository)

				endpoint := &datastore.Endpoint{UID: "endpoint-id-1"}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).Times(1).Return(endpoint, nil)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(nil, nil)
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrEventNotFound)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

				mockTracer, _ := args.tracer.(*mocks.MockBackend)
				mockTracer.EXPECT().Capture(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr: false,
		},
		{
			name: "should_process_event_for_outgoing_project_without_subscription",
			createEvent: &CreateEvent{
				JobID: "123",
				Event: &datastore.Event{
					UID:            ulid.Make().String(),
					EventType:      "*",
					SourceID:       "source-id-1",
					ProjectID:      "project-id-1",
					Endpoints:      []string{"endpoint-id-1"},
					Data:           []byte(`{}`),
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
					IdempotencyKey: "idem-key-1",
				},
				CreateSubscription: true,
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
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, nil)
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrEventNotFound)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

				mockTracer, _ := args.tracer.(*mocks.MockBackend)
				mockTracer.EXPECT().Capture(gomock.Any(), "event.creation.success", gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr: false,
		},
		{
			name: "should_process_event_for_incoming_project_api_event",
			createEvent: &CreateEvent{
				JobID: "123",
				Event: &datastore.Event{
					UID:            ulid.Make().String(),
					EventType:      "*",
					SourceID:       "source-id-1",
					ProjectID:      "project-id-1",
					Endpoints:      []string{"endpoint-id-1"},
					Data:           []byte(`{}`),
					IdempotencyKey: "idem-key-1",
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
				},
			},
			dbFn: func(args *testArgs) {
				project := &datastore.Project{
					UID:  "project-id-1",
					Type: datastore.IncomingProject,
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
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, nil)
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrEventNotFound)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

				mockTracer, _ := args.tracer.(*mocks.MockBackend)
				mockTracer.EXPECT().Capture(gomock.Any(), "event.creation.success", gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr: false,
		},
		{
			name: "should_process_event_for_incoming_project_cli_event",
			createEvent: &CreateEvent{
				JobID: "123",
				Event: &datastore.Event{
					UID:            ulid.Make().String(),
					EventType:      "*",
					SourceID:       "source-id-1",
					ProjectID:      "project-id-1",
					Endpoints:      []string{"endpoint-id-1"},
					Data:           []byte(`{}`),
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
					IdempotencyKey: "idem-key-1",
				},
			},
			dbFn: func(args *testArgs) {
				project := &datastore.Project{
					UID:  "project-id-1",
					Type: datastore.IncomingProject,
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
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, nil)
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrEventNotFound)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

				mockTracer, _ := args.tracer.(*mocks.MockBackend)
				mockTracer.EXPECT().Capture(gomock.Any(), "event.creation.success", gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
			},
			wantErr: false,
		},
		{
			name: "should_process_replayed_event",
			createEvent: &CreateEvent{
				JobID: "123",
				Event: &datastore.Event{
					UID:            ulid.Make().String(),
					EventType:      "*",
					SourceID:       "source-id-1",
					ProjectID:      "project-id-1",
					Endpoints:      []string{"endpoint-id-1"},
					Data:           []byte(`{}`),
					CreatedAt:      time.Now(),
					UpdatedAt:      time.Now(),
					IdempotencyKey: "idem-key-1",
				},
			},
			dbFn: func(args *testArgs) {
				project := &datastore.Project{
					UID:  "project-id-1",
					Type: datastore.IncomingProject,
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
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

				mockTracer, _ := args.tracer.(*mocks.MockBackend)
				mockTracer.EXPECT().Capture(gomock.Any(), "event.creation.success", gomock.Any(), gomock.Any(), gomock.Any()).Times(1)
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

			payload, err := msgpack.EncodeMsgPack(tt.createEvent)
			require.NoError(t, err)

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			fn := ProcessEventCreation(NewDefaultEventChannel(), args.endpointRepo, args.eventRepo, args.projectRepo, args.eventDeliveryRepo, args.eventQueue, args.subRepo, args.deviceRepo, args.licenser, args.tracer)
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
		dbFn       func(args *testArgs)
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
			dbFn: func(args *testArgs) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(true, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{"person.age": 10},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{},
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
			name: "Should skip filter for advanced subscriptions license check failed",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 10,
				},
			},
			dbFn: func(args *testArgs) {
				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(false)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{"person.age": 10},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{},
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
			name: "Equal Filter",
			payload: map[string]interface{}{
				"person": map[string]interface{}{
					"age": 10,
				},
			},
			dbFn: func(args *testArgs) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(true, nil)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(false, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{Body: map[string]interface{}{"person.age": 10}},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{Body: map[string]interface{}{"person.age": 5}},
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
			dbFn: func(args *testArgs) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(true, nil)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(false, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$eq": 10,
								},
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{Body: map[string]interface{}{"person.age": 5}},
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
			dbFn: func(args *testArgs) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(false, nil)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(true, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{Body: map[string]interface{}{"person.age": 10}},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$neq": 10,
								},
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
			dbFn: func(args *testArgs) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(true, nil)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(false, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$gte": 10,
								},
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$gt": 10,
								},
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
			dbFn: func(args *testArgs) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(4).Return(true, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$lte": 10,
								},
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$lt": 10,
								},
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
			dbFn: func(args *testArgs) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(true, nil)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(4).Return(false, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$in": []int{10, 1},
								},
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$in": []int{10, 1},
								},
							},
						},
					},
				},
				{
					UID: "12345",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"person.age": map[string]interface{}{
									"$gt": 10,
								},
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
			dbFn: func(args *testArgs) {
				s, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(false, nil)
				s.EXPECT().CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), false).Times(2).Return(true, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{
				{
					UID: "123",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{
							Body: map[string]interface{}{
								"event.action": map[string]interface{}{
									"$nin": []string{"update", "delete"},
								},
							},
						},
					},
				},
				{
					UID: "1234",
					FilterConfig: &datastore.FilterConfiguration{
						Filter: datastore.FilterSchema{Body: map[string]interface{}{
							"event.action": map[string]interface{}{
								"$nin": []string{"read", "delete"},
							},
						}},
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

			subs, err := matchSubscriptionsUsingFilter(context.Background(), &datastore.Event{Data: payload}, args.subRepo, args.licenser, tt.inputSubs, false)
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
