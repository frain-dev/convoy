package analytics

import (
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideOrganisationAnalytics(ctrl *gomock.Controller) *OrganisationAnalytics {
	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	client := NewNoopAnalyticsClient()

	return newOrganisationAnalytics(orgRepo, client, TestInstanceID)
}

func Test_TrackOrganisationAnalytics(t *testing.T) {

	tests := []struct {
		name    string
		dbFn    func(oa *OrganisationAnalytics)
		wantErr bool
	}{
		{
			name: "should_track_organisation_analytics",
			dbFn: func(oa *OrganisationAnalytics) {
				orgRepo := oa.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().LoadOrganisationsPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, nil)
			},
		},

		{
			name: "should_fail_to_track_organisation_analytics",
			dbFn: func(oa *OrganisationAnalytics) {
				orgRepo := oa.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().LoadOrganisationsPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			oa := provideOrganisationAnalytics(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(oa)
			}

			err := oa.Track()

			if tc.wantErr {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
		})
	}
}
