//go:build integration
// +build integration

package mongo

import (
	"context"
	"github.com/frain-dev/convoy/auth"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestLoadOrganisationMembersPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)
	orgID := uuid.NewString()

	for i := 1; i < 6; i++ {
		member := &datastore.OrganisationMember{
			UID:            uuid.NewString(),
			OrganisationID: orgID,
			UserID:         uuid.NewString(),
			Role:           auth.Role{Type: auth.RoleAdmin},
			DocumentStatus: datastore.ActiveDocumentStatus,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}

		err := organisationMemberRepo.CreateOrganisationMember(context.Background(), member)
		require.NoError(t, err)
	}

	organisationInvites, _, err := organisationMemberRepo.LoadOrganisationMembersPaged(context.Background(), orgID, datastore.Pageable{
		Page:    2,
		PerPage: 2,
		Sort:    -1,
	})

	require.NoError(t, err)
	require.Equal(t, 2, len(organisationInvites))
}

func TestCreateOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)

	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         uuid.NewString(),
		Role:           auth.Role{Type: auth.RoleAdmin},
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	member, err := organisationMemberRepo.FetchOrganisationMemberByID(context.Background(), m.UID, m.OrganisationID)
	require.NoError(t, err)

	require.Equal(t, m.UID, member.UID)
	require.Equal(t, m.OrganisationID, member.OrganisationID)
	require.Equal(t, m.UserID, member.UserID)
}

func TestUpdateOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)

	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         uuid.NewString(),
		Role:           auth.Role{Type: auth.RoleAdmin},
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	role := auth.Role{
		Type:   auth.RoleSuperUser,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}
	m.Role = role

	err = organisationMemberRepo.UpdateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	member, err := organisationMemberRepo.FetchOrganisationMemberByID(context.Background(), m.UID, m.OrganisationID)
	require.NoError(t, err)

	require.Equal(t, m.UID, member.UID)
	require.Equal(t, role, member.Role)
}

func TestDeleteOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)
	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         uuid.NewString(),
		Role:           auth.Role{Type: auth.RoleAdmin},
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	err = organisationMemberRepo.DeleteOrganisationMember(context.Background(), m.UID, m.OrganisationID)
	require.NoError(t, err)

	_, err = organisationMemberRepo.FetchOrganisationMemberByID(context.Background(), m.UID, m.OrganisationID)
	require.Equal(t, datastore.ErrOrgMemberNotFound, err)
}

func TestFetchOrganisationMemberByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)
	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         uuid.NewString(),
		Role:           auth.Role{Type: auth.RoleAdmin},
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	member, err := organisationMemberRepo.FetchOrganisationMemberByID(context.Background(), m.UID, m.OrganisationID)
	require.NoError(t, err)

	require.Equal(t, m.UID, member.UID)
	require.Equal(t, m.OrganisationID, member.OrganisationID)
	require.Equal(t, m.UserID, member.UserID)
}

func TestFetchOrganisationMemberByUserID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)
	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         uuid.NewString(),
		Role:           auth.Role{Type: auth.RoleAdmin},
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	member, err := organisationMemberRepo.FetchOrganisationMemberByUserID(context.Background(), m.UserID, m.OrganisationID)
	require.NoError(t, err)

	require.Equal(t, m.UID, member.UID)
	require.Equal(t, m.OrganisationID, member.OrganisationID)
	require.Equal(t, m.UserID, member.UserID)
}
