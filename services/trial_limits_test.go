package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func encryptTrialLicense(t *testing.T, orgID string, entitlements map[string]interface{}) string {
	t.Helper()
	enc, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{Key: "lk", Entitlements: entitlements})
	require.NoError(t, err)
	return enc
}

func TestCheckOrganisationUserLimit(t *testing.T) {
	orgID := "org-user-limit"

	tests := []struct {
		name                string
		licenseData         string
		countPendingInvites bool
		dbFn                func(m *mocks.MockOrganisationMemberRepository, iv *mocks.MockOrganisationInviteRepository)
		wantAllowed         bool
	}{
		{
			name:        "no license data fails open, no counting",
			licenseData: "",
			wantAllowed: true,
		},
		{
			name:        "unlimited cap fails open, no counting",
			licenseData: encryptTrialLicense(t, orgID, map[string]interface{}{"user_limit": int64(-1)}),
			wantAllowed: true,
		},
		{
			name:        "trial cap reached rejects (members only)",
			licenseData: encryptTrialLicense(t, orgID, map[string]interface{}{"user_limit": int64(1)}),
			dbFn: func(m *mocks.MockOrganisationMemberRepository, _ *mocks.MockOrganisationInviteRepository) {
				m.EXPECT().CountOrganisationMembers(gomock.Any(), orgID).Return(int64(1), nil)
			},
			wantAllowed: false,
		},
		{
			name:        "trial cap under limit allowed (members only)",
			licenseData: encryptTrialLicense(t, orgID, map[string]interface{}{"user_limit": int64(2)}),
			dbFn: func(m *mocks.MockOrganisationMemberRepository, _ *mocks.MockOrganisationInviteRepository) {
				m.EXPECT().CountOrganisationMembers(gomock.Any(), orgID).Return(int64(1), nil)
			},
			wantAllowed: true,
		},
		{
			name:                "pending invite pushes org to cap, rejected",
			licenseData:         encryptTrialLicense(t, orgID, map[string]interface{}{"user_limit": int64(2)}),
			countPendingInvites: true,
			dbFn: func(m *mocks.MockOrganisationMemberRepository, iv *mocks.MockOrganisationInviteRepository) {
				m.EXPECT().CountOrganisationMembers(gomock.Any(), orgID).Return(int64(1), nil)
				iv.EXPECT().CountOrganisationInvites(gomock.Any(), orgID, datastore.InviteStatusPending).Return(int64(1), nil)
			},
			wantAllowed: false,
		},
		{
			name:                "pending invites counted but still under cap, allowed",
			licenseData:         encryptTrialLicense(t, orgID, map[string]interface{}{"user_limit": int64(3)}),
			countPendingInvites: true,
			dbFn: func(m *mocks.MockOrganisationMemberRepository, iv *mocks.MockOrganisationInviteRepository) {
				m.EXPECT().CountOrganisationMembers(gomock.Any(), orgID).Return(int64(1), nil)
				iv.EXPECT().CountOrganisationInvites(gomock.Any(), orgID, datastore.InviteStatusPending).Return(int64(1), nil)
			},
			wantAllowed: true,
		},
		{
			name:        "member count lookup error fails open",
			licenseData: encryptTrialLicense(t, orgID, map[string]interface{}{"user_limit": int64(1)}),
			dbFn: func(m *mocks.MockOrganisationMemberRepository, _ *mocks.MockOrganisationInviteRepository) {
				m.EXPECT().CountOrganisationMembers(gomock.Any(), orgID).Return(int64(0), errors.New("db down"))
			},
			wantAllowed: true,
		},
		{
			name:                "pending invite count error fails open",
			licenseData:         encryptTrialLicense(t, orgID, map[string]interface{}{"user_limit": int64(2)}),
			countPendingInvites: true,
			dbFn: func(m *mocks.MockOrganisationMemberRepository, iv *mocks.MockOrganisationInviteRepository) {
				m.EXPECT().CountOrganisationMembers(gomock.Any(), orgID).Return(int64(1), nil)
				iv.EXPECT().CountOrganisationInvites(gomock.Any(), orgID, datastore.InviteStatusPending).Return(int64(0), errors.New("db down"))
			},
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			memberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
			inviteRepo := mocks.NewMockOrganisationInviteRepository(ctrl)
			if tt.dbFn != nil {
				tt.dbFn(memberRepo, inviteRepo)
			}

			org := &datastore.Organisation{UID: orgID, LicenseData: tt.licenseData}
			allowed, err := CheckOrganisationUserLimit(context.Background(), org, tt.countPendingInvites, OrgUserLimitDeps{
				OrgMemberRepo: memberRepo,
				InviteRepo:    inviteRepo,
				Logger:        log.New("convoy", log.LevelError),
			})
			require.NoError(t, err)
			require.Equal(t, tt.wantAllowed, allowed)
		})
	}
}

