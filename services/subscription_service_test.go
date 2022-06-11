package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideSubsctiptionService(ctrl *gomock.Controller) *SubcriptionService {
	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	appRepo := mocks.NewMockApplicationRepository(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	return NewSubscriptionService(subRepo, appRepo, sourceRepo)
}

func TestSubscription_CreateSubscription(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx             context.Context
		group           *datastore.Group
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
			name: "should create subscription",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					Type:       "incoming",
					AppID:      "app-id-1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				group: &datastore.Group{UID: "12345"},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       "incoming",
				AppID:      "app-id-1",
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
		},
		{
			name: "should fail to create subscription",
			args: args{
				ctx: ctx,
				newSubscription: &models.Subscription{
					Name:       "sub 1",
					Type:       "incoming",
					AppID:      "app-id-1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				group: &datastore.Group{
					UID: "12345",
				},
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create subscription",
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

			subscription, err := ss.CreateSubscription(tc.args.ctx, tc.args.group.UID, tc.args.newSubscription)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.NotEmpty(t, subscription.UID)

			require.Equal(t, subscription.Name, tc.wantSubscription.Name)
			require.Equal(t, subscription.Type, tc.wantSubscription.Type)
		})
	}
}

func TestSubscription_UpdateSubscription(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx            context.Context
		group          *datastore.Group
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
					AppID:      "app-id-1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				group: &datastore.Group{UID: "12345"},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       "incoming",
				AppID:      "app-id-1",
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *SubcriptionService) {
				s, _ := ss.subRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{
					UID:  "sub-uid-1",
					Type: "incoming",
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
					AppID:      "app-id-1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				group: &datastore.Group{
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

			subscription, err := ss.UpdateSubscription(tc.args.ctx, tc.args.group.UID, tc.args.subscriptionId, tc.args.update)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
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
				{
					UID: "123",
					Source: &datastore.Source{
						UID:  "123",
						Name: "some name",
						Type: datastore.HTTPSource,
						Verifier: &datastore.VerifierConfig{
							Type: datastore.APIKeyVerifier,
							ApiKey: &datastore.ApiKey{
								APIKey:       "123",
								APIKeyHeader: "header",
							},
						},
						GroupID:    "123",
						MaskID:     "mask",
						IsDisabled: false,
					},
					App: &datastore.Application{
						UID:          "abc",
						Title:        "Title",
						GroupID:      "123",
						SupportEmail: "SupportEmail",
					},
					Endpoint: &datastore.Endpoint{
						UID:               "1234",
						TargetURL:         "http://localhost.com",
						DocumentStatus:    "Active",
						Secret:            "Secret",
						HttpTimeout:       "30s",
						RateLimit:         10,
						RateLimitDuration: "1h",
					},
				},
				{
					UID: "123456",
					Source: &datastore.Source{
						UID:  "123",
						Name: "some name",
						Type: datastore.HTTPSource,
						Verifier: &datastore.VerifierConfig{
							Type: datastore.APIKeyVerifier,
							ApiKey: &datastore.ApiKey{
								APIKey:       "123",
								APIKeyHeader: "header",
							},
						},
						GroupID:    "123",
						MaskID:     "mask",
						IsDisabled: false,
					},
					App: &datastore.Application{
						UID:          "abc",
						Title:        "Title",
						GroupID:      "123",
						SupportEmail: "SupportEmail",
					},
					Endpoint: &datastore.Endpoint{
						UID:               "1234",
						TargetURL:         "http://localhost.com",
						DocumentStatus:    "Active",
						Secret:            "Secret",
						HttpTimeout:       "30s",
						RateLimit:         10,
						RateLimitDuration: "1h",
					},
				},
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

				ap, _ := ss.appRepo.(*mocks.MockApplicationRepository)
				ap.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Return(&datastore.Application{
					UID:          "abc",
					Title:        "Title",
					GroupID:      "123",
					SupportEmail: "SupportEmail",
				}, nil).Times(1)

				ev, _ := ss.sourceRepo.(*mocks.MockSourceRepository)
				ev.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&datastore.Source{
					UID:  "123",
					Name: "some name",
					Type: datastore.HTTPSource,
					Verifier: &datastore.VerifierConfig{
						Type: datastore.APIKeyVerifier,
						ApiKey: &datastore.ApiKey{
							APIKey:       "123",
							APIKeyHeader: "header",
						},
					},
					GroupID:    "123",
					MaskID:     "mask",
					IsDisabled: false,
				}, nil).Times(1)

				en, _ := ss.appRepo.(*mocks.MockApplicationRepository)
				en.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(&datastore.Endpoint{
					UID:               "1234",
					TargetURL:         "http://localhost.com",
					DocumentStatus:    "Active",
					Secret:            "Secret",
					HttpTimeout:       "30s",
					RateLimit:         10,
					RateLimitDuration: "1h",
				}, nil).Times(1)
			},
		},
		{
			name: "should_fail_load_sources",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "123"},
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
				group: &datastore.Group{UID: "123"},
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

func TestSubscription_DeleteSubscription(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx             context.Context
		group           *datastore.Group
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
					Type:       "incoming",
					AppID:      "app-id-1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				group: &datastore.Group{UID: "12345"},
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
					Type:       "incoming",
					AppID:      "app-id-1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				group: &datastore.Group{
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

			err := ss.DeleteSubscription(tc.args.ctx, tc.args.group.UID, tc.args.newSubscription)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}
			require.Nil(t, err)
		})
	}
}
