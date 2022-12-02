package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideOrganisationService(ctrl *gomock.Controller) *OrganisationService {
	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	orgMemberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
	return NewOrganisationService(orgRepo, orgMemberRepo)
}

func TestOrganisationService_CreateOrganisation(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		newOrg *models.Organisation
		user   *datastore.User
	}
	tests := []struct {
		name        string
		args        args
		want        *datastore.Organisation
		dbFn        func(os *OrganisationService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_organisation",
			args: args{
				ctx:    ctx,
				newOrg: &models.Organisation{Name: "new_org"},
				user:   &datastore.User{UID: "1234"},
			},
			want: &datastore.Organisation{Name: "new_org", OwnerID: "1234"},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().CreateOrganisation(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				om, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_validate_organisation",
			args: args{
				ctx:    ctx,
				newOrg: &models.Organisation{Name: ""},
				user:   &datastore.User{UID: "1234"},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "organisation name is required",
		},
		{
			name: "should_fail_to_create_organisation",
			args: args{
				ctx:    ctx,
				newOrg: &models.Organisation{Name: "new_org"},
				user:   &datastore.User{UID: "1234"},
			},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().CreateOrganisation(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create organisation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			os := provideOrganisationService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(os)
			}

			err := config.LoadConfig("")
			require.NoError(t, err)

			org, err := os.CreateOrganisation(tt.args.ctx, tt.args.newOrg, tt.args.user)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.NoError(t, err)
			stripVariableFields(t, "organisation", org)
			require.Equal(t, tt.want, org)
		})
	}
}

func TestOrganisationService_UpdateOrganisation(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		org    *datastore.Organisation
		update *models.Organisation
	}
	tests := []struct {
		name        string
		args        args
		want        *datastore.Organisation
		dbFn        func(os *OrganisationService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_update_organisation name",
			args: args{
				ctx:    ctx,
				org:    &datastore.Organisation{UID: "abc", Name: "test_org", DeletedAt: nil},
				update: &models.Organisation{Name: "test_update_org"},
			},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().UpdateOrganisation(gomock.Any(), &datastore.Organisation{UID: "abc", Name: "test_update_org"}).
					Times(1).Return(nil)
			},
			want:    &datastore.Organisation{UID: "abc", Name: "test_update_org"},
			wantErr: false,
		},
		{
			name: "should_update_organisation custom domain",
			args: args{
				ctx:    ctx,
				org:    &datastore.Organisation{UID: "abc", Name: "test_org", DeletedAt: nil},
				update: &models.Organisation{CustomDomain: "http://abc.com"},
			},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().UpdateOrganisation(gomock.Any(),
					&datastore.Organisation{
						UID:          "abc",
						Name:         "test_org",
						CustomDomain: "abc.com",
					}).
					Times(1).Return(nil)
			},
			want: &datastore.Organisation{
				UID:          "abc",
				Name:         "test_org",
				CustomDomain: "abc.com",
				DeletedAt:    nil,
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_update_organisation",
			args: args{
				ctx:    ctx,
				org:    &datastore.Organisation{UID: "123"},
				update: &models.Organisation{Name: "test_update_org"},
			},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().UpdateOrganisation(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to update organisation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			os := provideOrganisationService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(os)
			}

			org, err := os.UpdateOrganisation(tt.args.ctx, tt.args.org, tt.args.update)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, org)
		})
	}
}

func TestOrganisationService_FindOrganisationByID(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		id  string
	}
	tests := []struct {
		name        string
		args        args
		want        *datastore.Organisation
		dbFn        func(os *OrganisationService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_find_organisation_by_id",
			args: args{
				ctx: ctx,
				id:  "123",
			},
			want: &datastore.Organisation{UID: "123"},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().FetchOrganisationByID(gomock.Any(), "123").
					Times(1).Return(&datastore.Organisation{UID: "123"}, nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_find_organisation_by_id",
			args: args{
				ctx: ctx,
				id:  "123",
			},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().FetchOrganisationByID(gomock.Any(), "123").
					Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to find organisation by id",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			os := provideOrganisationService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(os)
			}

			org, err := os.FindOrganisationByID(tt.args.ctx, tt.args.id)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, org)
		})
	}
}

