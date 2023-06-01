package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func provideReplayEventService(ctrl *gomock.Controller, event *datastore.Event) *ReplayEventService {
	return &ReplayEventService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Queue:        mocks.NewMockQueuer(ctrl),
		Event:        event,
	}
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
				event: &datastore.Event{UID: "123"},
				g:     &datastore.Project{UID: "123", Name: "test_project"},
			},
			dbFn: func(es *ReplayEventService) {
				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, gomock.Any(), gomock.Any()).
					Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_replay_app_event",
			args: args{
				ctx:   ctx,
				event: &datastore.Event{UID: "123"},
				g:     &datastore.Project{UID: "123", Name: "test_project"},
			},
			dbFn: func(es *ReplayEventService) {
				eq, _ := es.Queue.(*mocks.MockQueuer)
				eq.EXPECT().Write(convoy.CreateEventProcessor, gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
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
