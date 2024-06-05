package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/guregu/null.v4"
)

func provideUpdateOrganisationService(ctrl *gomock.Controller, org *datastore.Organisation, update *models.Organisation) *UpdateOrganisationService {
	return &UpdateOrganisationService{
		OrgRepo:       mocks.NewMockOrganisationRepository(ctrl),
		OrgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
		Org:           org,
		Update:        update,
	}
}

func TestUpdateOrganisationService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		org    *datastore.Organisation
		update *models.Organisation
	}
	tests := []struct {
		name       string
		args       args
		want       *datastore.Organisation
		dbFn       func(os *UpdateOrganisationService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_update_organisation name",
			args: args{
				ctx:    ctx,
				org:    &datastore.Organisation{UID: "abc", Name: "test_org"},
				update: &models.Organisation{Name: "test_update_org"},
			},
			dbFn: func(os *UpdateOrganisationService) {
				a, _ := os.OrgRepo.(*mocks.MockOrganisationRepository)
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
			dbFn: func(os *UpdateOrganisationService) {
				a, _ := os.OrgRepo.(*mocks.MockOrganisationRepository)
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
			dbFn: func(os *UpdateOrganisationService) {
				a, _ := os.OrgRepo.(*mocks.MockOrganisationRepository)
				a.EXPECT().UpdateOrganisation(gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to update organisation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			os := provideUpdateOrganisationService(ctrl, tt.args.org, tt.args.update)

			// Arrange Expectations
			if tt.dbFn != nil {
				tt.dbFn(os)
			}

			org, err := os.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want, org)
		})
	}
}
