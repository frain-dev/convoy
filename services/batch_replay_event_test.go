package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
)

func provideBatchReplayEventService(ctrl *gomock.Controller, f *datastore.Filter) *BatchReplayEventService {
	return &BatchReplayEventService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Queue:        mocks.NewMockQueuer(ctrl),
		EventRepo:    mocks.NewMockEventRepository(ctrl),
		Filter:       f,
	}
}

func TestBatchReplayEventService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		f   *datastore.Filter
	}
	tests := []struct {
		name          string
		dbFn          func(br *BatchReplayEventService)
		args          args
		wantSuccesses int
		wantFailures  int
		wantErr       bool
		wantErrMsg    string
	}{
		{
			name: "should_batch_replay_events",
			dbFn: func(br *BatchReplayEventService) {
				e, _ := br.EventRepo.(*mocks.MockEventRepository)
				e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
					[]datastore.Event{
						{UID: "event1"},
						{UID: "event2"},
					},
					datastore.PaginationData{},
					nil,
				)

				q, _ := br.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(2).Return(nil)
			},
			args: args{
				ctx: ctx,
				f: &datastore.Filter{
					Project: &datastore.Project{UID: "1234"},
				},
			},
			wantSuccesses: 2,
			wantFailures:  0,
			wantErr:       false,
			wantErrMsg:    "",
		},
		{
			name: "should_batch_replay_one_event",
			dbFn: func(br *BatchReplayEventService) {
				e, _ := br.EventRepo.(*mocks.MockEventRepository)
				e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
					[]datastore.Event{
						{UID: "event1"},
						{UID: "event2"},
						{UID: "event3"},
					},
					datastore.PaginationData{},
					nil,
				)

				q, _ := br.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(2).Return(nil)
				q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			args: args{
				ctx: ctx,
				f: &datastore.Filter{
					Project: &datastore.Project{UID: "1234"},
				},
			},
			wantSuccesses: 2,
			wantFailures:  1,
			wantErr:       false,
			wantErrMsg:    "",
		},
		{
			name: "should_fail_to_load_events",
			dbFn: func(br *BatchReplayEventService) {
				e, _ := br.EventRepo.(*mocks.MockEventRepository)
				e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
					[]datastore.Event{},
					datastore.PaginationData{},
					errors.New("failed"),
				)
			},
			args: args{
				ctx: ctx,
				f: &datastore.Filter{
					Project: &datastore.Project{UID: "1234"},
				},
			},
			wantErr:    true,
			wantErrMsg: "failed to fetch event deliveries",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			e := provideBatchReplayEventService(ctrl, tt.args.f)

			if tt.dbFn != nil {
				tt.dbFn(e)
			}

			successes, failures, err := e.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantSuccesses, successes)
			require.Equal(t, tt.wantFailures, failures)
		})
	}
}
