package services

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
)

func provideCancelOrgMemberService(ctrl *gomock.Controller, inviteID string) *CancelOrgMemberService {
	return &CancelOrgMemberService{
		Queue:      mocks.NewMockQueuer(ctrl),
		InviteRepo: mocks.NewMockOrganisationInviteRepository(ctrl),
		InviteID:   inviteID,
	}
}

func TestCancelOrgMemberService_Run(t *testing.T) {
	ctx := context.Background()
	expiry := time.Now().Add(time.Hour)

	type args struct {
		inviteID string
		ctx      context.Context
	}
	tests := []struct {
		name       string
		args       args
		dbFn       func(co *CancelOrgMemberService)
		want       *datastore.OrganisationInvite
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_cancel_organisation_member_invite",
			args: args{
				ctx:      ctx,
				inviteID: "abc",
			},
			dbFn: func(co *CancelOrgMemberService) {
				a, _ := co.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				a.EXPECT().FetchOrganisationInviteByID(gomock.Any(), "abc").
					Times(1).Return(
					&datastore.OrganisationInvite{
						UID:            "abc",
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
			},
			want: &datastore.OrganisationInvite{
				OrganisationID: "123",
				Status:         datastore.InviteStatusCancelled,
				InviteeEmail:   "test@example.com",
				Role: auth.Role{
					Type:     auth.RoleAdmin,
					Project:  "ref",
					Endpoint: "",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			co := provideCancelOrgMemberService(ctrl, tt.args.inviteID)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(co)
			}

			iv, err := co.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, iv.DeletedAt)
			stripVariableFields(t, "organisation_invite", iv)
			require.Equal(t, tt.want, iv)
		})
	}
}
