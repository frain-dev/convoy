package services

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/util"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func provideResendOrgMemberService(ctrl *gomock.Controller, inviteID string, user *datastore.User, org *datastore.Organisation) *ResendOrgMemberService {
	return &ResendOrgMemberService{
		Queue:        mocks.NewMockQueuer(ctrl),
		InviteRepo:   mocks.NewMockOrganisationInviteRepository(ctrl),
		InviteID:     inviteID,
		User:         user,
		Organisation: org,
	}
}

func TestResendOrgMemberService_Run(t *testing.T) {
	ctx := context.Background()
	expiry := time.Now().Add(time.Hour)

	type args struct {
		ctx          context.Context
		InviteID     string
		User         *datastore.User
		Organisation *datastore.Organisation
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(rs *ResendOrgMemberService)
		want        *datastore.OrganisationInvite
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_resend_organisation_member_invite",
			args: args{
				ctx:          ctx,
				Organisation: &datastore.Organisation{UID: "123"},
				InviteID:     "abcd",
				User:         &datastore.User{},
			},
			dbFn: func(rs *ResendOrgMemberService) {
				a, _ := rs.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				a.EXPECT().FetchOrganisationInviteByID(gomock.Any(), gomock.Any()).
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@example.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					}, nil)

				a.EXPECT().UpdateOrganisationInvite(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				q := rs.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			want: &datastore.OrganisationInvite{
				OrganisationID: "123",
				InviteeEmail:   "test@example.com",
				Role: auth.Role{
					Type:    auth.RoleAdmin,
					Project: "ref",
				},
				Status: datastore.InviteStatusPending,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ri := provideResendOrgMemberService(ctrl, tt.args.InviteID, tt.args.User, tt.args.Organisation)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ri)
			}

			iv, err := ri.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, iv, tt.want)
		})
	}
}
