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

	t.Run("ignores caller prev direction and cursors", func(t *testing.T) {
		got := NormalizeBatchReplayPageable(datastore.Pageable{
			PerPage:    20,
			Direction:  datastore.Prev,
			NextCursor: "01J5XKQWZ8YN3M4P2R6T9V1C7D",
			PrevCursor: "01J5XKQWZ8YN3M4P2R6T9V1C7E",
		})
		require.Equal(t, BatchReplayPageSize, got.PerPage)
		require.Equal(t, datastore.Next, got.Direction)
		require.Equal(t, "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF", got.NextCursor)
		require.Empty(t, got.PrevCursor)
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
			name: "should_ignore_caller_cursors_and_paginate_internally",
			dbFn: func(br *BatchReplayEventService) {
				e, _ := br.EventRepo.(*mocks.MockEventRepository)
				gomock.InOrder(
					e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Cond(func(x any) bool {
						f, ok := x.(*datastore.Filter)
						return ok &&
							f.Pageable.PerPage == BatchReplayPageSize &&
							f.Pageable.Direction == datastore.Next &&
							f.Pageable.NextCursor == "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF" &&
							f.Pageable.PrevCursor == ""
					})).Times(1).Return(
						[]datastore.Event{{UID: "event1", ProjectID: "proj0"}},
						datastore.PaginationData{HasNextPage: true, NextPageCursor: "cursor-2"},
						nil,
					),
					e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Cond(func(x any) bool {
						f, ok := x.(*datastore.Filter)
						return ok &&
							f.Pageable.PerPage == BatchReplayPageSize &&
							f.Pageable.Direction == datastore.Next &&
							f.Pageable.NextCursor == "cursor-2"
					})).Times(1).Return(
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
					Pageable: datastore.Pageable{
						PerPage:    20,
						Direction:  datastore.Prev,
						NextCursor: "01J5XKQWZ8YN3M4P2R6T9V1C7D",
						PrevCursor: "01J5XKQWZ8YN3M4P2R6T9V1C7E",
					},
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
			name: "should_replay_held_page_when_next_page_fetch_fails",
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
		{
			name: "should_replay_held_page_when_a_later_fetch_fails",
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
						datastore.PaginationData{HasNextPage: true, NextPageCursor: "cursor-3"},
						nil,
					),
					e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
						[]datastore.Event{},
						datastore.PaginationData{},
						errors.New("failed"),
					),
				)

				q, _ := br.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(2).Return(nil)

				ml, _ := br.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to fetch events", "error", gomock.Any(), "successes", 2, "failures", 0).Times(1)
			},
			args: args{
				ctx: ctx,
				f: &datastore.Filter{
					Project: &datastore.Project{UID: "1234"},
				},
			},
			wantSuccesses: 2,
			wantFailures:  0,
			wantErr:       true,
			wantErrMsg:    "batch replay incomplete after 2 successful and 0 failed replays",
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

func TestBatchReplayEventService_OwnedEndpointIDs(t *testing.T) {
	ctx := context.Background()
	filter := &datastore.Filter{
		Project:     &datastore.Project{UID: "1234"},
		EndpointIDs: []string{"ep-a"},
	}

	t.Run("replays_multi_endpoint_event_when_all_targets_are_owned", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		br := provideBatchReplayEventService(ctrl, filter)
		br.OwnedEndpointIDs = []string{"ep-a", "ep-b"}

		e, _ := br.EventRepo.(*mocks.MockEventRepository)
		e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
			[]datastore.Event{
				{UID: "event1", ProjectID: "proj0", Endpoints: []string{"ep-a", "ep-b"}},
			},
			datastore.PaginationData{},
			nil,
		)

		q, _ := br.Queue.(*mocks.MockQueuer)
		q.EXPECT().Write(gomock.Any(), convoy.CreateEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(nil)

		successes, failures, err := br.Run(ctx)
		require.Nil(t, err)
		require.Equal(t, 1, successes)
		require.Equal(t, 0, failures)
	})

	t.Run("skips_multi_endpoint_event_when_ownership_is_only_the_filter", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		br := provideBatchReplayEventService(ctrl, filter)
		// Regression: OwnedEndpointIDs must be the portal allowlist, not the narrowed filter.
		br.OwnedEndpointIDs = []string{"ep-a"}

		e, _ := br.EventRepo.(*mocks.MockEventRepository)
		e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
			[]datastore.Event{
				{UID: "event1", ProjectID: "proj0", Endpoints: []string{"ep-a", "ep-b"}},
			},
			datastore.PaginationData{},
			nil,
		)

		ml, _ := br.Logger.(*mocks.MockLogger)
		ml.EXPECT().WarnContext(gomock.Any(), "batch replay skipped event not fully owned by caller", "event_id", "event1").Times(1)

		successes, failures, err := br.Run(ctx)
		require.Nil(t, err)
		require.Equal(t, 0, successes)
		require.Equal(t, 1, failures)
	})

	t.Run("keeps_zero_successes_when_ownership_skips_then_later_page_fetch_fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		br := provideBatchReplayEventService(ctrl, filter)
		br.OwnedEndpointIDs = []string{"ep-a"}

		e, _ := br.EventRepo.(*mocks.MockEventRepository)
		gomock.InOrder(
			e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
				[]datastore.Event{
					{UID: "event1", ProjectID: "proj0", Endpoints: []string{"ep-a", "ep-b"}},
				},
				datastore.PaginationData{HasNextPage: true, NextPageCursor: "cursor-2"},
				nil,
			),
			e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
				[]datastore.Event{
					{UID: "event2", ProjectID: "proj0", Endpoints: []string{"ep-a", "ep-b"}},
				},
				datastore.PaginationData{HasNextPage: true, NextPageCursor: "cursor-3"},
				nil,
			),
			e.EXPECT().LoadEventsPaged(gomock.Any(), "1234", gomock.Any()).Times(1).Return(
				[]datastore.Event{},
				datastore.PaginationData{},
				errors.New("failed"),
			),
		)

		ml, _ := br.Logger.(*mocks.MockLogger)
		ml.EXPECT().WarnContext(gomock.Any(), "batch replay skipped event not fully owned by caller", "event_id", "event1").Times(1)
		ml.EXPECT().WarnContext(gomock.Any(), "batch replay skipped event not fully owned by caller", "event_id", "event2").Times(1)
		ml.EXPECT().ErrorContext(gomock.Any(), "failed to fetch events", "error", gomock.Any(), "successes", 0, "failures", 2).Times(1)

		successes, failures, err := br.Run(ctx)
		require.Error(t, err)
		require.Equal(t, "batch replay incomplete after 0 successful and 2 failed replays", err.(*ServiceError).Error())
		require.Equal(t, 0, successes)
		require.Equal(t, 2, failures)
	})
}
