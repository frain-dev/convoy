//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func TestLoadOrganisationsInvitesPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)
	project := seedProject(t, db)

	uids := []string{}
	for i := 1; i < 3; i++ {
		iv := &datastore.OrganisationInvite{
			UID:            ulid.Make().String(),
			InviteeEmail:   fmt.Sprintf("%s@gmail.com", ulid.Make().String()),
			Token:          ulid.Make().String(),
			OrganisationID: org.UID,
			Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
			Status:         datastore.InviteStatusPending,
		}
		uids = append(uids, iv.UID)
		err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
		require.NoError(t, err)
	}

	for i := 1; i < 3; i++ {
		iv := &datastore.OrganisationInvite{
			UID:            ulid.Make().String(),
			InviteeEmail:   fmt.Sprintf("%s@gmail.com", ulid.Make().String()),
			Token:          ulid.Make().String(),
			OrganisationID: org.UID,
			Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
			Status:         datastore.InviteStatusDeclined,
		}

		err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
		require.NoError(t, err)
	}

	organisationInvites, _, err := inviteRepo.LoadOrganisationsInvitesPaged(context.Background(), org.UID, datastore.InviteStatusPending, datastore.Pageable{
		PerPage: 100,
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

	project := seedProject(t, db)

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", ulid.Make().String()),
		Token:          ulid.Make().String(),
		OrganisationID: org.UID,
		Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
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
	project := seedProject(t, db)

	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", ulid.Make().String()),
		Token:          ulid.Make().String(),
		OrganisationID: org.UID,
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: project.UID,
		},
		Status: datastore.InviteStatusPending,
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	role := auth.Role{
		Type:     auth.RoleSuperUser,
		Project:  seedProject(t, db).UID,
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
	project := seedProject(t, db)

	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", ulid.Make().String()),
		Token:          ulid.Make().String(),
		OrganisationID: org.UID,
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: project.UID,
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

	project := seedProject(t, db)

	org := seedOrg(t, db)
	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", ulid.Make().String()),
		Token:          ulid.Make().String(),
		OrganisationID: org.UID,
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: project.UID,
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
	project := seedProject(t, db)

	inviteRepo := NewOrgInviteRepo(db)
	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", ulid.Make().String()),
		Token:          ulid.Make().String(),
		OrganisationID: org.UID,
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: project.UID,
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
