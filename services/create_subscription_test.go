package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideCreateSubscriptionService(ctrl *gomock.Controller, project *datastore.Project, newSub *models.CreateSubscription) *CreateSubscriptionService {
	return &CreateSubscriptionService{
		SubRepo:         mocks.NewMockSubscriptionRepository(ctrl),
		EndpointRepo:    mocks.NewMockEndpointRepository(ctrl),
		SourceRepo:      mocks.NewMockSourceRepository(ctrl),
		Licenser:        mocks.NewMockLicenser(ctrl),
		Project:         project,
		NewSubscription: newSub,
	}
}

func TestCreateSubscriptionService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx             context.Context
		project         *datastore.Project
		newSubscription *models.CreateSubscription
	}

	tests := []struct {
		name             string
		args             args
		wantSubscription *datastore.Subscription
		dbFn             func(so *CreateSubscriptionService)
		wantErr          bool
		wantErrMsg       string
	}{
		{
			name: "should create subscription for outgoing project",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.OutgoingProject, Config: &datastore.ProjectConfig{MultipleEndpointSubscriptions: false}},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *CreateSubscriptionService) {
				licenser, _ := ss.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
				licenser.EXPECT().Transformations().Times(1).Return(true)

				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				s.EXPECT().CountEndpointSubscriptions(gomock.Any(), "12345", "endpoint-id-1").
					Times(1).
					Return(int64(0), nil)

				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						UID:       "endpoint-id-1",
						ProjectID: "12345",
					},
					nil,
				)
			},
		},
		{
			name: "should skip filter config & function fields for subscription for outgoing project",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
					Function:   "console.log",
					FilterConfig: &models.FilterConfiguration{
						EventTypes: []string{"invoice.created"},
						Filter: models.FS{
							Headers: datastore.M{"x-msg-type": "stream-data"},
							Body:    datastore.M{"offset": "1234"},
						},
					},
				},
				project: &datastore.Project{UID: "12345", Type: datastore.OutgoingProject, Config: &datastore.ProjectConfig{MultipleEndpointSubscriptions: false}},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *CreateSubscriptionService) {
				licenser, _ := ss.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(false)
				licenser.EXPECT().Transformations().Times(1).Return(false)

				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Cond(func(x any) bool {
					sub := x.(*datastore.Subscription)
					var uid string
					uid, sub.UID = sub.UID, ""
					sub.CreatedAt, sub.UpdatedAt = time.Time{}, time.Time{}

					c := &datastore.Subscription{
						Name:       "sub 1",
						SourceID:   "source-id-1",
						EndpointID: "endpoint-id-1",
						ProjectID:  "12345",
						Function:   null.String{},
						Type:       datastore.SubscriptionTypeAPI,
						FilterConfig: &datastore.FilterConfiguration{
							EventTypes: []string{"*"},
							Filter: datastore.FilterSchema{
								Headers: datastore.M{},
								Body:    datastore.M{},
							},
						},
					}

					ok := reflect.DeepEqual(sub, c)
					sub.UID = uid
					return ok
				})).Times(1).Return(nil)

				s.EXPECT().CountEndpointSubscriptions(gomock.Any(), "12345", "endpoint-id-1").
					Times(1).
					Return(int64(0), nil)

				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						UID:       "endpoint-id-1",
						ProjectID: "12345",
					},
					nil,
				)
			},
		},
		{
			name: "should fail to count endpoint subscriptions for outgoing project if multi endpoints for subscriptions is false",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.OutgoingProject, Config: &datastore.ProjectConfig{MultipleEndpointSubscriptions: false}},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *CreateSubscriptionService) {
				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CountEndpointSubscriptions(gomock.Any(), "12345", "endpoint-id-1").
					Times(1).
					Return(int64(0), errors.New("failed"))

				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						UID:       "endpoint-id-1",
						ProjectID: "12345",
					},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "failed to count endpoint subscriptions",
		},
		{
			name: "should count endpoint subscriptions for outgoing project if multi endpoints for subscriptions is true",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.OutgoingProject, Config: &datastore.ProjectConfig{MultipleEndpointSubscriptions: true}},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *CreateSubscriptionService) {
				licenser, _ := ss.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
				licenser.EXPECT().Transformations().Times(1).Return(true)

				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						UID:       "endpoint-id-1",
						ProjectID: "12345",
					},
					nil,
				)
			},
		},
		{
			name: "should error for endpoint already has a subscription",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.OutgoingProject, Config: &datastore.ProjectConfig{MultipleEndpointSubscriptions: false}},
			},
			wantSubscription: &datastore.Subscription{
				Name:       "sub 1",
				Type:       datastore.SubscriptionTypeAPI,
				SourceID:   "source-id-1",
				EndpointID: "endpoint-id-1",
			},
			dbFn: func(ss *CreateSubscriptionService) {
				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CountEndpointSubscriptions(gomock.Any(), "12345", "endpoint-id-1").
					Times(1).
					Return(int64(1), nil)

				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						UID:       "endpoint-id-1",
						ProjectID: "12345",
					},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "a subscription for this endpoint already exists",
		},
		{
			name: "should create subscription for incoming project",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
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
			dbFn: func(ss *CreateSubscriptionService) {
				licenser, _ := ss.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
				licenser.EXPECT().Transformations().Times(1).Return(true)

				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "12345",
					},
					nil,
				)

				sr, _ := ss.SourceRepo.(*mocks.MockSourceRepository)
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
				newSubscription: &models.CreateSubscription{
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
			dbFn: func(ss *CreateSubscriptionService) {
				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "12345",
					},
					nil,
				)

				sr, _ := ss.SourceRepo.(*mocks.MockSourceRepository)
				sr.EXPECT().FindSourceByID(gomock.Any(), "12345", "source-id-1").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to find source by id",
		},
		{
			name: "should fail to find endpoint",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
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
			dbFn: func(ss *CreateSubscriptionService) {
				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to find endpoint by id",
		},
		{
			name: "should error for endpoint does not belong to project",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
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
			dbFn: func(ss *CreateSubscriptionService) {
				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "abb",
					},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "endpoint does not belong to project",
		},
		{
			name: "should error for failed to find endpoint",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
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
			dbFn: func(ss *CreateSubscriptionService) {
				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(nil, errors.New("failed to find endpoint by id"))
			},
			wantErr:    true,
			wantErrMsg: "failed to find endpoint by id",
		},
		{
			name: "should fail to create subscription",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{
					UID: "12345",
				},
			},
			dbFn: func(ss *CreateSubscriptionService) {
				licenser, _ := ss.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
				licenser.EXPECT().Transformations().Times(1).Return(true)

				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(errors.New("failed"))

				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						ProjectID: "12345",
					},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "failed to create subscription",
		},
		{
			name: "create subscription for outgoing project - should set default event types array",
			args: args{
				ctx: ctx,
				newSubscription: &models.CreateSubscription{
					Name:       "sub 1",
					SourceID:   "source-id-1",
					EndpointID: "endpoint-id-1",
				},
				project: &datastore.Project{UID: "12345", Type: datastore.OutgoingProject, Config: &datastore.ProjectConfig{MultipleEndpointSubscriptions: false}},
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
			dbFn: func(ss *CreateSubscriptionService) {
				licenser, _ := ss.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().AdvancedSubscriptions().Times(1).Return(true)
				licenser.EXPECT().Transformations().Times(1).Return(true)

				s, _ := ss.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().CreateSubscription(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				s.EXPECT().CountEndpointSubscriptions(gomock.Any(), "12345", "endpoint-id-1").
					Times(1).
					Return(int64(0), nil)

				a, _ := ss.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).
					Times(1).Return(
					&datastore.Endpoint{
						UID:       "endpoint-id-1",
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

			ss := provideCreateSubscriptionService(ctrl, tc.args.project, tc.args.newSubscription)

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

			if tc.wantSubscription.FilterConfig != nil {
				require.Equal(t, subscription.FilterConfig.EventTypes,
					tc.wantSubscription.FilterConfig.EventTypes)
			}
		})
	}
}
