package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func provideOrganisationInviteService(ctrl *gomock.Controller) *OrganisationInviteService {
	orgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	userRepo := mocks.NewMockUserRepository(ctrl)
	orgInviteRepo := mocks.NewMockOrganisationInviteRepository(ctrl)
	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	queue := mocks.NewMockQueuer(ctrl)
	return NewOrganisationInviteService(orgRepo, userRepo, orgMemberRepo, orgInviteRepo, queue)
}

func TestOrganisationInviteService_CreateOrganisationMemberInvite(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx     context.Context
		org     *datastore.Organisation
		newIV   *models.OrganisationInvite
		user    *datastore.User
		baseURL string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ois *OrganisationInviteService)
		want        *datastore.OrganisationInvite
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_organisation_member_invite",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "123"},
				newIV: &models.OrganisationInvite{
					InviteeEmail: "test@example.com",
					Role: auth.Role{
						Type:  auth.RoleAdmin,
						Group: "abc",
					},
				},
				user:    &datastore.User{},
				baseURL: "https://google.com",
			},
			dbFn: func(ois *OrganisationInviteService) {
				a, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				a.EXPECT().CreateOrganisationInvite(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				q := ois.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			want: &datastore.OrganisationInvite{
				OrganisationID: "123",
				InviteeEmail:   "test@example.com",
				Role: auth.Role{
					Type:  auth.RoleAdmin,
					Group: "abc",
				},
				Status: datastore.InviteStatusPending,
			},
			wantErr: false,
		},
		{
			name: "should_error_for_empty_invitee_email",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "123"},
				newIV: &models.OrganisationInvite{
					InviteeEmail: "",
					Role: auth.Role{
						Type:  auth.RoleAdmin,
						Group: "abc",
					},
				},
				user:    &datastore.User{},
				baseURL: "https://google.com",
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "invitee_email:please provide a valid invitee email",
		},
		{
			name: "should_error_for_invalid_role",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "123"},
				newIV: &models.OrganisationInvite{
					InviteeEmail: "test@example.com",
					Role: auth.Role{
						Type: auth.RoleAdmin,
					},
				},
				user:    nil,
				baseURL: "",
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "please specify group for organisation member",
		},
		{
			name: "should_fail_to_create_organisation_member_invite",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "123"},
				newIV: &models.OrganisationInvite{
					InviteeEmail: "test@example.com",
					Role: auth.Role{
						Type:  auth.RoleAdmin,
						Group: "abc",
					},
				},
				user:    nil,
				baseURL: "",
			},
			dbFn: func(ois *OrganisationInviteService) {
				a, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				a.EXPECT().CreateOrganisationInvite(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create organisation member invite",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ois := provideOrganisationInviteService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ois)
			}

			iv, err := ois.CreateOrganisationMemberInvite(tt.args.ctx, tt.args.newIV, tt.args.org, tt.args.user, tt.args.baseURL)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, iv.Token)
			stripVariableFields(t, "organisation_invite", iv)
			require.Equal(t, tt.want, iv)
		})
	}
}