func TestOrganisationService_DeleteOrganisation(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx context.Context
		id  string
	}
	tests := []struct {
		name        string
		args        args
		dbFn        func(os *OrganisationService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_find_organisation_by_id",
			args: args{
				ctx: ctx,
				id:  "123",
			},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().DeleteOrganisation(gomock.Any(), "123").
					Times(1).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_find_organisation_by_id",
			args: args{
				ctx: ctx,
				id:  "123",
			},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().DeleteOrganisation(gomock.Any(), "123").
					Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete organisation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			os := provideOrganisationService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(os)
			}

			err := os.DeleteOrganisation(tt.args.ctx, tt.args.id)
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

func TestOrganisationService_LoadOrganisationsPaged(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx      context.Context
		pageable datastore.Pageable
	}
	tests := []struct {
		name               string
		dbFn               func(os *OrganisationService)
		args               args
		wantOrganisations  []datastore.Organisation
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_load_organisations_paged",
			args: args{
				ctx: ctx,
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				},
			},
			wantOrganisations: []datastore.Organisation{
				{UID: "123"},
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
			dbFn: func(os *OrganisationService) {
				o, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().LoadOrganisationsPaged(gomock.Any(), datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				}).Times(1).Return(
					[]datastore.Organisation{
						{UID: "123"},
						{UID: "abc"},
					}, datastore.PaginationData{
						Total:     1,
						Page:      1,
						PerPage:   1,
						Prev:      1,
						Next:      1,
						TotalPage: 1,
					},
					nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_load_organisations_paged",
			args: args{
				ctx: ctx,
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				},
			},
			wantOrganisations: []datastore.Organisation{
				{UID: "123"},
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
			dbFn: func(os *OrganisationService) {
				o, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				o.EXPECT().LoadOrganisationsPaged(gomock.Any(), datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				}).Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while fetching organisations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			os := provideOrganisationService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(os)
			}

			orgs, paginationData, err := os.LoadOrganisationsPaged(tt.args.ctx, tt.args.pageable)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantOrganisations, orgs)
			require.Equal(t, tt.wantPaginationData, paginationData)
		})
	}
}

func TestOrganisationService_LoadUserOrganisationsPaged(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx      context.Context
		pageable datastore.Pageable
		user     *datastore.User
	}
	tests := []struct {
		name               string
		dbFn               func(os *OrganisationService)
		args               args
		wantOrganisations  []datastore.Organisation
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_load_organisations_paged",
			args: args{
				ctx: ctx,
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				},
				user: &datastore.User{UID: "123"},
			},
			wantOrganisations: []datastore.Organisation{
				{UID: "123"},
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
			dbFn: func(os *OrganisationService) {
				o, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				o.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), "123", datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				}).Times(1).Return(
					[]datastore.Organisation{
						{UID: "123"},
						{UID: "abc"},
					}, datastore.PaginationData{
						Total:     1,
						Page:      1,
						PerPage:   1,
						Prev:      1,
						Next:      1,
						TotalPage: 1,
					},
					nil)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_load_organisations_paged",
			args: args{
				ctx: ctx,
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				},
				user: &datastore.User{UID: "123"},
			},
			wantOrganisations: []datastore.Organisation{
				{UID: "123"},
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
			dbFn: func(os *OrganisationService) {
				o, _ := os.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				o.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), "123", datastore.Pageable{
					Page:    1,
					PerPage: 1,
					Sort:    1,
				}).Times(1).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while fetching user organisations",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			os := provideOrganisationService(ctrl)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(os)
			}

			orgs, paginationData, err := os.LoadUserOrganisationsPaged(tt.args.ctx, tt.args.user, tt.args.pageable)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tt.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.wantOrganisations, orgs)
			require.Equal(t, tt.wantPaginationData, paginationData)
		})
	}
}
