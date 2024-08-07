package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func provideCreateOrganisationService(ctrl *gomock.Controller, newOrg *models.Organisation, user *datastore.User) *CreateOrganisationService {
	return &CreateOrganisationService{
		OrgRepo:       mocks.NewMockOrganisationRepository(ctrl),
		OrgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
		Licenser:      mocks.NewMockLicenser(ctrl),
		NewOrg:        newOrg,
		User:          user,
	}
}

func TestCreateOrganisationService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		newOrg *models.Organisation
		user   *datastore.User
	}
	tests := []struct {
		name       string
		args       args
		want       *datastore.Organisation
		dbFn       func(os *CreateOrganisationService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_create_organisation",
			args: args{
				ctx:    ctx,
				newOrg: &models.Organisation{Name: "new_org"},
				user:   &datastore.User{UID: "1234"},
			},
			want: &datastore.Organisation{Name: "new_org", OwnerID: "1234"},
			dbFn: func(os *CreateOrganisationService) {
				a, _ := os.OrgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().CreateOrganisation(gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				om, _ := os.OrgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CreateOrganisationMember(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				licenser, _ := os.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CanCreateOrg(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().CanCreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
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
			dbFn: func(os *CreateOrganisationService) {
				licenser, _ := os.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CanCreateOrg(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().CanCreateOrgMember(gomock.Any()).Times(1).Return(true, nil)
			},
			wantErr:    true,
			wantErrMsg: "organisation name is required",
		},
		{
			name: "should_fail_to_create_organisation",
			args: args{
				ctx:    ctx,
				newOrg: &models.Organisation{Name: "new_org"},
				user:   &datastore.User{UID: "1234"},
			},
			dbFn: func(os *CreateOrganisationService) {
				licenser, _ := os.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CanCreateOrg(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().CanCreateOrgMember(gomock.Any()).Times(1).Return(true, nil)

				a, _ := os.OrgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().CreateOrganisation(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to create organisation",
		},
		{
			name: "should_fail_to_create_organisation_for_license_check",
			args: args{
				ctx:    ctx,
				newOrg: &models.Organisation{Name: "new_org"},
				user:   &datastore.User{UID: "1234"},
			},
			dbFn: func(os *CreateOrganisationService) {
				licenser, _ := os.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CanCreateOrg(gomock.Any()).Times(1).Return(false, nil)
			},
			wantErr:    true,
			wantErrMsg: ErrOrgLimit.Error(),
		},
		{
			name: "should_fail_to_create_organisation_member_for_license_check",
			args: args{
				ctx:    ctx,
				newOrg: &models.Organisation{Name: "new_org"},
				user:   &datastore.User{UID: "1234"},
			},
			dbFn: func(os *CreateOrganisationService) {
				licenser, _ := os.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CanCreateOrg(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().CanCreateOrgMember(gomock.Any()).Times(1).Return(false, nil)
			},
			wantErr:    true,
			wantErrMsg: ErrOrgMemberLimit.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			os := provideCreateOrganisationService(ctrl, tt.args.newOrg, tt.args.user)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(os)
			}

			err := config.LoadConfig("")
			require.NoError(t, err)

			org, err := os.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.NoError(t, err)
			stripVariableFields(t, "organisation", org)
			require.Equal(t, tt.want, org)
		})
	}
}