func TestOrganisationInviteService_ProcessOrganisationMemberInvite(t *testing.T) {
	ctx := context.Background()
	expiry := primitive.NewDateTimeFromTime(time.Now().Add(time.Hour))

	type args struct {
		ctx      context.Context
		token    string
		accepted bool
		newUser  *models.User
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ois *OrganisationInviteService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_process_and_accept_organisation_member_invite",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
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
						Group:    "ref",
						Endpoint: "",
					},
				}).Times(1).Return(nil)

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{
						UID: "user-123",
					},
					nil,
				)

				o, _ := ois.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "org-123"},
					nil,
				)

				om, _ := ois.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)
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
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusAccepted,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "organisation member invite already accepted",
		},
		{
			name: "should_error_for_invite_already_declined",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusDeclined,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "organisation member invite already declined",
		},
		{
			name: "should_error_for_invite_already_expired",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      primitive.NewDateTimeFromTime(time.Now().Add(-time.Minute)),
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "organisation member invite already expired",
		},
		{
			name: "should_fail_to_find_invite_by_token_and_email",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch organisation member invite",
		},
		{
			name: "should_process_and_decline_organisation_member_invite",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: false,
				newUser:  nil,
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
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
						Group:    "ref",
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
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)
				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find user by email",
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
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
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
						Group:    "ref",
						Endpoint: "",
					},
				}).Times(1).Return(nil)

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)

				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				o, _ := ois.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "org-123"},
					nil,
				)

				om, _ := ois.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)
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
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)
				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "new user is nil",
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
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "first_name:please provide a first name",
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
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").
					Times(1).Return(nil, datastore.ErrUserNotFound)

				u.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create user",
		},
		{
			name: "should_fail_to_fetch_organisation_by_id",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{
						UID: "user-123",
					},
					nil,
				)

				o, _ := ois.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to fetch organisation by id",
		},
		{
			name: "should_fail_to_create_organisation_member",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					},
					nil,
				)

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{
						UID: "user-123",
					},
					nil,
				)

				o, _ := ois.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "org-123"},
					nil,
				)

				om, _ := ois.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create organisation member",
		},
		{
			name: "should_fail_to_update_organisation_member_invite",
			args: args{
				ctx:      ctx,
				token:    "abcdef",
				accepted: true,
				newUser:  nil,
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@email.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
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
						Group:    "ref",
						Endpoint: "",
					},
				}).Times(1).Return(errors.New("failed"))

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(
					&datastore.User{
						UID: "user-123",
					},
					nil,
				)

				o, _ := ois.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().FetchOrganisationByID(gomock.Any(), "123ab").Times(1).Return(
					&datastore.Organisation{UID: "org-123"},
					nil,
				)

				om, _ := ois.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to update accepted organisation invite",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ois := provideOrganisationInviteService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ois)
			}

			err := ois.ProcessOrganisationMemberInvite(tt.args.ctx, tt.args.token, tt.args.accepted, tt.args.newUser)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestOrganisationInviteService_LoadOrganisationInvitesPaged(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx          context.Context
		org          *datastore.Organisation
		inviteStatus datastore.InviteStatus
		pageable     datastore.Pageable
	}
	tests := []struct {
		name               string
		args               args
		wantInvites        []datastore.OrganisationInvite
		wantPaginationData datastore.PaginationData
		dbFn               func(ois *OrganisationInviteService)
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_load_organisation_invites",
			args: args{
				ctx:          ctx,
				org:          &datastore.Organisation{UID: "123"},
				inviteStatus: datastore.InviteStatusAccepted,
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				},
			},
			dbFn: func(ois *OrganisationInviteService) {
				o, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				o.EXPECT().LoadOrganisationsInvitesPaged(gomock.Any(), "123", datastore.InviteStatusAccepted, datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				}).Times(1).Return(
					[]datastore.OrganisationInvite{
						{UID: "abc"},
					},
					datastore.PaginationData{
						Total:     1,
						Page:      1,
						PerPage:   1,
						Prev:      1,
						Next:      1,
						TotalPage: 1,
					},
					nil,
				)
			},
			wantInvites: []datastore.OrganisationInvite{
				{UID: "abc"},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     1,
				Page:      1,
				PerPage:   1,
				Prev:      1,
				Next:      1,
				TotalPage: 1,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_load_organisation_invites",
			args: args{
				ctx:          ctx,
				org:          &datastore.Organisation{UID: "123"},
				inviteStatus: datastore.InviteStatusAccepted,
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				},
			},
			dbFn: func(ois *OrganisationInviteService) {
				o, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				o.EXPECT().LoadOrganisationsInvitesPaged(gomock.Any(), "123", datastore.InviteStatusAccepted, datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				}).Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to load organisation invites",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			ois := provideOrganisationInviteService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ois)
			}

			invites, paginationData, err := ois.LoadOrganisationInvitesPaged(tt.args.ctx, tt.args.org, tt.args.inviteStatus, tt.args.pageable)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantInvites, invites)
			require.Equal(t, tt.wantPaginationData, paginationData)
		})
	}
}

