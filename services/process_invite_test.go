package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"go.uber.org/mock/gomock"
)

func provideProcessInviteService(ctrl *gomock.Controller, token string, accepted bool, newUser *models.User) *ProcessInviteService {
	return &ProcessInviteService{
		Queue:         mocks.NewMockQueuer(ctrl),
		InviteRepo:    mocks.NewMockOrganisationInviteRepository(ctrl),
		UserRepo:      mocks.NewMockUserRepository(ctrl),
		OrgRepo:       mocks.NewMockOrganisationRepository(ctrl),
		OrgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
		Licenser:      mocks.NewMockLicenser(ctrl),

		Token:    token,
		Accepted: accepted,
		NewUser:  newUser,
	}
}

func TestProcessInviteService_Run(t *testing.T) {
	ctx := context.Background()
	expiry := time.Now().Add(time.Hour)

	type args struct {
		ctx      context.Context
		token    string
		accepted bool
		newUser  *models.User
	}
	tests := []struct {
		name       string
		dbFn       func(pis *ProcessInviteService)
		args       args
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_process_and_accept_organisation_member_invite",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
				oir.EXPECT().UpdateOrganisationInvite(gomock.Any(), &datastore.OrganisationInvite{
					OrganisationID: "123ab",
					Status:         datastore.InviteStatusAccepted,
					ExpiresAt:      expiry,
					InviteeEmail:   "test@email.com",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "ref",
						Endpoint: "",
					},
				}).Times(1).Return(nil)

				u, _ := pis.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{
						UID: "user-123",
					},
					nil,
				)

				o, _ := pis.OrgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "org-123"},
					nil,
				)

				om, _ := pis.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr: false,
		},

		{
			name: "should_error_for_invite_already_accepted",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusAccepted,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "organisation member invite already accepted",
		},

		{
			name: "should_error_for_licence_cant_create_org_member",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						InviteeEmail:   "test@email.com",
						ExpiresAt:      time.Now().Add(time.Hour),
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)

				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(false, nil)
			},
			wantErr:    true,
			wantErrMsg: ErrOrgMemberLimit.Error(),
		},
		{
			name: "should_error_for_invite_already_declined",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusDeclined,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "organisation member invite already declined",
		},
		{
			name: "should_error_for_invite_already_expired",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      time.Now().Add(-time.Minute),
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
			},
			wantErr:    true,
			wantErrMsg: "organisation member invite already expired",
		},
		{
			name: "should_fail_to_find_invite_by_token_and_email",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to fetch organisation member invite",
		},
		{
			name: "should_process_and_decline_organisation_member_invite",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: false,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
				oir.EXPECT().UpdateOrganisationInvite(gomock.Any(), &datastore.OrganisationInvite{
					OrganisationID: "123ab",
					Status:         datastore.InviteStatusDeclined,
					ExpiresAt:      expiry,
					InviteeEmail:   "test@email.com",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "ref",
						Endpoint: "",
					},
				}).Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_error_for_failed_to_find_user_by_email",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
				u, _ := pis.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, errors.New("failed"))

				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr:    true,
			wantErrMsg: "failed to find user by email",
		},
		{
			name: "should_process_and_accept_organisation_member_invite_for_new_user",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser: &models.User{
					FirstName: "Daniel",
					LastName:  "O.J",
					Email:     "test@gmail.com",
					Password:  "1234",
				},
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
				oir.EXPECT().UpdateOrganisationInvite(gomock.Any(), &datastore.OrganisationInvite{
					OrganisationID: "123ab",
					Status:         datastore.InviteStatusAccepted,
					ExpiresAt:      expiry,
					InviteeEmail:   "test@email.com",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "ref",
						Endpoint: "",
					},
				}).Times(1).Return(nil)

				u, _ := pis.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)

				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				o, _ := pis.OrgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "org-123"},
					nil,
				)

				om, _ := pis.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr: false,
		},
		{
			name: "should_error_for_nil_new_user",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
				u, _ := pis.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)

				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr:    true,
			wantErrMsg: "new user is nil",
		},
		{
			name: "should_error_for_failed_to_validate_new_user",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser: &models.User{
					FirstName: "",
					LastName:  "O.J",
					Email:     "test@gmail.com",
					Password:  "1234",
				},
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)

				u, _ := pis.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)

				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr:    true,
			wantErrMsg: "first_name:please provide a first name",
		},
		{
			name: "should_fail_to_create_new_user",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser: &models.User{
					FirstName: "Daniel",
					LastName:  "O.J",
					Email:     "test@gmail.com",
					Password:  "1234",
				},
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)

				u, _ := pis.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)

				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))

				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr:    true,
			wantErrMsg: "failed to create user",
		},
		{
			name: "should_fail_to_fetch_organisation_by_id",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)

				u, _ := pis.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{
						UID: "user-123",
					},
					nil,
				)

				o, _ := pis.OrgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").
					Times(1).Return(nil, errors.New("failed"))
				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr:    true,
			wantErrMsg: "failed to fetch organisation by id",
		},

		// TODO: temporarily removed pending the refactor of org member service
		//{
		//	name: "should_fail_to_create_organisation_member",
		//	args: args{
		//		ctx:      ctx,
		//		token:    "abcdef",
		//		accepted: true,
		//		newUser:  nil,
		//	},
		//	dbFn: func(pis *ProcessInviteService) {
		//		oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
		//		oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
		//			Times(1).Return(
		//			&datastore.OrganisationInvite{
		//				OrganisationID: "123ab",
		//				Status:         datastore.InviteStatusPending,
		//				ExpiresAt:      expiry,
		//				InviteeEmail:   "test@email.com",
		//				Role: auth.Role{
		//					Type:     auth.RoleAdmin,
		//					Project:  "ref",
		//					Endpoint: "",
		//				},
		//			},
		//			nil,
		//		)
		//
		//		u, _ := pis.UserRepo.(*mocks.MockUserRepository)
		//		u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
		//			&datastore.User{
		//				UID: "user-123",
		//			},
		//			nil,
		//		)
		//
		//		o, _ := pis.OrgRepo.(*mocks.MockOrganisationRepository)
		//		o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
		//			&datastore.Organisation{UID: "org-123"},
		//			nil,
		//		)
		//
		//		om, _ := pis.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
		//		om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).
		//			Times(1).Return(errors.New("failed"))
		//	},
		//	wantErr:    true,
		//	wantErrMsg: "failed to create organisation member",
		//},
		{
			name: "should_fail_to_update_organisation_member_invite",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(pis *ProcessInviteService) {
				oir, _ := pis.InviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Project:  "ref",
							Endpoint: "",
						},
					},
					nil,
				)
				oir.EXPECT().UpdateOrganisationInvite(gomock.Any(), &datastore.OrganisationInvite{
					OrganisationID: "123ab",
					Status:         datastore.InviteStatusAccepted,
					ExpiresAt:      expiry,
					InviteeEmail:   "test@email.com",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "ref",
						Endpoint: "",
					},
				}).Times(1).Return(errors.New("failed"))

				u, _ := pis.UserRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{
						UID: "user-123",
					},
					nil,
				)

				o, _ := pis.OrgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "org-123"},
					nil,
				)

				om, _ := pis.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				licenser, _ := pis.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr:    true,
			wantErrMsg: "failed to update accepted organisation invite",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			pis := provideProcessInviteService(ctrl, tt.args.token, tt.args.accepted, tt.args.newUser)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(pis)
			}

			err := pis.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
