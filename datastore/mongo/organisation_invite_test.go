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
		UID:            uuid.NewString(),
		Name:           "test_org",
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.OrganisationCollection)
	err := NewOrgRepo(store).CreateOrganisation(orgCtx, org)
	require.NoError(t, err)

	uids := []string{}
	orgInviteCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.OrganisationInvitesCollection)
	for i := 1; i < 3; i++ {
		iv := &datastore.OrganisationInvite{
			UID:            uuid.NewString(),
			InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
			Token:          uuid.NewString(),
			OrganisationID: org.UID,
			Role:           auth.Role{Type: auth.RoleAdmin},
			Status:         datastore.InviteStatusPending,
			DocumentStatus: datastore.ActiveDocumentStatus,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}
		uids = append(uids, iv.UID)
		err := inviteRepo.CreateOrganisationInvite(orgInviteCtx, iv)
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
			DocumentStatus: datastore.ActiveDocumentStatus,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}

		err := inviteRepo.CreateOrganisationInvite(orgInviteCtx, iv)
		require.NoError(t, err)
	}

	organisationInvites, _, err := inviteRepo.LoadOrganisationsInvitesPaged(orgInviteCtx, org.UID, datastore.InviteStatusPending, datastore.Pageable{
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
		UID:            uuid.NewString(),
		InviteeEmail:   fmt.Sprintf("%s@gmail.com", uuid.NewString()),
		Token:          uuid.NewString(),
		Role:           auth.Role{Type: auth.RoleAdmin},
		Status:         datastore.InviteStatusPending,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgInviteCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.OrganisationInvitesCollection)
	err := inviteRepo.CreateOrganisationInvite(orgInviteCtx, iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByID(orgInviteCtx, iv.UID)
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
			Type:  auth.RoleAdmin,
			Group: uuid.NewString(),
		},
		Status:         datastore.InviteStatusPending,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgInviteCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.OrganisationInvitesCollection)

	err := inviteRepo.CreateOrganisationInvite(orgInviteCtx, iv)
	require.NoError(t, err)

	role := auth.Role{
		Type:  auth.RoleSuperUser,
		Group: uuid.NewString(),
		App:   "",
	}
	status := datastore.InviteStatusAccepted
	updatedAt := primitive.NewDateTimeFromTime(time.Now())

	iv.Role = role
	iv.Status = status
	iv.UpdatedAt = updatedAt

	err = inviteRepo.UpdateOrganisationInvite(orgInviteCtx, iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByID(orgInviteCtx, iv.UID)
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
			Type:  auth.RoleAdmin,
			Group: uuid.NewString(),
		},
		Status:         datastore.InviteStatusPending,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgInviteCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.OrganisationInvitesCollection)

	err := inviteRepo.CreateOrganisationInvite(orgInviteCtx, org)
	require.NoError(t, err)

	err = inviteRepo.DeleteOrganisationInvite(orgInviteCtx, org.UID)
	require.NoError(t, err)

	_, err = inviteRepo.FetchOrganisationInviteByID(orgInviteCtx, org.UID)
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
			Type:  auth.RoleAdmin,
			Group: uuid.NewString(),
		},
		Status:         datastore.InviteStatusPending,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgInviteCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.OrganisationInvitesCollection)

	err := inviteRepo.CreateOrganisationInvite(orgInviteCtx, iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByID(orgInviteCtx, iv.UID)
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
			Type:  auth.RoleAdmin,
			Group: uuid.NewString(),
		},
		Status:         datastore.InviteStatusPending,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	orgInviteCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.OrganisationInvitesCollection)

	err := inviteRepo.CreateOrganisationInvite(orgInviteCtx, iv)
	require.NoError(t, err)

	invite, err := inviteRepo.FetchOrganisationInviteByToken(orgInviteCtx, iv.Token)
	require.NoError(t, err)

	require.Equal(t, iv.UID, invite.UID)
	require.Equal(t, iv.Token, invite.Token)
	require.Equal(t, iv.InviteeEmail, invite.InviteeEmail)
}
