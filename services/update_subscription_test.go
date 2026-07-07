package services

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideUpdateSubscriptionService(ctrl *gomock.Controller, projectID string, subID string, update *models.UpdateSubscription) *UpdateSubscriptionService {
	return NewUpdateSubscriptionService(
		mocks.NewMockSubscriptionRepository(ctrl),
		mocks.NewMockEndpointRepository(ctrl),
		mocks.NewMockProjectRepository(ctrl),
		mocks.NewMockSourceRepository(ctrl),
		mocks.NewMockLicenser(ctrl),
		mocks.NewMockLogger(ctrl),
		projectID,
		subID,
		update,
	)
}

func TestUpdateSubscriptionService_Run(t *testing.T) {
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
		dbFn             func(so *UpdateSubscriptionService)
		wantErr          bool
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
			dbFn: func(ss *UpdateSubscriptionService) {
				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{
					UID:  "sub-uid-1",
					Type: datastore.SubscriptionTypeAPI,
				}, nil)

				s.EXPECT().UpdateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				ss.ProjectRepo.(*mocks.MockProjectRepository).EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						UID:  "12345",
						Type: datastore.OutgoingProject,
					}, nil)

				ss.EndpointRepo.(*mocks.MockEndpointRepository).EXPECT().
					FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						UID: "endpoint-id-1",
					}, nil)

				s.EXPECT().CountEndpointSubscriptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(0), nil)
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
			dbFn: func(ss *UpdateSubscriptionService) {
				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.Subscription{}, nil)

				s.EXPECT().UpdateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed"))

				ss.ProjectRepo.(*mocks.MockProjectRepository).EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						UID:  "12345",
						Type: datastore.OutgoingProject,
					}, nil)

				ss.EndpointRepo.(*mocks.MockEndpointRepository).EXPECT().
					FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						UID: "endpoint-id-1",
					}, nil)

				s.EXPECT().CountEndpointSubscriptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(int64(0), nil)

				ml, _ := ss.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to update subscription", "error", gomock.Any()).Times(1)
			},
			wantErr:    true,
			wantErrMsg: "failed to update subscription",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ss := provideUpdateSubscriptionService(ctrl, tc.args.project.UID, tc.args.subscriptionId, tc.args.update)

			if tc.dbFn != nil {
				tc.dbFn(ss)
			}

			subscription, err := ss.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
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

func TestUpdateSubscriptionService_PreservesQueryAndPathFilterConfig(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	update := &models.UpdateSubscription{
		FilterConfig: &models.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter: models.FS{
				Query: datastore.M{"event_type": "push"},
				Path:  datastore.M{"path": "/ingest/source-id"},
			},
		},
	}

	ss := provideUpdateSubscriptionService(ctrl, "12345", "sub-uid-1", update)

	ss.SubRepo.(*mocks.MockSubscriptionRepository).EXPECT().
		FindSubscriptionByID(gomock.Any(), "12345", "sub-uid-1").
		Return(&datastore.Subscription{
			UID: "sub-uid-1",
			FilterConfig: &datastore.FilterConfiguration{
				EventTypes: []string{"*"},
				Filter:     datastore.FilterSchema{},
			},
		}, nil)

	ss.ProjectRepo.(*mocks.MockProjectRepository).EXPECT().
		FetchProjectByID(gomock.Any(), "12345").
		Return(&datastore.Project{UID: "12345", Type: datastore.OutgoingProject}, nil)

	ss.Licenser.(*mocks.MockLicenser).EXPECT().AdvancedSubscriptions().Times(1).Return(true)

	ss.SubRepo.(*mocks.MockSubscriptionRepository).EXPECT().
		UpdateSubscription(gomock.Any(), "12345", gomock.Cond(func(x any) bool {
			sub := x.(*datastore.Subscription)
			return reflect.DeepEqual(sub.FilterConfig.Filter.Query, datastore.M{"event_type": "push"}) &&
				reflect.DeepEqual(sub.FilterConfig.Filter.Path, datastore.M{"path": "/ingest/source-id"}) &&
				reflect.DeepEqual(sub.FilterConfig.Filter.RawQuery, datastore.M{"event_type": "push"}) &&
				reflect.DeepEqual(sub.FilterConfig.Filter.RawPath, datastore.M{"path": "/ingest/source-id"})
		})).
		Return(nil)

	subscription, err := ss.Run(ctx)

	require.NoError(t, err)
	require.Equal(t, datastore.M{"event_type": "push"}, subscription.FilterConfig.Filter.Query)
	require.Equal(t, datastore.M{"path": "/ingest/source-id"}, subscription.FilterConfig.Filter.Path)
}
