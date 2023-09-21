package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func provideSearchEventService(ctrl *gomock.Controller, f *datastore.Filter) *SearchEventService {
	return &SearchEventService{
		EventRepo: mocks.NewMockEventRepository(ctrl),
		Searcher:  mocks.NewMockSearcher(ctrl),
		Filter:    f,
	}
}

func TestSearchEventService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx    context.Context
		filter *datastore.Filter
	}
	tests := []struct {
		name               string
		args               args
		dbFn               func(es *SearchEventService)
		wantEvents         []datastore.Event
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrMsg         string
	}{
		{
			name: "should_get_event_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:    &datastore.Project{UID: "123"},
					EndpointID: "abc",
					Pageable: datastore.Pageable{
						PerPage:    10,
						Direction:  datastore.Next,
						NextCursor: datastore.DefaultCursor,
					},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *SearchEventService) {
				se, _ := es.Searcher.(*mocks.MockSearcher)
				se.EXPECT().Search(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]string{"1234"}, datastore.PaginationData{
						PerPage: 2,
					}, nil)

				ed, _ := es.EventRepo.(*mocks.MockEventRepository)
				ed.EXPECT().FindEventsByIDs(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return([]datastore.Event{{UID: "1234"}}, nil)
			},
			wantEvents: []datastore.Event{
				{UID: "1234"},
			},
			wantPaginationData: datastore.PaginationData{
				PerPage: 2,
			},
		},
		{
			name: "should_fail_to_get_events_paged",
			args: args{
				ctx: ctx,
				filter: &datastore.Filter{
					Project:    &datastore.Project{UID: "123"},
					EndpointID: "abc",
					EventID:    "ref",
					Status:     []datastore.EventDeliveryStatus{datastore.SuccessEventStatus, datastore.ScheduledEventStatus},
					SearchParams: datastore.SearchParams{
						CreatedAtStart: 13323,
						CreatedAtEnd:   1213,
					},
				},
			},
			dbFn: func(es *SearchEventService) {
				ed, _ := es.Searcher.(*mocks.MockSearcher)
				ed.EXPECT().
					Search(gomock.Any(), gomock.Any()).
					Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideSearchEventService(ctrl, tc.args.filter)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			events, paginationData, err := es.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantEvents, events)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}
