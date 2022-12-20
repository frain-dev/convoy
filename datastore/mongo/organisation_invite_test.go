//go:build integration
// +build integration

package mongo

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestLoadOrganisationsInvitesPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	store := getStore(db)

	inviteRepo := NewOrgInviteRepo(store)
	org := &datastore.Organisation{
		UID:       uuid.NewString(),
		Name:      "test_org",
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := NewOrgRepo(store).CreateOrganisation(context.Background(), org)
	require.NoError(t, err)

	uids := []string{}
	for i := 1; i < 3; i++ {
		iv := &datastore.OrganisationInvite{
			UID:            uuid.NewString(),
			InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
			Token:          uuid.NewString(),
			OrganisationID: org.UID,
			Role:           auth.Role{Type: auth.RoleAdmin},
			Status:         datastore.InviteStatusPending,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}
		uids = append(uids, iv.UID)
		err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
		require.NoError(t, err)
	}

	for i := 1; i < 3; i++ {
		iv := &datastore.OrganisationInvite{
			UID:            uuid.NewString(),
			InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
			Token:          uuid.NewString(),
			OrganisationID: org.UID,
			Role:           auth.Role{Type: auth.RoleAdmin},
			Status:         datastore.InviteStatusDeclined,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
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
	store := getStore(db)

	inviteRepo := NewOrgInviteRepo(store)
	iv := &datastore.OrganisationInvite{
		UID:          uuid.NewString(),
		InviteeEmail: fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:        uuid.NewString(),
		Role:         auth.Role{Type: auth.RoleAdmin},
		Status:       datastore.InviteStatusPending,
		CreatedAt:    primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:    primitive.NewDateTimeFromTime(time.Now()),
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
	store := getStore(db)

	inviteRepo := NewOrgInviteRepo(store)
	iv := &datastore.OrganisationInvite{
		UID:          uuid.NewString(),
		InviteeEmail: fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:        uuid.NewString(),
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: uuid.NewString(),
		},
		Status:    datastore.InviteStatusPending,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	role := auth.Role{
		Type:     auth.RoleSuperUser,
		Project:  uuid.NewString(),
		Endpoint: "",
	}
	status := datastore.InviteStatusAccepted
	updatedAt := primitive.NewDateTimeFromTime(time.Now())

	iv.Role = role
	iv.Status = status
	iv.UpdatedAt = updatedAt

	err = inviteRepo.UpdateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByID(context.Background(), iv.UID)
	require.NoError(t, err)

	require.Equal(t, invite.UID, iv.UID)
	require.Equal(t, invite.Role, role)
	require.Equal(t, invite.UpdatedAt, updatedAt)
	require.Equal(t, invite.Status, status)
}

func TestDeleteOrganisationInvite(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	store := getStore(db)

	inviteRepo := NewOrgInviteRepo(store)
	org := &datastore.OrganisationInvite{
		UID:          uuid.NewString(),
		InviteeEmail: fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:        uuid.NewString(),
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: uuid.NewString(),
		},
		Status:    datastore.InviteStatusPending,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), org)
	require.NoError(t, err)

	err = inviteRepo.DeleteOrganisationInvite(context.Background(), org.UID)
	require.NoError(t, err)

	_, err = inviteRepo.FetchOrganisationInviteByID(context.Background(), org.UID)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)
}

func TestFetchOrganisationInviteByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	store := getStore(db)

	inviteRepo := NewOrgInviteRepo(store)
	iv := &datastore.OrganisationInvite{
		UID:          uuid.NewString(),
		InviteeEmail: fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:        uuid.NewString(),
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: uuid.NewString(),
		},
		Status:    datastore.InviteStatusPending,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByID(context.Background(), iv.UID)
	require.NoError(t, err)

	require.Equal(t, iv.UID, invite.UID)
	require.Equal(t, iv.Token, invite.Token)
	require.Equal(t, iv.InviteeEmail, invite.InviteeEmail)
}

func TestFetchOrganisationInviteByTokenAndEmail(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	store := getStore(db)

	inviteRepo := NewOrgInviteRepo(store)
	iv := &datastore.OrganisationInvite{
		UID:          uuid.NewString(),
		InviteeEmail: fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:        uuid.NewString(),
		Role: auth.Role{
			Type:    auth.RoleAdmin,
			Project: uuid.NewString(),
		},
		Status:    datastore.InviteStatusPending,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err := inviteRepo.CreateOrganisationInvite(context.Background(), iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByToken(context.Background(), iv.Token)
	require.NoError(t, err)

	require.Equal(t, iv.UID, invite.UID)
	require.Equal(t, iv.Token, invite.Token)
	require.Equal(t, iv.InviteeEmail, invite.InviteeEmail)
}
