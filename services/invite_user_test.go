package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
)

func TestInviteUserService(t *testing.T) {
	type args struct {
		inviteRepo    datastore.OrganisationInviteRepository
		orgMemberRepo datastore.OrganisationMemberRepository
		queue         queue.Queuer
		Licenser      license.Licenser
	}

	dbErr := errors.New("failed to create invite")

	tests := []struct {
		name         string
		err          error
		inviteeEmail string
		mockDep      func(args, *mocks.MockLogger)
		role         auth.Role
		user         *datastore.User
		organisation *datastore.Organisation
	}{
		{
			name:         "should_invite_user_successfully",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{},
			mockDep: func(a args, ml *mocks.MockLogger) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CheckUserLimit(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(true, nil)

				ivRepo, _ := a.inviteRepo.(*mocks.MockOrganisationInviteRepository)
				ivRepo.EXPECT().CreateOrganisationInvite(
					gomock.Any(),
					gomock.Any(),
				)

				q, _ := a.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
		},
		{
			name:         "should_fail_to_invite_user",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{},
			err:          dbErr,
			mockDep: func(a args, ml *mocks.MockLogger) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CheckUserLimit(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(true, nil)

				ivRepo, _ := a.inviteRepo.(*mocks.MockOrganisationInviteRepository)
				ivRepo.EXPECT().CreateOrganisationInvite(
					gomock.Any(),
					gomock.Any(),
				).Return(dbErr)

				ml.EXPECT().ErrorContext(gomock.Any(), "failed to invite member", "error", gomock.Any()).Times(1)
			},
		},
		{
			name:         "should_fail_to_invite_user_for_license_check",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{},
			err:          ErrUserLimit,
			mockDep: func(a args, ml *mocks.MockLogger) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CheckUserLimit(gomock.Any()).Times(1).Return(false, nil)
			},
		},
		{
			name:         "should_reject_invite_when_trial_org_at_user_cap",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{UID: "inv-org", LicenseData: encryptTrialLicense(t, "inv-org", map[string]interface{}{"user_limit": int64(1)})},
			err:          ErrOrgUserLimit,
			mockDep: func(a args, ml *mocks.MockLogger) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CheckUserLimit(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(true, nil)

				om, _ := a.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CountOrganisationMembers(gomock.Any(), "inv-org").Times(1).Return(int64(1), nil)

				// Members alone already fill the cap; pending invites are still counted but
				// the org is rejected regardless. CreateOrganisationInvite must not be called.
				ivRepo, _ := a.inviteRepo.(*mocks.MockOrganisationInviteRepository)
				ivRepo.EXPECT().CountOrganisationInvites(gomock.Any(), "inv-org", datastore.InviteStatusPending).Times(1).Return(int64(0), nil)
			},
		},
		{
			name:         "should_reject_invite_when_pending_invite_reaches_cap",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{UID: "inv-org", LicenseData: encryptTrialLicense(t, "inv-org", map[string]interface{}{"user_limit": int64(2)})},
			err:          ErrOrgUserLimit,
			mockDep: func(a args, ml *mocks.MockLogger) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CheckUserLimit(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(true, nil)

				om, _ := a.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CountOrganisationMembers(gomock.Any(), "inv-org").Times(1).Return(int64(1), nil)

				ivRepo, _ := a.inviteRepo.(*mocks.MockOrganisationInviteRepository)
				ivRepo.EXPECT().CountOrganisationInvites(gomock.Any(), "inv-org", datastore.InviteStatusPending).Times(1).Return(int64(1), nil)
			},
		},
		{
			name:         "should_invite_when_trial_org_under_user_cap",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{UID: "inv-org", LicenseData: encryptTrialLicense(t, "inv-org", map[string]interface{}{"user_limit": int64(3)})},
			mockDep: func(a args, ml *mocks.MockLogger) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CheckUserLimit(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().IsMultiUserMode(gomock.Any()).Times(1).Return(true, nil)

				om, _ := a.orgMemberRepo.(*mocks.MockOrganisationMemberRepository)
				om.EXPECT().CountOrganisationMembers(gomock.Any(), "inv-org").Times(1).Return(int64(1), nil)

				ivRepo, _ := a.inviteRepo.(*mocks.MockOrganisationInviteRepository)
				ivRepo.EXPECT().CountOrganisationInvites(gomock.Any(), "inv-org", datastore.InviteStatusPending).Times(1).Return(int64(1), nil)
				ivRepo.EXPECT().CreateOrganisationInvite(gomock.Any(), gomock.Any())

				q, _ := a.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-convoy.json")
			require.NoError(t, err)

			args := args{
				inviteRepo:    mocks.NewMockOrganisationInviteRepository(ctrl),
				orgMemberRepo: mocks.NewMockOrganisationMemberRepository(ctrl),
				queue:         mocks.NewMockQueuer(ctrl),
				Licenser:      mocks.NewMockLicenser(ctrl),
			}

			ml := mocks.NewMockLogger(ctrl)

			if tt.mockDep != nil {
				tt.mockDep(args, ml)
			}

			inviteService := NewInviteUserService(
				args.queue,
				args.inviteRepo,
				args.orgMemberRepo,
				tt.inviteeEmail,
				tt.role,
				tt.user,
				tt.organisation,
				args.Licenser,
				ml,
			)

			// Act.
			iv, err := inviteService.Run(context.Background())

			// Assert.
			if tt.err != nil {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.inviteeEmail, iv.InviteeEmail)
		})
	}
}
