package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func provideOrganisationMemberService(ctrl *gomock.Controller) *OrganisationMemberService {
	orgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	l := mocks.NewMockLicenser(ctrl)
	return NewOrganisationMemberService(orgMemberRepo, l)
}

func TestOrganisationMemberService_CreateOrgaTnisationMember(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx  context.Context
		org  *datastore.Organisation
		user *datastore.User
		role *auth.Role
	}
	tests := []struct {
		name        string
		args        args
		want        *datastore.OrganisationMember
		dbFn        func(os *OrganisationMemberService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_organisation_member_admin_role",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "1234"},
				role: &auth.Role{
					Type:     auth.RoleAdmin,
					Project:  "123",
					Endpoint: "abc",
				},
				user: &datastore.User{UID: "1234"},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				l, _ := os.licenser.(*mocks.MockLicenser)
				l.EXPECT().MultiPlayerMode().Times(1).Return(true)
			},
			want: &datastore.OrganisationMember{
				OrganisationID: "1234",
				UserID:         "1234",
				Role: auth.Role{
					Type:     auth.RoleAdmin,
					Project:  "123",
					Endpoint: "abc",
				},
			},
			wantErr: false,
		},
		{
			name: "should_create_organisation_member_super_user_role",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "1234"},
				role: &auth.Role{
					Type:     auth.RoleAdmin,
					Project:  "123",
					Endpoint: "abc",
				},
				user: &datastore.User{UID: "1234"},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				l, _ := os.licenser.(*mocks.MockLicenser)
				l.EXPECT().MultiPlayerMode().Times(1).Return(false)
			},
			want: &datastore.OrganisationMember{
				OrganisationID: "1234",
				UserID:         "1234",
				Role: auth.Role{
					Type:     auth.RoleOrganisationAdmin,
					Project:  "123",
					Endpoint: "abc",
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_organisation_member",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "1234"},
				role: &auth.Role{
					Type:     auth.RoleAdmin,
					Project:  "123",
					Endpoint: "abc",
				},
				user: &datastore.User{UID: "1234"},
			},
			dbFn: func(os *OrganisationMemberService) {
				l, _ := os.licenser.(*mocks.MockLicenser)
				l.EXPECT().MultiPlayerMode().Times(1).Return(true)

				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create organisation member",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			om := provideOrganisationMemberService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(om)
			}

			member, err := om.CreateOrganisationMember(tt.args.ctx, tt.args.org, tt.args.user, tt.args.role)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			stripVariableFields(t, "organisation_member", member)
			require.Equal(t, tt.want, member)
		})
	}
}

func TestOrganisationMemberService_UpdateOrganisationMember(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx                context.Context
		organisationMember *datastore.OrganisationMember
		role               *auth.Role
	}
	tests := []struct {
		name        string
		args        args
		want        *datastore.OrganisationMember
		dbFn        func(os *OrganisationMemberService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_organisation_member_api_role",
			args: args{
				ctx: ctx,
				organisationMember: &datastore.OrganisationMember{
					UID:            "123",
					OrganisationID: "abc",
					UserID:         "def",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "111",
						Endpoint: "",
					},
				},
				role: &auth.Role{
					Type:     auth.RoleAPI,
					Project:  "333",
					Endpoint: "",
				},
			},
			want: &datastore.OrganisationMember{
				UID:            "123",
				OrganisationID: "abc",
				UserID:         "def",
				Role: auth.Role{
					Type:     auth.RoleAPI,
					Project:  "333",
					Endpoint: "",
				},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().UpdateOrganisationMember(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				l, _ := os.licenser.(*mocks.MockLicenser)
				l.EXPECT().MultiPlayerMode().Times(1).Return(true)
			},
			wantErr: false,
		},
		{
			name: "should_update_organisation_member_superuser_role",
			args: args{
				ctx: ctx,
				organisationMember: &datastore.OrganisationMember{
					UID:            "123",
					OrganisationID: "abc",
					UserID:         "def",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "111",
						Endpoint: "",
					},
				},
				role: &auth.Role{
					Type:     auth.RoleAPI,
					Project:  "333",
					Endpoint: "",
				},
			},
			want: &datastore.OrganisationMember{
				UID:            "123",
				OrganisationID: "abc",
				UserID:         "def",
				Role: auth.Role{
					Type:     auth.RoleOrganisationAdmin,
					Project:  "333",
					Endpoint: "",
				},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().UpdateOrganisationMember(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				l, _ := os.licenser.(*mocks.MockLicenser)
				l.EXPECT().MultiPlayerMode().Times(1).Return(false)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_update_organisation_member",
			args: args{
				ctx: ctx,
				organisationMember: &datastore.OrganisationMember{
					UID:            "123",
					OrganisationID: "abc",
					UserID:         "def",
					Role: auth.Role{
						Type:     auth.RoleAdmin,
						Project:  "111",
						Endpoint: "",
					},
				},
				role: &auth.Role{
					Type:     auth.RoleAPI,
					Project:  "333",
					Endpoint: "",
				},
			},
			dbFn: func(os *OrganisationMemberService) {
				l, _ := os.licenser.(*mocks.MockLicenser)
				l.EXPECT().MultiPlayerMode().Times(1).Return(true)

				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().UpdateOrganisationMember(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to update organisation member",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			om := provideOrganisationMemberService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(om)
			}

			member, err := om.UpdateOrganisationMember(tt.args.ctx, tt.args.organisationMember, tt.args.role)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			member.UpdatedAt = time.Time{}
			require.Equal(t, tt.want, member)
		})
	}
}

func TestOrganisationMemberService_DeleteOrganisationMember(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		id  string
		org *datastore.Organisation
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(os *OrganisationMemberService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_delete_organisation_member",
			args: args{
				ctx: ctx,
				id:  "123",
				org: &datastore.Organisation{
					UID:     "abc",
					OwnerID: "12345",
				},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().FetchOrganisationMemberByID(gomock.Any(), "123", "abc").Times(1).Return(&datastore.OrganisationMember{UID: "12345", UserID: "123"}, nil)
				a.EXPECT().DeleteOrganisationMember(gomock.Any(), "123", "abc").
					Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_delete_organisation_owner",
			args: args{
				ctx: ctx,
				id:  "123",
				org: &datastore.Organisation{
					UID:     "abc",
					OwnerID: "123",
				},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().FetchOrganisationMemberByID(gomock.Any(), "123", "abc").Times(1).Return(&datastore.OrganisationMember{UID: "12345", UserID: "123"}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusForbidden,
			wantErrMsg:  "cannot deactivate organisation owner",
		},
		{
			name: "should_fail_to_delete_organisation_member",
			args: args{
				ctx: ctx,
				id:  "123",
				org: &datastore.Organisation{
					UID:     "abc",
					OwnerID: "12345",
				},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().FetchOrganisationMemberByID(gomock.Any(), "123", "abc").Times(1).Return(&datastore.OrganisationMember{UID: "12345", UserID: "123"}, nil)
				a.EXPECT().DeleteOrganisationMember(gomock.Any(), "123", "abc").
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete organisation member",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			om := provideOrganisationMemberService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(om)
			}

			err := om.DeleteOrganisationMember(tt.args.ctx, tt.args.id, tt.args.org)
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
