package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideBatchReplayEventService(ctrl *gomock.Controller, f *datastore.Filter) *BatchReplayEventService {
	return &BatchReplayEventService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Queue:        mocks.NewMockQueuer(ctrl),
		EventRepo:    mocks.NewMockEventRepository(ctrl),
		Filter:       f,
		Logger:       mocks.NewMockLogger(ctrl),
	}
}

func TestNormalizeBatchReplayPageable(t *testing.T) {
	t.Run("defaults empty pageable", func(t *testing.T) {
		got := NormalizeBatchReplayPageable(datastore.Pageable{})
		require.Equal(t, BatchReplayPageSize, got.PerPage)
		require.Equal(t, datastore.Next, got.Direction)
		require.NotEmpty(t, got.NextCursor)
	})

	t.Run("caps oversized pageable", func(t *testing.T) {
		got := NormalizeBatchReplayPageable(datastore.Pageable{PerPage: 2000000000})
		require.Equal(t, BatchReplayPageSize, got.PerPage)
	})

	t.Run("coerces invalid direction", func(t *testing.T) {
		got := NormalizeBatchReplayPageable(datastore.Pageable{Direction: "invalid"})
		require.Equal(t, datastore.Next, got.Direction)
	})

	t.Run("resets list view pagination from dashboard batch replay", func(t *testing.T) {
		got := NormalizeBatchReplayPageable(datastore.Pageable{
			PerPage:    20,
			Sort:       "DESC",
			Direction:  datastore.Next,
			NextCursor: "01J5XKQWZ8YN3M4P2R6T9V1C7D",
		})
		
		require.Equal(t, BatchReplayPageSize, got.PerPage)
		require.Equal(t, datastore.Next, got.Direction)
		require.Equal(t, "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF", got.NextCursor)
		require.Empty(t, got.PrevCursor)
		require.Equal(t, "DESC", got.Sort)
	})
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
						{UID: "event1", ProjectID: "proj0"},
						{UID: "event2", ProjectID: "proj1"},
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
						{UID: "event1", ProjectID: "proj0"},
						{UID: "event2", ProjectID: "proj1"},
						{UID: "event3", ProjectID: "proj2"},
					},
					datastore.PaginationData{},
					nil,
				)

				q, _ := br.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(2).Return(nil)
				q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(errors.New("failed"))

				ml, _ := br.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "replay_event: failed to write event to the queue", "error", gomock.Any()).Times(1)
				ml.EXPECT().ErrorContext(gomock.Any(), "an item in the batch replay failed", "error", gomock.Any()).Times(1)
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
			name: "should_paginate_through_all_events",
			dbFn: func(br *BatchReplayEventService) {
				e, _ := br.EventRepo.(*mocks.MockEventRepository)
				gomock.InOrder(
					e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
						[]datastore.Event{{UID: "event1", ProjectID: "proj0"}},
						datastore.PaginationData{HasNextPage: true, NextPageCursor: "cursor-2"},
						nil,
					),
					e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
						[]datastore.Event{{UID: "event2", ProjectID: "proj0"}},
						datastore.PaginationData{},
						nil,
					),
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

				ml, _ := br.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to fetch events", "error", gomock.Any(), "successes", 0, "failures", 0).Times(1)
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
		{
			name: "should_return_partial_progress_when_later_page_fetch_fails",
			dbFn: func(br *BatchReplayEventService) {
				e, _ := br.EventRepo.(*mocks.MockEventRepository)
				gomock.InOrder(
					e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
						[]datastore.Event{{UID: "event1", ProjectID: "proj0"}},
						datastore.PaginationData{HasNextPage: true, NextPageCursor: "cursor-2"},
						nil,
					),
					e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
						[]datastore.Event{},
						datastore.PaginationData{},
						errors.New("failed"),
					),
				)

				q, _ := br.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(nil)

				ml, _ := br.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to fetch events", "error", gomock.Any(), "successes", 1, "failures", 0).Times(1)
			},
			args: args{
				ctx: ctx,
				f: &datastore.Filter{
					Project: &datastore.Project{UID: "1234"},
				},
			},
			wantSuccesses: 1,
			wantFailures:  0,
			wantErr:       true,
			wantErrMsg:    "batch replay incomplete after 1 successful and 0 failed replays",
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
				if tt.wantSuccesses > 0 || tt.wantFailures > 0 {
					require.Equal(t, tt.wantSuccesses, successes)
					require.Equal(t, tt.wantFailures, failures)
				}
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantSuccesses, successes)
			require.Equal(t, tt.wantFailures, failures)
		})
	}
}
