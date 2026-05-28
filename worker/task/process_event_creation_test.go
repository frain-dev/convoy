package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/compare"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

type testArgs struct {
	endpointRepo       datastore.EndpointRepository
	eventRepo          datastore.EventRepository
	projectRepo        datastore.ProjectRepository
	eventDeliveryRepo  datastore.EventDeliveryRepository
	db                 database.Database
	cache              cache.Cache
	eventQueue         queue.Queuer
	subRepo            datastore.SubscriptionRepository
	filterRepo         datastore.FilterRepository
	subTable           memorystore.ITable
	licenser           license.Licenser
	tracer             tracer.Backend
	logger             logger.Logger
	oauth2TokenService OAuth2TokenService
}

func provideArgs(ctrl *gomock.Controller) *testArgs {
	mockCache := mocks.NewMockCache(ctrl)
	mockQueuer := mocks.NewMockQueuer(ctrl)
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	eventDeliveryRepo := mocks.NewMockEventDeliveryRepository(ctrl)
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	db := mocks.NewMockDatabase(ctrl)
	subTable := mocks.NewMockITable(ctrl)
	filterRepo := mocks.NewMockFilterRepository(ctrl)
	mockTracer := mocks.NewMockBackend(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	// Create a simple mock OAuth2TokenService that returns empty token (no-op for tests)
	oAuth2TokenService := &mockOAuth2TokenService{}

	return &testArgs{
		endpointRepo:       endpointRepo,
		eventRepo:          eventRepo,
		projectRepo:        projectRepo,
		db:                 db,
		eventDeliveryRepo:  eventDeliveryRepo,
		cache:              mockCache,
		eventQueue:         mockQueuer,
		subRepo:            subRepo,
		subTable:           subTable,
		filterRepo:         filterRepo,
		licenser:           mocks.NewMockLicenser(ctrl),
		tracer:             mockTracer,
		logger:             mockLogger,
		oauth2TokenService: oAuth2TokenService,
	}
}

// mockOAuth2TokenService is a simple no-op implementation for tests
type mockOAuth2TokenService struct{}

func (m *mockOAuth2TokenService) GetAuthorizationHeader(ctx context.Context, endpoint *datastore.Endpoint) (string, error) {
	// Return empty token for tests - OAuth2 functionality is tested separately
	return "", nil
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
				JobID: "123:1234567890",
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
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).Times(2).Return(false, nil)
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrEventNotFound)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

			},
			wantErr: false,
		},
		{
			name: "should_process_event_for_outgoing_project_without_subscription",
			createEvent: &CreateEvent{
				JobID: "123:1234567890",
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
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrEventNotFound)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

			},
			wantErr: false,
		},
		{
			name: "should_process_event_for_incoming_project_api_event",
			createEvent: &CreateEvent{
				JobID: "123:1234567890",
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
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrEventNotFound)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

			},
			wantErr: false,
		},
		{
			name: "should_process_event_for_incoming_project_cli_event",
			createEvent: &CreateEvent{
				JobID: "123:1234567890",
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
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(false, nil)
				e.EXPECT().FindEventByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil, datastore.ErrEventNotFound)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

			},
			wantErr: false,
		},
		{
			name: "should_process_replayed_event",
			createEvent: &CreateEvent{
				JobID: "123:1234567890",
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
				q.EXPECT().Write(gomock.Any(), convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

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

			deps := EventProcessorDeps{
				EndpointRepo:       args.endpointRepo,
				EventRepo:          args.eventRepo,
				ProjectRepo:        args.projectRepo,
				EventQueue:         args.eventQueue,
				SubRepo:            args.subRepo,
				FilterRepo:         args.filterRepo,
				Licenser:           args.licenser,
				OAuth2TokenService: args.oauth2TokenService,
			}
			fn := ProcessEventCreation(deps)
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
		eventType  string
		payload    map[string]interface{}
		query      string
		path       string
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
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{"person.age": 10}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{}}, nil)

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
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{"person.age": 10}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{"person.age": 5}}, nil)

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
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{"person.age": map[string]interface{}{"$eq": 10}}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{"person.age": 5}}, nil)

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
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{"person.age": 10}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{"person.age": map[string]interface{}{
						"$neq": 10,
					}}}, nil)

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
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"person.age": map[string]interface{}{
							"$gte": 10,
						},
					}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"person.age": map[string]interface{}{
							"$gt": 10,
						},
					}}, nil)

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
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"person.age": map[string]interface{}{
							"$lt": 10,
						},
					}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"person.age": map[string]interface{}{
							"$lte": 10,
						},
					}}, nil)

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
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"person.age": map[string]interface{}{
							"$gt": 10,
						},
					}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"person.age": map[string]interface{}{
							"$in": []interface{}{float64(10), float64(1)},
						},
					}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"person.age": map[string]interface{}{
							"$gt": 10,
						},
					}}, nil)

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
									"$gt": 10,
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
					UID: "1234",
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
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"event.action": map[string]interface{}{
							"$nin": []interface{}{"update", "delete"},
						},
					}}, nil)

				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Body: map[string]interface{}{
						"event.action": map[string]interface{}{
							"$nin": []interface{}{"read", "delete"},
						},
					}}, nil)

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
		{
			name:  "Query Filter",
			query: "event_type=push&ref=main",
			payload: map[string]interface{}{
				"person": map[string]interface{}{"age": 10},
			},
			dbFn: func(args *testArgs) {
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Query: datastore.M{"event_type": "push"}}, nil)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Query: datastore.M{"event_type": "merge_request"}}, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{{UID: "123"}, {UID: "1234"}},
			wantSubs:  []datastore.Subscription{{UID: "123"}},
		},
		{
			name: "Path Filter",
			path: "/ingest/source-id",
			payload: map[string]interface{}{
				"person": map[string]interface{}{"age": 10},
			},
			dbFn: func(args *testArgs) {
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Path: datastore.M{"path": "/ingest/source-id"}}, nil)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(&datastore.EventTypeFilter{Path: datastore.M{"path": "/ingest/other"}}, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{{UID: "123"}, {UID: "1234"}},
			wantSubs:  []datastore.Subscription{{UID: "123"}},
		},
		{
			name:      "Wildcard filter matches when no exact event filter exists",
			eventType: "invoice.created",
			payload: map[string]interface{}{
				"kind": "allowed",
			},
			dbFn: func(args *testArgs) {
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				gomock.InOrder(
					fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "invoice.created").
						Return(nil, datastore.ErrFilterNotFound),
					fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "*").
						Return(&datastore.EventTypeFilter{Body: datastore.M{"kind": "allowed"}}, nil),
				)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{{UID: "123"}},
			wantSubs:  []datastore.Subscription{{UID: "123"}},
		},
		{
			name:      "Subscription matches when no filter exists",
			eventType: "invoice.created",
			payload: map[string]interface{}{
				"kind": "allowed",
			},
			dbFn: func(args *testArgs) {
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				gomock.InOrder(
					fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "invoice.created").
						Return(nil, datastore.ErrFilterNotFound),
					fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "*").
						Return(nil, datastore.ErrFilterNotFound),
				)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{{UID: "123"}},
			wantSubs:  []datastore.Subscription{{UID: "123"}},
		},
		{
			name:      "Disabled exact event filter falls back to wildcard filter",
			eventType: "invoice.created",
			payload: map[string]interface{}{
				"kind": "allowed",
			},
			dbFn: func(args *testArgs) {
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				gomock.InOrder(
					fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "invoice.created").
						Return(&datastore.EventTypeFilter{Body: datastore.M{"kind": "blocked"}, EnabledAtSet: true}, nil),
					fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "*").
						Return(&datastore.EventTypeFilter{Body: datastore.M{"kind": "allowed"}}, nil),
				)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{{UID: "123"}},
			wantSubs:  []datastore.Subscription{{UID: "123"}},
		},
		{
			name:      "Query filter wildcard selectors fail closed",
			eventType: "invoice.created",
			payload: map[string]interface{}{
				"kind": "allowed",
			},
			query: "items999999999=value",
			dbFn: func(args *testArgs) {
				fe, _ := args.filterRepo.(*mocks.MockFilterRepository)
				fe.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "invoice.created").
					Return(&datastore.EventTypeFilter{Query: datastore.M{"items.$.id": "value"}}, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
			},
			inputSubs: []datastore.Subscription{{UID: "123"}},
			wantSubs:  []datastore.Subscription{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			args := provideArgs(ctrl)
			subRepo, _ := args.subRepo.(*mocks.MockSubscriptionRepository)
			subRepo.EXPECT().
				CompareFlattenedPayload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				AnyTimes().
				DoAndReturn(func(_ context.Context, payload, filter flatten.M, _ bool) (bool, error) {
					return compare.Compare(payload, filter)
				})

			if tt.dbFn != nil {
				tt.dbFn(args)
			}

			payload, err := json.Marshal(tt.payload)
			require.NoError(t, err)

			event := &datastore.Event{
				Data:           payload,
				EventType:      datastore.EventType(tt.eventType),
				URLQueryParams: tt.query,
				URLPath:        tt.path,
			}

			subs, err := matchSubscriptionsUsingFilter(context.Background(), event, args.subRepo, args.filterRepo, args.licenser, tt.inputSubs, false, logger.New("test", logger.LevelInfo))
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

func TestCompareFilterScopeFlattensPayloadBeforeComparison(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	payload := datastore.M{
		"meta": map[string]interface{}{"version": "1"},
	}
	filter := datastore.M{"meta.version": "1"}

	subRepo.EXPECT().
		CompareFlattenedPayload(gomock.Any(), datastore.M{"meta.version": "1"}, filter, true).
		Return(true, nil)

	matched, err := compareFilterScope(context.Background(), subRepo, payload, filter)

	require.NoError(t, err)
	require.True(t, matched)
}

func TestMatchSubscriptions_DisabledFiltersAreIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	args := provideArgs(ctrl)
	filterRepo, _ := args.filterRepo.(*mocks.MockFilterRepository)

	gomock.InOrder(
		filterRepo.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "invoice.created").
			Return(&datastore.EventTypeFilter{EventType: "invoice.created", EnabledAtSet: true}, nil),
		filterRepo.EXPECT().FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "*").
			Return(&datastore.EventTypeFilter{EventType: "*"}, nil),
	)

	subs, err := matchSubscriptions(context.Background(), "invoice.created", []datastore.Subscription{{UID: "123"}}, filterRepo)

	require.NoError(t, err)
	require.Equal(t, []datastore.Subscription{{UID: "123"}}, subs)
}

func TestMatchSubscriptions_DisabledWildcardFilterIsIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	args := provideArgs(ctrl)
	filterRepo, _ := args.filterRepo.(*mocks.MockFilterRepository)

	filterRepo.EXPECT().
		FindFilterBySubscriptionAndEventType(gomock.Any(), "123", "*").
		Return(&datastore.EventTypeFilter{EventType: "*", EnabledAtSet: true}, nil)

	subs, err := matchSubscriptions(context.Background(), "*", []datastore.Subscription{{UID: "123"}}, filterRepo)

	require.NoError(t, err)
	require.Empty(t, subs)
}
