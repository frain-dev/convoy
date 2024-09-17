package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestInviteUserService(t *testing.T) {
	type args struct {
		inviteRepo datastore.OrganisationInviteRepository
		queue      queue.Queuer
		Licenser   license.Licenser
	}

	dbErr := errors.New("failed to create invite")

	tests := []struct {
		name         string
		err          error
		inviteeEmail string
		mockDep      func(args)
		role         auth.Role
		user         *datastore.User
		organisation *datastore.Organisation
	}{
		{
			name:         "should_invite_user_successfully",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{},
			mockDep: func(a args) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateUser(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().MultiPlayerMode().Times(1).Return(true)

				ivRepo, _ := a.inviteRepo.(*mocks.MockOrganisationInviteRepository)
				ivRepo.EXPECT().CreateOrganisationInvite(
					gomock.Any(),
					gomock.Any(),
				)

				q, _ := a.queue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any())
			},
		},
		{
			name:         "should_fail_to_invite_user",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{},
			err:          dbErr,
			mockDep: func(a args) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateUser(gomock.Any()).Times(1).Return(true, nil)
				licenser.EXPECT().MultiPlayerMode().Times(1).Return(true)

				ivRepo, _ := a.inviteRepo.(*mocks.MockOrganisationInviteRepository)
				ivRepo.EXPECT().CreateOrganisationInvite(
					gomock.Any(),
					gomock.Any(),
				).Return(dbErr)
			},
		},
		{
			name:         "should_fail_to_invite_user_for_license_check",
			inviteeEmail: "sidemen@default.com",
			user:         &datastore.User{},
			organisation: &datastore.Organisation{},
			err:          ErrUserLimit,
			mockDep: func(a args) {
				licenser, _ := a.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().CreateUser(gomock.Any()).Times(1).Return(false, nil)
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
				inviteRepo: mocks.NewMockOrganisationInviteRepository(ctrl),
				queue:      mocks.NewMockQueuer(ctrl),
				Licenser:   mocks.NewMockLicenser(ctrl),
			}

			if tt.mockDep != nil {
				tt.mockDep(args)
			}

			inviteService := &InviteUserService{
				Queue:        args.queue,
				InviteRepo:   args.inviteRepo,
				InviteeEmail: tt.inviteeEmail,
				User:         tt.user,
				Organisation: tt.organisation,
				Licenser:     args.Licenser,
				Role:         tt.role,
			}

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
