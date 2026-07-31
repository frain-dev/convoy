package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/dynamiceventack"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

func provideCreateDynamicEventService(ctrl *gomock.Controller, de *models.DynamicEvent, project *datastore.Project) *CreateDynamicEventService {
	return &CreateDynamicEventService{
		Queue:        mocks.NewMockQueuer(ctrl),
		DynamicEvent: de,
		Project:      project,
	}
}

func redisOrSkip(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DialTimeout: 300 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestCreateDynamicEventService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx          context.Context
		dynamicEvent *models.DynamicEvent
		g            *datastore.Project
	}
	tests := []struct {
		name        string
		dbFn        func(es *CreateDynamicEventService)
		args        args
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_dynamic_event",
			dbFn: func(es *CreateDynamicEventService) {
				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.CreateDynamicEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				dynamicEvent: &models.DynamicEvent{
					URL:            "https://google.com",
					Secret:         "abc",
					EventTypes:     []string{"*"},
					Data:           []byte(`{"name":"daniel"}`),
					EventType:      "*",
					IdempotencyKey: "",
				},
				g: &datastore.Project{UID: "12345"},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_nil_project",
			dbFn: func(es *CreateDynamicEventService) {},
			args: args{
				ctx:          ctx,
				dynamicEvent: &models.DynamicEvent{},
				g:            nil,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while creating dynamic event - invalid project",
		},
		{
			name: "should_fail_closed_when_sync_ack_enabled_without_redis",
			dbFn: func(es *CreateDynamicEventService) {
				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.CreateDynamicEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				dynamicEvent: &models.DynamicEvent{
					URL:       "https://example.com/hook",
					Data:      []byte(`{}`),
					EventType: "*",
				},
				g: &datastore.Project{
					UID: "sync-nil-redis",
					Config: &datastore.ProjectConfig{
						SyncDynamicEventAck: true,
					},
				},
			},
			wantErr:     true,
			wantErrCode: http.StatusServiceUnavailable,
			wantErrMsg:  dynamiceventack.ErrNilRedis.Error(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideCreateDynamicEventService(ctrl, tc.args.dynamicEvent, tc.args.g)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				if tc.wantErrCode != 0 {
					var se *util.ServiceError
					if ok := errorAsService(err, &se); ok {
						require.Equal(t, tc.wantErrCode, se.ErrCode())
					}
				}
				return
			}

			require.Nil(t, err)
		})
	}
}

func errorAsService(err error, dest **util.ServiceError) bool {
	se, ok := err.(*util.ServiceError)
	if !ok {
		return false
	}
	*dest = se
	return true
}

func TestCreateDynamicEventService_SyncAckWaitSuccess(t *testing.T) {
	rdb := redisOrSkip(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	require.NoError(t, config.LoadConfig("./testdata/basic-config.json"))

	project := &datastore.Project{
		UID: "sync-ok-project",
		Config: &datastore.ProjectConfig{
			SyncDynamicEventAck: true,
		},
	}
	de := &models.DynamicEvent{
		URL:       "https://example.com/ok",
		Data:      []byte(`{"a":1}`),
		EventType: "*",
	}
	es := provideCreateDynamicEventService(ctrl, de, project)
	es.Redis = rdb

	q, _ := es.Queue.(*mocks.MockQueuer)
	q.EXPECT().Write(gomock.Any(), convoy.CreateDynamicEventProcessor, convoy.CreateEventQueue, gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ convoy.TaskName, _ convoy.QueueName, _ *queue.Job) error {
			go func() {
				time.Sleep(50 * time.Millisecond)
				_ = dynamiceventack.Publish(context.Background(), rdb, project.UID, de.EventID, dynamiceventack.Result{OK: true})
			}()
			return nil
		})

	err := es.Run(context.Background())
	require.NoError(t, err)
}

func TestCreateDynamicEventService_SyncAckWaitResolveError(t *testing.T) {
	rdb := redisOrSkip(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	require.NoError(t, config.LoadConfig("./testdata/basic-config.json"))

	project := &datastore.Project{
		UID: "sync-err-project",
		Config: &datastore.ProjectConfig{
			SyncDynamicEventAck: true,
		},
	}
	de := &models.DynamicEvent{
		URL:       "https://example.com/err",
		Data:      []byte(`{}`),
		EventType: "*",
	}
	es := provideCreateDynamicEventService(ctrl, de, project)
	es.Redis = rdb

	q, _ := es.Queue.(*mocks.MockQueuer)
	q.EXPECT().Write(gomock.Any(), convoy.CreateDynamicEventProcessor, convoy.CreateEventQueue, gomock.Any()).
		DoAndReturn(func(ctx context.Context, _ convoy.TaskName, _ convoy.QueueName, _ *queue.Job) error {
			go func() {
				time.Sleep(50 * time.Millisecond)
				_ = dynamiceventack.Publish(context.Background(), rdb, project.UID, de.EventID, dynamiceventack.Result{
					OK:    false,
					Error: "dynamic URL does not match any configured endpoint URL template",
				})
			}()
			return nil
		})

	err := es.Run(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match")
	se, ok := err.(*util.ServiceError)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, se.ErrCode())
}

func TestCreateDynamicEventService_SyncAckWaitTimeout(t *testing.T) {
	rdb := redisOrSkip(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	require.NoError(t, config.LoadConfig("./testdata/basic-config.json"))
	cfg, err := config.Get()
	require.NoError(t, err)
	cfg.SyncDynamicEventAckTimeout = 1
	require.NoError(t, config.Override(&cfg))

	project := &datastore.Project{
		UID: "sync-timeout-project",
		Config: &datastore.ProjectConfig{
			SyncDynamicEventAck: true,
		},
	}
	de := &models.DynamicEvent{
		URL:       "https://example.com/timeout",
		Data:      []byte(`{}`),
		EventType: "*",
	}
	es := provideCreateDynamicEventService(ctrl, de, project)
	es.Redis = rdb

	q, _ := es.Queue.(*mocks.MockQueuer)
	q.EXPECT().Write(gomock.Any(), convoy.CreateDynamicEventProcessor, convoy.CreateEventQueue, gomock.Any()).Return(nil)

	err = es.Run(context.Background())
	require.Error(t, err)
	require.Equal(t, dynamiceventack.ErrTimeout.Error(), err.Error())
	se, ok := err.(*util.ServiceError)
	require.True(t, ok)
	require.Equal(t, http.StatusGatewayTimeout, se.ErrCode())
}
