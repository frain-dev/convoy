package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideSubsctiptionService(ctrl *gomock.Controller) *SubcriptionService {
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	return NewSubscriptionService(subRepo)
}

func TestSubscription_LoadSubscriptionsPaged(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx      context.Context
		group    *datastore.Group
		pageable datastore.Pageable
	}

	tests := []struct {
		name               string
		args               args
		dbFn               func(so *SubcriptionService)
		wantSubscription   []datastore.Subscription
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_load_subscriptions",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			wantSubscription: []datastore.Subscription{
				{UID: "123"},
				{UID: "123456"},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 3,
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().
					LoadSubscriptionsPaged(gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]datastore.Subscription{
						{UID: "123"},
						{UID: "123456"},
					}, datastore.PaginationData{
						Total:     2,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      2,
						TotalPage: 3,
					}, nil)
			},
		},
		{
			name: "should_fail_load_sources",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			dbFn: func(so *SubcriptionService) {
				s, _ := so.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().
					LoadSubscriptionsPaged(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.PaginationData{}, errors.New("failed"))

			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching subscriptions",
		},
		{
			name: "should_load_sources_empty_list",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			wantSubscription: []datastore.Subscription{},
			wantPaginationData: datastore.PaginationData{
				Total:     0,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 0,
			},
			dbFn: func(so *SubcriptionService) {
				s, _ := so.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().
					LoadSubscriptionsPaged(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Subscription{},
						datastore.PaginationData{
							Total:     0,
							Page:      1,
							PerPage:   10,
							Prev:      0,
							Next:      2,
							TotalPage: 0,
						}, nil)

			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ss := provideSubsctiptionService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			subscriptions, paginationData, err := ss.LoadSubscriptionsPaged(tc.args.ctx, tc.args.group.UID, tc.args.pageable)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.wantSubscription, subscriptions)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}