func TestOrganisationInviteService_FindUserByInviteToken(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx   context.Context
		token string
	}

	tests := []struct {
		name        string
		args        args
		dbFn        func(ois *OrganisationInviteService)
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
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						InviteeEmail:   "test@email.com",
					},
					nil,
				)

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
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
				OrganisationID: "123ab",
				InviteeEmail:   "test@email.com",
			},
		},

		{
			name: "should_not_find_user_by_invite_token",
			args: args{
				ctx:   ctx,
				token: "abcdef",
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				oir.EXPECT().FetchOrganisationInviteByToken(gomock.Any(), "abcdef").
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123ab",
						InviteeEmail:   "test@email.com",
					},
					nil,
				)

				u, _ := ois.userRepo.(*mocks.MockUserRepository)
				u.EXPECT().FindUserByEmail(gomock.Any(), "test@email.com").Times(1).Return(nil, datastore.ErrUserNotFound)
			},
			wantUser: nil,
			wantInvite: &datastore.OrganisationInvite{
				OrganisationID: "123ab",
				InviteeEmail:   "test@email.com",
			},
		},

		{
			name: "should_fail_to_find_user_by_invite_token",
			args: args{
				ctx:   ctx,
				token: "abcdef",
			},
			dbFn: func(ois *OrganisationInviteService) {
				oir, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
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
			ois := provideOrganisationInviteService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ois)
			}

			user, iv, err := ois.FindUserByInviteToken(tt.args.ctx, tt.args.token)
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

func TestOrganisationInviteService_ResendOrganisationMemberInvite(t *testing.T) {
	ctx := context.Background()
	expiry := primitive.NewDateTimeFromTime(time.Now().Add(time.Hour))
	type args struct {
		ctx     context.Context
		org     *datastore.Organisation
		ivID    string
		user    *datastore.User
		baseURL string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ois *OrganisationInviteService)
		want        *datastore.OrganisationInvite
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_resend_organisation_member_invite",
			args: args{
				ctx:     ctx,
				org:     &datastore.Organisation{UID: "123"},
				ivID:    "abcd",
				user:    &datastore.User{},
				baseURL: "https://google.com",
			},
			dbFn: func(ois *OrganisationInviteService) {
				a, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				a.EXPECT().FetchOrganisationInviteByID(gomock.Any(), gomock.Any()).
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@example.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
							Endpoint: "",
						},
					}, nil)

				a.EXPECT().UpdateOrganisationInvite(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				q := ois.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			want: &datastore.OrganisationInvite{
				OrganisationID: "123",
				InviteeEmail:   "test@example.com",
				Role: auth.Role{
					Type:  auth.RoleAdmin,
					Group: "ref",
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
			ois := provideOrganisationInviteService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ois)
			}

			iv, err := ois.ResendOrganisationMemberInvite(tt.args.ctx, tt.args.ivID, tt.args.org, tt.args.user, tt.args.baseURL)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			stripVariableFields(t, "organisation_invite", iv)
			require.Equal(t, tt.want, iv)
		})
	}
}

func TestOrganisationInviteService_CancelOrganisationMemberInvite(t *testing.T) {
	ctx := context.Background()
	expiry := primitive.NewDateTimeFromTime(time.Now().Add(time.Hour))
	type args struct {
		ctx  context.Context
		org  *datastore.Organisation
		ivID string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(ois *OrganisationInviteService)
		want        *datastore.OrganisationInvite
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_cancel_organisation_member_invite",
			args: args{
				ctx:  ctx,
				org:  &datastore.Organisation{UID: "123"},
				ivID: uuid.NewString(),
			},
			dbFn: func(ois *OrganisationInviteService) {
				a, _ := ois.orgInviteRepo.(*mocks.MockOrganisationInviteRepository)
				a.EXPECT().FetchOrganisationInviteByID(gomock.Any(), gomock.Any()).
					Times(1).Return(
					&datastore.OrganisationInvite{
						OrganisationID: "123",
						Status:         datastore.InviteStatusPending,
						ExpiresAt:      expiry,
						InviteeEmail:   "test@example.com",
						Role: auth.Role{
							Type:     auth.RoleAdmin,
							Group:    "ref",
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
					Group:    "ref",
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
			ois := provideOrganisationInviteService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(ois)
			}

			iv, err := ois.CancelOrganisationMemberInvite(tt.args.ctx, tt.args.ivID)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, iv.DeletedAt)
			stripVariableFields(t, "organisation_invite", iv)
			require.Equal(t, tt.want, iv)
		})
	}
}
