package keygen

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/frain-dev/convoy/mocks"
	"github.com/keygen-sh/keygen-go/v3"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"
)

func TestKeygenLicenserBoolMethods(t *testing.T) {
	k := Licenser{featureList: map[Feature]*Properties{UseForwardProxy: {}}, license: &keygen.License{}}
	require.True(t, k.UseForwardProxy())

	k = Licenser{featureList: map[Feature]*Properties{UseForwardProxy: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.UseForwardProxy())

	k = Licenser{featureList: map[Feature]*Properties{ExportPrometheusMetrics: {}}, license: &keygen.License{}}
	require.True(t, k.CanExportPrometheusMetrics())

	k = Licenser{featureList: map[Feature]*Properties{ExportPrometheusMetrics: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.CanExportPrometheusMetrics())

	k = Licenser{featureList: map[Feature]*Properties{AdvancedEndpointMgmt: {}}, license: &keygen.License{}}
	require.True(t, k.AdvancedEndpointMgmt())

	k = Licenser{featureList: map[Feature]*Properties{AdvancedEndpointMgmt: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.AdvancedEndpointMgmt())

	k = Licenser{featureList: map[Feature]*Properties{AdvancedWebhookArchiving: {}}, license: &keygen.License{}}
	require.True(t, k.AdvancedRetentionPolicy())

	k = Licenser{featureList: map[Feature]*Properties{AdvancedWebhookArchiving: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.AdvancedRetentionPolicy())

	k = Licenser{featureList: map[Feature]*Properties{AdvancedMsgBroker: {}}, license: &keygen.License{}}
	require.True(t, k.AdvancedMsgBroker())
	k = Licenser{featureList: map[Feature]*Properties{AdvancedMsgBroker: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.AdvancedMsgBroker())

	k = Licenser{featureList: map[Feature]*Properties{AdvancedSubscriptions: {}}, license: &keygen.License{}}
	require.True(t, k.AdvancedSubscriptions())
	k = Licenser{featureList: map[Feature]*Properties{AdvancedSubscriptions: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.AdvancedSubscriptions())

	k = Licenser{featureList: map[Feature]*Properties{WebhookTransformations: {}}, license: &keygen.License{}}
	require.True(t, k.Transformations())
	k = Licenser{featureList: map[Feature]*Properties{WebhookTransformations: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.Transformations())

	k = Licenser{featureList: map[Feature]*Properties{HADeployment: {}}, license: &keygen.License{}}
	require.True(t, k.HADeployment())
	k = Licenser{featureList: map[Feature]*Properties{HADeployment: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.HADeployment())

	k = Licenser{featureList: map[Feature]*Properties{WebhookAnalytics: {}}, license: &keygen.License{}}
	require.True(t, k.WebhookAnalytics())
	k = Licenser{featureList: map[Feature]*Properties{WebhookAnalytics: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.WebhookAnalytics())

	k = Licenser{featureList: map[Feature]*Properties{MutualTLS: {}}, license: &keygen.License{}}
	require.True(t, k.MutualTLS())
	k = Licenser{featureList: map[Feature]*Properties{MutualTLS: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.MutualTLS())

	k = Licenser{featureList: map[Feature]*Properties{AsynqMonitoring: {}}, license: &keygen.License{}}
	require.True(t, k.AsynqMonitoring())

	k = Licenser{featureList: map[Feature]*Properties{AsynqMonitoring: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.AsynqMonitoring())

	k = Licenser{featureList: map[Feature]*Properties{SynchronousWebhooks: {}}, license: &keygen.License{}}
	require.True(t, k.SynchronousWebhooks())
	k = Licenser{featureList: map[Feature]*Properties{SynchronousWebhooks: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.SynchronousWebhooks())

	k = Licenser{featureList: map[Feature]*Properties{PortalLinks: {}}, license: &keygen.License{}}
	require.True(t, k.PortalLinks())
	k = Licenser{featureList: map[Feature]*Properties{PortalLinks: {}}, license: &keygen.License{Expiry: timePtr(time.Now().Add(-400000 * time.Hour))}}
	require.False(t, k.PortalLinks())

	k = Licenser{enabledProjects: map[string]bool{
		"12345": true,
	}}
	require.True(t, k.ProjectEnabled("12345"))
	require.False(t, k.ProjectEnabled("5555"))

	// when k.enabledProjects is nil, should not add anything
	k = Licenser{}
	k.AddEnabledProject("11111")
	require.False(t, k.enabledProjects["11111"])

	k = Licenser{enabledProjects: map[string]bool{}}
	k.AddEnabledProject("11111")
	require.True(t, k.enabledProjects["11111"])

	k = Licenser{enabledProjects: map[string]bool{"11111": true, "2222": true}}
	k.RemoveEnabledProject("11111")
	require.NotContains(t, k.enabledProjects, "11111")
	require.Contains(t, k.enabledProjects, "2222")

	falseLicenser := Licenser{featureList: map[Feature]*Properties{}, license: &keygen.License{Expiry: timePtr(time.Now().Add(400000 * time.Hour))}}

	require.False(t, falseLicenser.UseForwardProxy())
	require.False(t, falseLicenser.PortalLinks())
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

func provideLicenser(ctrl *gomock.Controller, license *keygen.License, fl map[Feature]*Properties) *Licenser {
	return &Licenser{
		featureList: fl,
		license:     license,
		orgRepo:     mocks.NewMockOrganisationRepository(ctrl),
		userRepo:    mocks.NewMockUserRepository(ctrl),
		projectRepo: mocks.NewMockProjectRepository(ctrl),
	}
}

func TestKeygenLicenser_CreateProject(t *testing.T) {
	tests := []struct {
		name        string
		featureList map[Feature]*Properties
		ctx         context.Context
		dbFn        func(k *Licenser)
		license     *keygen.License
		want        bool
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name: "should_return_true",
			featureList: map[Feature]*Properties{
				CreateProject: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				projectRepo := k.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(0), nil)
			},
			ctx:     context.Background(),
			want:    true,
			wantErr: false,
		},
		{
			name: "should_return_false_for_license_expired",
			featureList: map[Feature]*Properties{
				CreateProject: {
					Limit: 1,
				},
			},
			license:    &keygen.License{Expiry: timePtr(time.Now().Add(-time.Hour * 40000))},
			ctx:        context.Background(),
			want:       false,
			wantErr:    true,
			wantErrMsg: ErrLicenseExpired.Error(),
		},
		{
			name: "should_return_false_for_limit_reached",
			featureList: map[Feature]*Properties{
				CreateProject: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				projectRepo := k.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(1), nil)
			},
			ctx:     context.Background(),
			want:    false,
			wantErr: false,
		},
		{
			name: "should_return_true_for_no_limit",
			featureList: map[Feature]*Properties{
				CreateProject: {
					Limit: -1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				projectRepo := k.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(math.MaxInt64), nil)
			},
			ctx:     context.Background(),
			want:    true,
			wantErr: false,
		},
		{
			name: "should_error_for_failed_to_count_org",
			featureList: map[Feature]*Properties{
				CreateProject: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				projectRepo := k.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(0), errors.New("failed"))
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
			defer ctrl.Finish()
			k := provideLicenser(ctrl, tt.license, tt.featureList)

			if tt.dbFn != nil {
				tt.dbFn(k)
			}

			got, err := k.CreateProject(tt.ctx)
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

func TestKeygenLicenser_CanCreateOrg(t *testing.T) {
	tests := []struct {
		name        string
		featureList map[Feature]*Properties
		ctx         context.Context
		dbFn        func(k *Licenser)
		license     *keygen.License
		want        bool
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name: "should_return_true",
			featureList: map[Feature]*Properties{
				CreateOrg: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(0), nil)
			},
			ctx:     context.Background(),
			want:    true,
			wantErr: false,
		},
		{
			name: "should_return_false_for_license_expired",
			featureList: map[Feature]*Properties{
				CreateOrg: {
					Limit: 1,
				},
			},
			license:    &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * -40000))},
			ctx:        context.Background(),
			want:       false,
			wantErr:    true,
			wantErrMsg: ErrLicenseExpired.Error(),
		},
		{
			name: "should_return_false_for_limit_reached",
			featureList: map[Feature]*Properties{
				CreateOrg: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(1), nil)
			},
			ctx:     context.Background(),
			want:    false,
			wantErr: false,
		},
		{
			name: "should_return_true_for_no_limit",
			featureList: map[Feature]*Properties{
				CreateOrg: {
					Limit: -1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(math.MaxInt64), nil)
			},
			ctx:     context.Background(),
			want:    true,
			wantErr: false,
		},
		{
			name: "should_error_for_failed_to_count_org",
			featureList: map[Feature]*Properties{
				CreateOrg: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
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
			defer ctrl.Finish()
			k := provideLicenser(ctrl, tt.license, tt.featureList)

			if tt.dbFn != nil {
				tt.dbFn(k)
			}

			got, err := k.CreateOrg(tt.ctx)
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

func TestKeygenLicenser_CanCreateUser(t *testing.T) {
	tests := []struct {
		name            string
		featureList     map[Feature]*Properties
		ctx             context.Context
		license         *keygen.License
		dbFn            func(k *Licenser)
		canCreateMember bool
		wantErr         bool
		wantErrMsg      string
	}{
		{
			name: "should_return_true",
			featureList: map[Feature]*Properties{
				CreateUser: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				userRepository := k.userRepo.(*mocks.MockUserRepository)
				userRepository.EXPECT().CountUsers(gomock.Any()).Return(int64(0), nil)
			},
			ctx:             context.Background(),
			canCreateMember: true,
			wantErr:         false,
		},
		{
			name: "should_return_false_for_limit_reached",
			featureList: map[Feature]*Properties{
				CreateUser: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				userRepository := k.userRepo.(*mocks.MockUserRepository)
				userRepository.EXPECT().CountUsers(gomock.Any()).Return(int64(1), nil)
			},
			ctx:             context.Background(),
			canCreateMember: false,
			wantErr:         false,
		},
		{
			name: "should_return_true_for_no_limit",
			featureList: map[Feature]*Properties{
				CreateUser: {
					Limit: -1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				userRepository := k.userRepo.(*mocks.MockUserRepository)
				userRepository.EXPECT().CountUsers(gomock.Any()).Return(int64(0), nil)
			},
			ctx:             context.Background(),
			canCreateMember: true,
			wantErr:         false,
		},
		{
			name: "should_error_for_failed_to_count_org_members",
			featureList: map[Feature]*Properties{
				CreateUser: {
					Limit: 1,
				},
			},
			license: &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * 40000))},
			dbFn: func(k *Licenser) {
				userRepository := k.userRepo.(*mocks.MockUserRepository)
				userRepository.EXPECT().CountUsers(gomock.Any()).Return(int64(0), errors.New("failed"))
			},
			ctx:             context.Background(),
			canCreateMember: false,
			wantErr:         true,
			wantErrMsg:      "failed",
		},
		{
			name: "should_error_for_license_expired",
			featureList: map[Feature]*Properties{
				CreateUser: {
					Limit: 1,
				},
			},
			license:         &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour * -40000))},
			ctx:             context.Background(),
			canCreateMember: false,
			wantErr:         true,
			wantErrMsg:      ErrLicenseExpired.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			k := provideLicenser(ctrl, tt.license, tt.featureList)

			if tt.dbFn != nil {
				tt.dbFn(k)
			}

			got, err := k.CreateUser(tt.ctx)
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

func TestLicenser_FeatureListJSON(t *testing.T) {
	tests := []struct {
		name        string
		featureList map[Feature]*Properties
		dbFn        func(k *Licenser)
		want        json.RawMessage
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name: "should_get_feature_list",
			featureList: map[Feature]*Properties{
				CreateOrg:             {Limit: 1},
				CreateProject:         {Limit: 1},
				CreateUser:            {Limit: 1},
				AdvancedSubscriptions: {Limit: 1, Allowed: true},
				AdvancedEndpointMgmt:  {Limit: 1, Allowed: true},
			},
			dbFn: func(k *Licenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(0), nil)

				userRepo := k.userRepo.(*mocks.MockUserRepository)
				userRepo.EXPECT().CountUsers(gomock.Any()).Return(int64(0), nil)

				projectRepo := k.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(0), nil)
			},
			want:       []byte(`{"ADVANCED_ENDPOINT_MANAGEMENT":{"allowed":true},"ADVANCED_SUBSCRIPTIONS":{"allowed":true},"CREATE_ORG":{"allowed":true},"CREATE_PROJECT":{"allowed":true},"CREATE_USER":{"allowed":true}}`),
			wantErr:    false,
			wantErrMsg: "",
		},

		{
			name: "should_be_false_create_org",
			featureList: map[Feature]*Properties{
				CreateOrg:             {Limit: 1},
				CreateProject:         {Limit: 1},
				CreateUser:            {Limit: 1},
				AdvancedSubscriptions: {Limit: 1, Allowed: true},
				AdvancedEndpointMgmt:  {Limit: 1, Allowed: true},
			},
			dbFn: func(k *Licenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(1), nil)

				userRepo := k.userRepo.(*mocks.MockUserRepository)
				userRepo.EXPECT().CountUsers(gomock.Any()).Return(int64(0), nil)

				projectRepo := k.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(0), nil)
			},
			want:       []byte(`{"ADVANCED_ENDPOINT_MANAGEMENT":{"allowed":true},"ADVANCED_SUBSCRIPTIONS":{"allowed":true},"CREATE_ORG":{"allowed":false},"CREATE_PROJECT":{"allowed":true},"CREATE_USER":{"allowed":true}}`),
			wantErr:    false,
			wantErrMsg: "",
		},

		{
			name: "should_be_false_create_user",
			featureList: map[Feature]*Properties{
				CreateOrg:             {Limit: 1},
				CreateProject:         {Limit: 1},
				CreateUser:            {Limit: 1},
				AdvancedSubscriptions: {Limit: 1, Allowed: true},
				AdvancedEndpointMgmt:  {Limit: 1, Allowed: true},
			},
			dbFn: func(k *Licenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(0), nil)

				userRepo := k.userRepo.(*mocks.MockUserRepository)
				userRepo.EXPECT().CountUsers(gomock.Any()).Return(int64(1), nil)

				projectRepo := k.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(0), nil)
			},
			want:       []byte(`{"ADVANCED_ENDPOINT_MANAGEMENT":{"allowed":true},"ADVANCED_SUBSCRIPTIONS":{"allowed":true},"CREATE_ORG":{"allowed":true},"CREATE_PROJECT":{"allowed":true},"CREATE_USER":{"allowed":false}}`),
			wantErr:    false,
			wantErrMsg: "",
		},

		{
			name: "should_be_false_create_project",
			featureList: map[Feature]*Properties{
				CreateOrg:             {Limit: 1},
				CreateProject:         {Limit: 1},
				CreateUser:            {Limit: 1},
				AdvancedSubscriptions: {Limit: 1, Allowed: true},
				AdvancedEndpointMgmt:  {Limit: 1, Allowed: true},
			},
			dbFn: func(k *Licenser) {
				orgRepo := k.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().CountOrganisations(gomock.Any()).Return(int64(0), nil)

				userRepo := k.userRepo.(*mocks.MockUserRepository)
				userRepo.EXPECT().CountUsers(gomock.Any()).Return(int64(0), nil)

				projectRepo := k.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().CountProjects(gomock.Any()).Return(int64(2), nil)
			},
			want:       []byte(`{"ADVANCED_ENDPOINT_MANAGEMENT":{"allowed":true},"ADVANCED_SUBSCRIPTIONS":{"allowed":true},"CREATE_ORG":{"allowed":true},"CREATE_PROJECT":{"allowed":false},"CREATE_USER":{"allowed":true}}`),
			wantErr:    false,
			wantErrMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			k := provideLicenser(ctrl, &keygen.License{Expiry: timePtr(time.Now().Add(time.Hour))}, tt.featureList)

			if tt.dbFn != nil {
				tt.dbFn(k)
			}

			got, err := k.FeatureListJSON(context.Background())
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCheckExpiry(t *testing.T) {
	tests := []struct {
		name       string
		expiry     *time.Time
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "No Expiry Date",
			expiry:     nil,
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name:    "License Expired within 21 Days",
			expiry:  timePtr(time.Now().Add(-10 * 24 * time.Hour)), // 10 days ago
			wantErr: false,
		},
		{
			name:       "License Expired beyond 21 Days",
			expiry:     timePtr(time.Now().Add(-22 * 24 * time.Hour)), // 22 days ago
			wantErr:    true,
			wantErrMsg: ErrLicenseExpired.Error(),
		},
		{
			name:    "License Not Yet Expired",
			expiry:  timePtr(time.Now().Add(5 * 24 * time.Hour)), // 5 days in the future
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			license := &keygen.License{
				Expiry: tt.expiry,
			}

			err := checkExpiry(license)

			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

// timePtr is a helper function to get a pointer to a time.Time value
func timePtr(t time.Time) *time.Time {
	return &t
}
