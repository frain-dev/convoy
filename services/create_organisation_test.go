package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideCreateOrganisationService(ctrl *gomock.Controller, newOrg *models.Organisation, user *datastore.User) *CreateOrganisationService {
	return &CreateOrganisationService{
		OrgRepo:       mocks.NewMockOrganisationRepository(ctrl),
		OrgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
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
				a, _ := os.OrgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().CreateOrganisation(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to create organisation",
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
