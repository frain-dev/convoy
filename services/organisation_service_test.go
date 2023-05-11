package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
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
				org:    &datastore.Organisation{UID: "abc", Name: "test_org"},
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
				org:    &datastore.Organisation{UID: "abc", Name: "test_org"},
				update: &models.Organisation{CustomDomain: "http://abc.com"},
			},
			dbFn: func(os *OrganisationService) {
				a, _ := os.orgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().UpdateOrganisation(gomock.Any(),
					&datastore.Organisation{
						UID:          "abc",
						Name:         "test_org",
						CustomDomain: null.NewString("abc.com", true),
					}).
					Times(1).Return(nil)
			},
			want: &datastore.Organisation{
				UID:          "abc",
				Name:         "test_org",
				CustomDomain: null.NewString("abc.com", true),
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
