package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/util"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
)

func provideFindUserByInviteTokenService(ctrl *gomock.Controller, token string) *FindUserByInviteTokenService {
	return &FindUserByInviteTokenService{
		Queue:      mocks.NewMockQueuer(ctrl),
		InviteRepo: mocks.NewMockOrganisationInviteRepository(ctrl),
		OrgRepo:    mocks.NewMockOrganisationRepository(ctrl),
		UserRepo:   mocks.NewMockUserRepository(ctrl),
		Token:      token,
	}
}

func TestFindUserByInviteTokenService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx   context.Context
		token string
	}

	tests := []struct {
		name        string
		args        args
		dbFn        func(co *FindUserByInviteTokenService)
		wantUser    *datastore.User
		wantInvite  *datastore.OrganisationInvite
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_find_user_by_invite_token",
			args: args{
				ctx:   ctx,
				token: "abcdef",
			},
			dbFn: func(ois *FindUserByInviteTokenService) {
				oir, _ := ois.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						InviteeEmail:   "test@email.com",
					},
					nil,
				)

				o, _ := ois.OrgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "123ab", Name: "test_org"},
					nil)

				u, _ := ois.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{
						UID:   "user-123",
						Email: "test@email.com",
					},
					nil,
				)
			},
			wantUser: &datastore.User{
				UID:   "user-123",
				Email: "test@email.com",
			},
			wantInvite: &datastore.OrganisationInvite{
				OrganisationID:   "123ab",
				OrganisationName: "test_org",
				InviteeEmail:     "test@email.com",
			},
		},

		{
			name: "should_not_find_user_by_invite_token",
			args: args{
				ctx:   ctx,
				token: "abcdef",
			},
			dbFn: func(ois *FindUserByInviteTokenService) {
				o, _ := ois.OrgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "123ab", Name: "test_org"},
					nil)

				oir, _ := ois.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						InviteeEmail:   "test@email.com",
					},
					nil,
				)

				u, _ := ois.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(nil, datastore.ErrUserNotFound)
			},
			wantUser: nil,
			wantInvite: &datastore.OrganisationInvite{
				OrganisationID:   "123ab",
				OrganisationName: "test_org",
				InviteeEmail:     "test@email.com",
			},
		},

		{
			name: "should_fail_to_find_user_by_invite_token",
			args: args{
				ctx:   ctx,
				token: "abcdef",
			},
			dbFn: func(ois *FindUserByInviteTokenService) {
				oir, _ := ois.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch organisation member invite",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ri := provideFindUserByInviteTokenService(ctrl, tt.args.token)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ri)
			}

			user, iv, err := ri.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, user, tt.wantUser)
			require.Equal(t, iv, tt.wantInvite)
		})
	}
}
