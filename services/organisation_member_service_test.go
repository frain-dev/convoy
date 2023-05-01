package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func provideOrganisationMemberService(ctrl *gomock.Controller) *OrganisationMemberService {
	orgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	return NewOrganisationMemberService(orgMemberRepo)
}

func TestOrganisationMemberService_CreateOrganisationMember(t *testing.T) {
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
			name: "should_create_organisation_member",
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
			name: "should_update_organisation_member",
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
			},
			wantErr: false,
		},
		{
			name: "should_update_organisation_member",
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

func TestOrganisationMemberService_FindOrganisationMemberByID(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		org *datastore.Organisation
		id  string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(os *OrganisationMemberService)
		want        *datastore.OrganisationMember
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_find_organisation_member_by_id",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "abc"},
				id:  "123",
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().FetchOrganisationMemberByID(gomock.Any(), "123", "abc").
					Times(1).Return(&datastore.OrganisationMember{UID: "123"}, nil)
			},
			want:    &datastore.OrganisationMember{UID: "123"},
			wantErr: false,
		},
		{
			name: "should_fail_to_find_organisation_member_by_id",
			args: args{
				ctx: ctx,
				id:  "123",
				org: &datastore.Organisation{UID: "abc"},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().FetchOrganisationMemberByID(gomock.Any(), "123", "abc").
					Times(1).Return(nil, errors.New("failed"))
			},
			want:        &datastore.OrganisationMember{UID: "123"},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find organisation member by id",
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

			member, err := om.FindOrganisationMemberByID(tt.args.ctx, tt.args.org, tt.args.id)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, member)
		})
	}
}

func TestOrganisationMemberService_LoadOrganisationMembersPaged(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx      context.Context
		org      *datastore.Organisation
		pageable datastore.Pageable
	}
	tests := []struct {
		name               string
		args               args
		dbFn               func(os *OrganisationMemberService)
		wantMembers        []*datastore.OrganisationMember
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_load_organisation_members_paged",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "123"},
				pageable: datastore.Pageable{
					PerPage:    1,
					NextCursor: datastore.DefaultCursor,
					Direction:  datastore.Next,
				},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().LoadOrganisationMembersPaged(gomock.Any(), "123", "123", datastore.Pageable{
					PerPage:    1,
					NextCursor: datastore.DefaultCursor,
					Direction:  datastore.Next,
				}).Times(1).Return([]*datastore.OrganisationMember{
					{UID: "123"},
					{UID: "345"},
					{UID: "abc"},
				}, datastore.PaginationData{
					PerPage: 1,
				},
					nil)
			},
			wantMembers: []*datastore.OrganisationMember{
				{UID: "123"},
				{UID: "345"},
				{UID: "abc"},
			},
			wantPaginationData: datastore.PaginationData{
				PerPage: 1,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_load_organisation_members_paged",
			args: args{
				ctx: ctx,
				org: &datastore.Organisation{UID: "123"},
				pageable: datastore.Pageable{
					PerPage:    1,
					NextCursor: datastore.DefaultCursor,
					Direction:  datastore.Next,
				},
			},
			dbFn: func(os *OrganisationMemberService) {
				a, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				a.EXPECT().LoadOrganisationMembersPaged(gomock.Any(), "123", "123", datastore.Pageable{
					PerPage:    1,
					NextCursor: datastore.DefaultCursor,
					Direction:  datastore.Next,
				}).Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to load organisation members",
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

			members, paginationData, err := om.LoadOrganisationMembersPaged(tt.args.ctx, tt.args.org, "", tt.args.pageable)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantMembers, members)
			require.Equal(t, tt.wantPaginationData, paginationData)
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
