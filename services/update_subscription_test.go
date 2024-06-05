package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideUpdateSubscriptionService(ctrl *gomock.Controller, projectID string, subID string, update *models.UpdateSubscription) *UpdateSubscriptionService {
	return &UpdateSubscriptionService{
		SubRepo:        mocks.NewMockSubscriptionRepository(ctrl),
		EndpointRepo:   mocks.NewMockEndpointRepository(ctrl),
		SourceRepo:     mocks.NewMockSourceRepository(ctrl),
		ProjectId:      projectID,
		SubscriptionId: subID,
		Update:         update,
	}
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
