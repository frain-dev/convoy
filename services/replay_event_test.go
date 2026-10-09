package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/worker/task"
)

func provideReplayEventService(ctrl *gomock.Controller, event *datastore.Event) *ReplayEventService {
	return &ReplayEventService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Queue:        mocks.NewMockQueuer(ctrl),
		Logger:       mocks.NewMockLogger(ctrl),
		Event:        event,
	}
}

func TestReplayEventService_RunCreatesDistinctEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	original := &datastore.Event{
		UID: "original", ProjectID: "project", IdempotencyKey: "original-key",
		IsDuplicateEvent: true, FailureReason: "old failure", EventType: "created",
	}
	service := provideReplayEventService(ctrl, original)
	q := service.Queue.(*mocks.MockQueuer)
	seen := map[string]bool{}
	q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).DoAndReturn(
		func(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
			var payload task.CreateEvent
			require.NoError(t, msgpack.DecodeMsgPack(job.Payload, &payload))
			require.NotNil(t, payload.Event)
			require.NotEqual(t, original.UID, payload.Event.UID)
			require.False(t, seen[payload.Event.UID], "each replay must have a fresh ID")
			seen[payload.Event.UID] = true
			require.Equal(t, original.ProjectID, payload.Event.ProjectID)
			require.Equal(t, original.EventType, payload.Event.EventType)
			require.Empty(t, payload.Event.IdempotencyKey)
			require.False(t, payload.Event.IsDuplicateEvent)
			require.Empty(t, payload.Event.FailureReason)
			require.True(t, payload.Event.AcknowledgedAt.Valid)
			return nil
		},
	).Times(2)
	require.NoError(t, service.Run(context.Background()))
	require.NoError(t, service.Run(context.Background()))
	require.Len(t, seen, 2)
	require.Equal(t, "original", original.UID)
	require.Equal(t, "original-key", original.IdempotencyKey)
	require.True(t, original.IsDuplicateEvent)
	require.Equal(t, "old failure", original.FailureReason)
}

func TestReplayEventService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx   context.Context
		event *datastore.Event
		g     *datastore.Project
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(es *ReplayEventService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_replay_app_event",
			args: args{
				ctx:   ctx,
				event: &datastore.Event{UID: "123", ProjectID: "proj1"},
				g:     &datastore.Project{UID: "123", Name: "test_project"},
			},
			dbFn: func(es *ReplayEventService) {
				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_replay_app_event",
			args: args{
				ctx:   ctx,
				event: &datastore.Event{UID: "123", ProjectID: "proj1"},
				g:     &datastore.Project{UID: "123", Name: "test_project"},
			},
			dbFn: func(es *ReplayEventService) {
				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))

				ml, _ := es.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "replay_event: failed to write event to the queue", "error", gomock.Any()).Times(1)
			},
			wantErr:    true,
			wantErrMsg: "failed to write event to queue",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideReplayEventService(ctrl, tc.args.event)
			require.NoError(t, err)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
