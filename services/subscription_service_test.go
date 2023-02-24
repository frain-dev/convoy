package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideSubsctiptionService(ctrl *gomock.Controller) *SubcriptionService {
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	return NewSubscriptionService(subRepo, endpointRepo, sourceRepo)
}

func TestSubscription_CreateSubscription(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx             context.Context
		project         *datastore.Project
		newSubscription *models.Subscription
	}

	tests := []struct {
		name             string
		args             args
		wantSubscription *datastore.Subscription
		dbFn             func(so *SubcriptionService)
		wantErr          bool
		wantErrCode      int
		wantErrMsg       string
	}{
		{
			name: "should create subscription for outgoing project",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.OutgoingProject},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				a, _ := ss.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "12345",
					},
					nil,
				)
			},
		},
		{
			name: "should create subscription for incoming project",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.IncomingProject},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				a, _ := ss.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "12345",
					},
					nil,
				)

				sr, _ := ss.sourceRepo.(*mocks.MockSourceRepository)
				sr.EXPECT().FindSourceByID(gomock.Any(), "12345", "source-id-1").
					Times(1).Return(
					&datastore.Source{
						ProjectID: "12345",
						UID:       "abc",
					},
					nil,
				)
			},
		},
		{
			name: "should fail to find source",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.IncomingProject},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				a, _ := ss.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "12345",
					},
					nil,
				)

				sr, _ := ss.sourceRepo.(*mocks.MockSourceRepository)
				sr.EXPECT().FindSourceByID(gomock.Any(), "12345", "source-id-1").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find source by id",
		},
		{
			name: "should fail to find endpoint",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				a, _ := ss.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find endpoint by id",
		},
		{
			name: "should error for endpoint does not belong to project",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				a, _ := ss.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "abb",
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusUnauthorized,
			wantErrMsg:  "endpoint does not belong to project",
		},
		{
			name: "should error for failed to find endpoint",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				a, _ := ss.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(nil, errors.New("failed to find endpoint by id"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find endpoint by id",
		},
		{
			name: "should fail to create subscription",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{
					UID: "12345",
				},
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed"))

				a, _ := ss.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "12345",
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create subscription",
		},
		{
			name: "create subscription for outgoing project - should set default event types array",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.OutgoingProject},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
				FilterConfig: &datastore.FilterConfiguration{
					EventTypes: []string{"*"},
				},
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				a, _ := ss.endpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "12345",
					},
					nil,
				)
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

			subscription, err := ss.CreateSubscription(tc.args.ctx, tc.args.project, tc.args.newSubscription)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.NotEmpty(t, subscription.UID)

			require.Equal(t, subscription.Name, tc.wantSubscription.Name)
			require.Equal(t, subscription.Type, tc.wantSubscription.Type)

			if tc.wantSubscription.FilterConfig != nil {
				require.Equal(t, subscription.FilterConfig.EventTypes,
					tc.wantSubscription.FilterConfig.EventTypes)
			}
		})
	}
}

func TestSubscription_UpdateSubscription(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx            context.Context
		project        *datastore.Project
		subscriptionId string
		update         *models.UpdateSubscription
	}

	tests := []struct {
		name             string
		args             args
		wantSubscription *datastore.Subscription
		dbFn             func(so *SubcriptionService)
		wantErr          bool
		wantErrCode      int
		wantErrMsg       string
	}{
		{
			name: "should update subscription",
			args: args{
				ctx: ctx,
				update: &models.UpdateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{
					UID:  "sub-uid-1",
					Type: datastore.SubscriptionTypeAPI,
				}, nil)

				s.EXPECT().UpdateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
		},
		{
			name: "should fail to update subscription",
			args: args{
				ctx: ctx,
				update: &models.UpdateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{
					UID: "12345",
				},
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{}, nil)

				s.EXPECT().UpdateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to update subscription",
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

			subscription, err := ss.UpdateSubscription(tc.args.ctx, tc.args.project.UID, tc.args.subscriptionId, tc.args.update)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.NotEmpty(t, subscription.UID)

			require.Equal(t, subscription.Name, tc.wantSubscription.Name)
			require.Equal(t, subscription.Type, tc.wantSubscription.Type)
		})
	}
}

func TestSubscription_LoadSubscriptionsPaged(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx      context.Context
		filter   *datastore.FilterBy
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
				ctx:    ctx,
				filter: &datastore.FilterBy{ProjectID: "12345"},
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
					LoadSubscriptionsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
				ctx:    ctx,
				filter: &datastore.FilterBy{ProjectID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			dbFn: func(so *SubcriptionService) {
				s, _ := so.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().
					LoadSubscriptionsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching subscriptions",
		},
		{
			name: "should_load_sources_empty_list",
			args: args{
				ctx:    ctx,
				filter: &datastore.FilterBy{ProjectID: "12345"},
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
					LoadSubscriptionsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
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

			subscriptions, paginationData, err := ss.LoadSubscriptionsPaged(tc.args.ctx, tc.args.filter, tc.args.pageable)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.wantSubscription, subscriptions)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}

func TestSubscription_DeleteSubscription(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx             context.Context
		project         *datastore.Project
		newSubscription *datastore.Subscription
	}

	tests := []struct {
		name        string
		args        args
		dbFn        func(so *SubcriptionService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should delete subscription",
			args: args{
				ctx: ctx,
				newSubscription: &datastore.Subscription{
					Name:       "sub 1",
					Type:       datastore.SubscriptionTypeAPI,
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().DeleteSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
		},
		{
			name: "should fail to delete subscription",
			args: args{
				ctx: ctx,
				newSubscription: &datastore.Subscription{
					Name:       "sub 1",
					Type:       datastore.SubscriptionTypeAPI,
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{
					UID: "12345",
				},
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().DeleteSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete subscription",
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

			err := ss.DeleteSubscription(tc.args.ctx, tc.args.project.UID, tc.args.newSubscription)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}
			require.Nil(t, err)
		})
	}
}