func TestCheckUserOrgCreationAllowed(t *testing.T) {
	userID := "user-1"
	orgA := "org-a"
	orgB := "org-b"

	tests := []struct {
		name        string
		dbFn        func(m *mocks.MockOrganisationMemberRepository)
		wantAllowed bool
	}{
		{
			name: "no finite cap on any org fails open, no count",
			dbFn: func(m *mocks.MockOrganisationMemberRepository) {
				m.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), userID, gomock.Any()).Return(
					[]datastore.Organisation{{UID: orgA, LicenseData: encryptTrialLicense(t, orgA, map[string]interface{}{"org_limit": int64(-1)})}},
					datastore.PaginationData{}, nil,
				)
			},
			wantAllowed: true,
		},
		{
			name: "trialing org at cap rejects",
			dbFn: func(m *mocks.MockOrganisationMemberRepository) {
				m.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), userID, gomock.Any()).Return(
					[]datastore.Organisation{{UID: orgA, LicenseData: encryptTrialLicense(t, orgA, map[string]interface{}{"org_limit": int64(1)})}},
					datastore.PaginationData{}, nil,
				)
				m.EXPECT().CountUserOrganisations(gomock.Any(), userID, "").Return(int64(1), nil)
			},
			wantAllowed: false,
		},
		{
			name: "trialing org under cap allowed",
			dbFn: func(m *mocks.MockOrganisationMemberRepository) {
				m.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), userID, gomock.Any()).Return(
					[]datastore.Organisation{{UID: orgA, LicenseData: encryptTrialLicense(t, orgA, map[string]interface{}{"org_limit": int64(2)})}},
					datastore.PaginationData{}, nil,
				)
				m.EXPECT().CountUserOrganisations(gomock.Any(), userID, "").Return(int64(1), nil)
			},
			wantAllowed: true,
		},
		{
			name: "smallest finite cap wins across orgs",
			dbFn: func(m *mocks.MockOrganisationMemberRepository) {
				m.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), userID, gomock.Any()).Return(
					[]datastore.Organisation{
						{UID: orgA, LicenseData: encryptTrialLicense(t, orgA, map[string]interface{}{"org_limit": int64(5)})},
						{UID: orgB, LicenseData: encryptTrialLicense(t, orgB, map[string]interface{}{"org_limit": int64(1)})},
					},
					datastore.PaginationData{}, nil,
				)
				m.EXPECT().CountUserOrganisations(gomock.Any(), userID, "").Return(int64(2), nil)
			},
			wantAllowed: false,
		},
		{
			name: "load orgs error fails open",
			dbFn: func(m *mocks.MockOrganisationMemberRepository) {
				m.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), userID, gomock.Any()).Return(nil, datastore.PaginationData{}, errors.New("db down"))
			},
			wantAllowed: true,
		},
		{
			name: "count error fails open",
			dbFn: func(m *mocks.MockOrganisationMemberRepository) {
				m.EXPECT().LoadUserOrganisationsPaged(gomock.Any(), userID, gomock.Any()).Return(
					[]datastore.Organisation{{UID: orgA, LicenseData: encryptTrialLicense(t, orgA, map[string]interface{}{"org_limit": int64(1)})}},
					datastore.PaginationData{}, nil,
				)
				m.EXPECT().CountUserOrganisations(gomock.Any(), userID, "").Return(int64(0), errors.New("db down"))
			},
			wantAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			memberRepo := mocks.NewMockOrganisationMemberRepository(ctrl)
			if tt.dbFn != nil {
				tt.dbFn(memberRepo)
			}

			user := &datastore.User{UID: userID}
			allowed, err := CheckUserOrgCreationAllowed(context.Background(), user, UserOrgLimitDeps{
				OrgMemberRepo: memberRepo,
				Logger:        log.New("convoy", log.LevelError),
			})
			require.NoError(t, err)
			require.Equal(t, tt.wantAllowed, allowed)
		})
	}
}
