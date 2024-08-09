package keygen

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/frain-dev/convoy/mocks"

	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"
)

func TestKeygenLicenser_MutualTLS(t *testing.T) {
	trueLicenser := KeygenLicenser{featureList: map[Feature]Properties{
		UseForwardProxy:         {},
		ExportPrometheusMetrics: {},
		AdvancedEndpointMgmt:    {},
		AdvancedRetentionPolicy: {},
		AdvancedMsgBroker:       {},
		AdvancedSubscriptions:   {},
		Transformations:         {},
		HADeployment:            {},
		WebhookAnalytics:        {},
		MutualTLS:               {},
		AsynqMonitoring:         {},
		SynchronousWebhooks:     {},
	}}
	require.True(t, trueLicenser.CanUseForwardProxy())
	require.True(t, trueLicenser.CanExportPrometheusMetrics())
	require.True(t, trueLicenser.AdvancedEndpointMgmt())
	require.True(t, trueLicenser.AdvancedRetentionPolicy())
	require.True(t, trueLicenser.AdvancedMsgBroker())
	require.True(t, trueLicenser.AdvancedSubscriptions())
	require.True(t, trueLicenser.Transformations())
	require.True(t, trueLicenser.HADeployment())
	require.True(t, trueLicenser.WebhookAnalytics())
	require.True(t, trueLicenser.MutualTLS())
	require.True(t, trueLicenser.AsynqMonitoring())
	require.True(t, trueLicenser.SynchronousWebhooks())

	falseLicenser := KeygenLicenser{featureList: map[Feature]Properties{}}

	require.False(t, falseLicenser.CanUseForwardProxy())
	require.False(t, falseLicenser.CanExportPrometheusMetrics())
	require.False(t, falseLicenser.AdvancedEndpointMgmt())
	require.False(t, falseLicenser.AdvancedRetentionPolicy())
	require.False(t, falseLicenser.AdvancedMsgBroker())
	require.False(t, falseLicenser.AdvancedSubscriptions())
	require.False(t, falseLicenser.Transformations())
	require.False(t, falseLicenser.HADeployment())
	require.False(t, falseLicenser.WebhookAnalytics())
	require.False(t, falseLicenser.MutualTLS())
	require.False(t, falseLicenser.AsynqMonitoring())
	require.False(t, falseLicenser.SynchronousWebhooks())
}

func provideLicenser(ctrl *gomock.Controller, fl map[Feature]Properties) *KeygenLicenser {
	return &KeygenLicenser{
		featureList:   fl,
		orgRepo:       mocks.NewMockOrganisationRepository(ctrl),
		orgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
	}
}

func TestKeygenLicenser_CanCreateOrg(t *testing.T) {
	tests := []struct {
		name        string
		featureList map[Feature]Properties
		ctx         context.Context
		dbFn        func(k *KeygenLicenser)
		want        bool
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name: "should_return_true",
			featureList: map[Feature]Properties{
				CreateOrg: {
					Limit: 1,
				},
			},
			dbFn: func(k *KeygenLicenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(0), nil)
			},
			ctx:     context.Background(),
			want:    true,
			wantErr: false,
		},
		{
			name: "should_return_false_for_limit_reached",
			featureList: map[Feature]Properties{
				CreateOrg: {
					Limit: 1,
				},
			},
			dbFn: func(k *KeygenLicenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(1), nil)
			},
			ctx:     context.Background(),
			want:    false,
			wantErr: false,
		},
		{
			name: "should_return_true_for_no_limit",
			featureList: map[Feature]Properties{
				CreateOrg: {
					Limit: -1,
				},
			},
			dbFn: func(k *KeygenLicenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(math.MaxInt64), nil)
			},
			ctx:     context.Background(),
			want:    true,
			wantErr: false,
		},
		{
			name: "should_error_for_failed_to_count_org",
			featureList: map[Feature]Properties{
				CreateOrg: {
					Limit: 1,
				},
			},
			dbFn: func(k *KeygenLicenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(0), errors.New("failed"))
			},
			ctx:        context.Background(),
			want:       false,
			wantErr:    true,
			wantErrMsg: "failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			k := provideLicenser(ctrl, tt.featureList)

			if tt.dbFn != nil {
				tt.dbFn(k)
			}

			got, err := k.CanCreateOrg(tt.ctx)
			require.Equal(t, tt.want, got)

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestKeygenLicenser_CanCreateOrgMember(t *testing.T) {
	tests := []struct {
		name            string
		featureList     map[Feature]Properties
		ctx             context.Context
		dbFn            func(k *KeygenLicenser)
		canCreateMember bool
		wantErr         bool
		wantErrMsg      string
	}{
		{
			name: "should_return_true",
			featureList: map[Feature]Properties{
				CreateOrgMember: {
					Limit: 1,
				},
			},
			dbFn: func(k *KeygenLicenser) {
				orgRepo := k.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				orgRepo.EXPECT().CountOrganisationMembers(gomock.Any()).Return(int64(0), nil)
			},
			ctx:             context.Background(),
			canCreateMember: true,
			wantErr:         false,
		},
		{
			name: "should_return_false_for_limit_reached",
			featureList: map[Feature]Properties{
				CreateOrgMember: {
					Limit: 1,
				},
			},
			dbFn: func(k *KeygenLicenser) {
				orgRepo := k.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				orgRepo.EXPECT().CountOrganisationMembers(gomock.Any()).Return(int64(1), nil)
			},
			ctx:             context.Background(),
			canCreateMember: false,
			wantErr:         false,
		},
		{
			name: "should_return_true_for_no_limit",
			featureList: map[Feature]Properties{
				CreateOrgMember: {
					Limit: -1,
				},
			},
			dbFn: func(k *KeygenLicenser) {
				orgRepo := k.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				orgRepo.EXPECT().CountOrganisationMembers(gomock.Any()).Return(int64(0), nil)
			},
			ctx:             context.Background(),
			canCreateMember: true,
			wantErr:         false,
		},
		{
			name: "should_error_for_failed_to_count_org_members",
			featureList: map[Feature]Properties{
				CreateOrgMember: {
					Limit: 1,
				},
			},
			dbFn: func(k *KeygenLicenser) {
				orgRepo := k.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				orgRepo.EXPECT().CountOrganisationMembers(gomock.Any()).Return(int64(0), errors.New("failed"))
			},
			ctx:             context.Background(),
			canCreateMember: false,
			wantErr:         true,
			wantErrMsg:      "failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			k := provideLicenser(ctrl, tt.featureList)

			if tt.dbFn != nil {
				tt.dbFn(k)
			}

			got, err := k.CanCreateOrgMember(tt.ctx)
			require.Equal(t, tt.canCreateMember, got)

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.NoError(t, err)
		})
	}
}
