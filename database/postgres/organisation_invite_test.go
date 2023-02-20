//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLoadOrganisationsInvitesPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)

	uids := []string{}
	for i := 1; i < 3; i++ {
		iv := &datastore.OrganisationInvite{
			UID:            ulid.Make().String(),
			InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
			Token:          uuid.NewString(),
			OrganisationID: org.UID,
			Role:           auth.Role{Type: auth.RoleAdmin},
			Status:         datastore.InviteStatusPending,
		}
		uids = append(uids, iv.UID)
		err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
		require.NoError(t, err)
	}

	for i := 1; i < 3; i++ {
		iv := &datastore.OrganisationInvite{
			UID:            ulid.Make().String(),
			InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
			Token:          uuid.NewString(),
			OrganisationID: org.UID,
			Role:           auth.Role{Type: auth.RoleAdmin},
			Status:         datastore.InviteStatusDeclined,
		}

		err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
		require.NoError(t, err)
	}

	organisationInvites, _, err := inviteRepo.LoadOrganisationsInvitesPaged(context.Background(), org.UID, datastore.InviteStatusPending, datastore.Pageable{
		Page:    1,
		PerPage: 100,
		Sort:    -1,
	})

	require.NoError(t, err)
	require.Equal(t, 2, len(organisationInvites))
	for _, invite := range organisationInvites {
		require.Contains(t, uids, invite.UID)
	}
}

func TestCreateOrganisationInvite(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:          uuid.NewString(),
		OrganisationID: org.UID,
		Role:           auth.Role{Type: auth.RoleAdmin},
		Status:         datastore.InviteStatusPending,
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByID(context.Background(), iv.UID)
	require.NoError(t, err)

	require.Equal(t, iv.UID, invite.UID)
	require.Equal(t, iv.Token, invite.Token)
}

func TestUpdateOrganisationInvite(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:          uuid.NewString(),
		OrganisationID: org.UID,
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: uuid.NewString(),
		},
		Status: datastore.InviteStatusPending,
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	role := auth.Role{
		Type:     auth.RoleSuperUser,
		Project:  uuid.NewString(),
		Endpoint: "",
	}
	status := datastore.InviteStatusAccepted

	iv.Role = role
	iv.Status = status

	err = inviteRepo.UpdateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByID(context.Background(), iv.UID)
	require.NoError(t, err)

	require.Equal(t, invite.UID, iv.UID)
	require.Equal(t, invite.Role, role)
	require.Equal(t, invite.Status, status)
}

func TestDeleteOrganisationInvite(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:          uuid.NewString(),
		OrganisationID: org.UID,
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: uuid.NewString(),
		},
		Status: datastore.InviteStatusPending,
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	err = inviteRepo.DeleteOrganisationInvite(context.Background(), iv.UID)
	require.NoError(t, err)

	_, err = inviteRepo.FetchOrganisationInviteByID(context.Background(), iv.UID)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)
}

func TestFetchOrganisationInviteByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:          uuid.NewString(),
		OrganisationID: org.UID,
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: uuid.NewString(),
		},
		Status: datastore.InviteStatusPending,
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByID(context.Background(), iv.UID)
	require.NoError(t, err)

	require.Equal(t, iv.UID, invite.UID)
	require.Equal(t, iv.Token, invite.Token)
	require.Equal(t, iv.InviteeEmail, invite.InviteeEmail)
}

func TestFetchOrganisationInviteByToken(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:          uuid.NewString(),
		OrganisationID: org.UID,
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: uuid.NewString(),
		},
		Status: datastore.InviteStatusPending,
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByToken(context.Background(), iv.Token)
	require.NoError(t, err)

	require.Equal(t, iv.UID, invite.UID)
	require.Equal(t, iv.Token, invite.Token)
	require.Equal(t, iv.InviteeEmail, invite.InviteeEmail)
}

func seedOrg(t *testing.T, db *sqlx.DB) *datastore.Organisation {
	user := seedUser(t, db)

	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "test-org",
		OwnerID: user.UID,
	}

	err := NewOrgRepo(db).CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	return org
}
