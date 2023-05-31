package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
)

func provideFinSubscriptionByIDService(ctrl *gomock.Controller, project *datastore.Project, subID string, skipCache bool) *FindSubscriptionByIDService {
	return &FindSubscriptionByIDService{
		SubRepo:        mocks.NewMockSubscriptionRepository(ctrl),
		EndpointRepo:   mocks.NewMockEndpointRepository(ctrl),
		SourceRepo:     mocks.NewMockSourceRepository(ctrl),
		Project:        project,
		SubscriptionId: subID,
		SkipCache:      skipCache,
	}
}

func TestFindSubscriptionByIDService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx       context.Context
		project   *datastore.Project
		subID     string
		skipCache bool
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(so *FindSubscriptionByIDService)
		wantSub    *datastore.Subscription
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_find_subscription_by_id_for_incoming_project",
			args: args{
				ctx:       ctx,
				project:   &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				subID:     "abc",
				skipCache: false,
			},
			dbFn: func(so *FindSubscriptionByIDService) {
				s, _ := so.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), "1234", "abc").Times(1).Return(
					&datastore.Subscription{UID: "abc", EndpointID: "endpoint1", ProjectID: "1234", SourceID: "source1"},
					nil,
				)

				e, _ := so.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "endpoint1", "1234").Times(1).Return(
					&datastore.Endpoint{UID: "endpoint1", ProjectID: "1234"},
					nil,
				)

				sr, _ := so.SourceRepo.(*mocks.MockSourceRepository)
				sr.EXPECT().FindSourceByID(gomock.Any(), "1234", "source1").Times(1).Return(
					&datastore.Source{UID: "source1", ProjectID: "1234"},
					nil,
				)
			},
			wantSub: &datastore.Subscription{
				UID:        "abc",
				EndpointID: "endpoint1",
				ProjectID:  "1234",
				SourceID:   "source1",
				Source:     &datastore.Source{UID: "source1", ProjectID: "1234"},
				Endpoint:   &datastore.Endpoint{UID: "endpoint1", ProjectID: "1234"},
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_find_subscription_by_id_for_outgoing_project",
			args: args{
				ctx:       ctx,
				project:   &datastore.Project{UID: "1234", Type: datastore.OutgoingProject},
				subID:     "abc",
				skipCache: true,
			},
			dbFn: func(so *FindSubscriptionByIDService) {
				s, _ := so.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), "1234", "abc").Times(1).Return(
					&datastore.Subscription{UID: "abc", EndpointID: "endpoint1", ProjectID: "1234", SourceID: "source1"},
					nil,
				)
			},
			wantSub: &datastore.Subscription{
				UID:        "abc",
				EndpointID: "endpoint1",
				ProjectID:  "1234",
				SourceID:   "source1",
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_fail_to_find_subscription",
			args: args{
				ctx:       ctx,
				project:   &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				subID:     "abc",
				skipCache: false,
			},
			dbFn: func(so *FindSubscriptionByIDService) {
				s, _ := so.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), "1234", "abc").Times(1).Return(
					nil,
					errors.New("failed"),
				)
			},
			wantErr:    true,
			wantErrMsg: "subscription not found",
		},
		{
			name: "should_fail_to_find_endpoint",
			args: args{
				ctx:       ctx,
				project:   &datastore.Project{UID: "1234", Type: datastore.OutgoingProject},
				subID:     "abc",
				skipCache: false,
			},
			dbFn: func(so *FindSubscriptionByIDService) {
				s, _ := so.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), "1234", "abc").Times(1).Return(
					&datastore.Subscription{UID: "abc", EndpointID: "endpoint1", ProjectID: "1234", SourceID: "source1"},
					nil,
				)

				e, _ := so.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), "endpoint1", "1234").Times(1).Return(
					nil,
					errors.New("failed"),
				)
			},
			wantErr:    true,
			wantErrMsg: "failed to find subscription app endpoint",
		},
		{
			name: "should_fail_to_find_source",
			args: args{
				ctx:       ctx,
				project:   &datastore.Project{UID: "1234", Type: datastore.IncomingProject},
				subID:     "abc",
				skipCache: false,
			},
			dbFn: func(so *FindSubscriptionByIDService) {
				s, _ := so.SubRepo.(*mocks.MockSubscriptionRepository)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), "1234", "abc").Times(1).Return(
					&datastore.Subscription{UID: "abc", EndpointID: "endpoint1", ProjectID: "1234", SourceID: "source1"},
					nil,
				)

				sr, _ := so.SourceRepo.(*mocks.MockSourceRepository)
				sr.EXPECT().FindSourceByID(gomock.Any(), "1234", "source1").Times(1).Return(
					nil,
					errors.New("failed"),
				)
			},
			wantErr:    true,
			wantErrMsg: "failed to find subscription source",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ss := provideFinSubscriptionByIDService(ctrl, tt.args.project, tt.args.subID, tt.args.skipCache)

			if tt.dbFn != nil {
				tt.dbFn(ss)
			}

			got, err := ss.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tt.wantSub, got)
		})
	}
}
