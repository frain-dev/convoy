//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestLoadOrganisationMembersPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)
	org := seedOrg(t, db)

	userMap := map[string]*datastore.UserMetadata{}
	userRepo := NewUserRepo(db)

	for i := 1; i < 6; i++ {
		user := generateUser(t)

		require.NoError(t, userRepo.CreateUser(context.Background(), user))

		member := &datastore.OrganisationMember{
			UID:            ulid.Make().String(),
			OrganisationID: org.UID,
			UserID:         user.UID,
			Role:           auth.Role{Type: auth.RoleAdmin},
		}

		userMap[user.UID] = &datastore.UserMetadata{
			UserID:    user.UID,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
		}

		err := organisationMemberRepo.CreateOrganisationMember(context.Background(), member)
		require.NoError(t, err)
	}

	members, _, err := organisationMemberRepo.LoadOrganisationMembersPaged(context.Background(), org.UID, datastore.Pageable{
		Page:    2,
		PerPage: 2,
		Sort:    -1,
	})

	require.NoError(t, err)
	require.Equal(t, 2, len(members))

	for _, member := range members {
		m := userMap[member.UserID]
		require.Equal(t, *m, member.UserMetadata)
	}
}

func TestLoadUserOrganisationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)
	orgRepo := NewOrgRepo(db)

	user := seedUser(t, db)
	for i := 0; i < 7; i++ {

		org := &datastore.Organisation{
			UID:     uuid.NewString(),
			OwnerID: user.UID,
		}

		err := orgRepo.CreateOrganisation(context.Background(), org)
		require.NoError(t, err)

		member := &datastore.OrganisationMember{
			UID:            ulid.Make().String(),
			OrganisationID: org.UID,
			UserID:         user.UID,
			Role:           auth.Role{Type: auth.RoleAdmin},
		}

		err = organisationMemberRepo.CreateOrganisationMember(context.Background(), member)
		require.NoError(t, err)
	}

	organisations, _, err := organisationMemberRepo.LoadUserOrganisationsPaged(context.Background(), user.UID, datastore.Pageable{
		Page:    1,
		PerPage: 10,
		Sort:    -1,
	})

	require.NoError(t, err)
	require.Equal(t, 7, len(organisations))
}

func TestCreateOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := generateUser(t)
	require.NoError(t, NewUserRepo(db).CreateUser(context.Background(), user))
	org := seedOrg(t, db)

	organisationMemberRepo := NewOrgMemberRepo(db)

	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin},
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	member, err := organisationMemberRepo.FetchOrganisationMemberByID(context.Background(), m.UID, m.OrganisationID)
	require.NoError(t, err)

	require.Equal(t, m.UID, member.UID)
	require.Equal(t, m.OrganisationID, member.OrganisationID)
	require.Equal(t, m.UserID, member.UserID)
	require.Equal(t, datastore.UserMetadata{
		UserID:    user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, member.UserMetadata)
}

func TestUpdateOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := generateUser(t)
	org := seedOrg(t, db)
	require.NoError(t, NewUserRepo(db).CreateUser(context.Background(), user))

	organisationMemberRepo := NewOrgMemberRepo(db)
	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin},
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	role := auth.Role{
		Type:     auth.RoleSuperUser,
		Project:  uuid.NewString(),
		Endpoint: "",
	}
	m.Role = role

	err = organisationMemberRepo.UpdateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	member, err := organisationMemberRepo.FetchOrganisationMemberByID(context.Background(), m.UID, m.OrganisationID)
	require.NoError(t, err)

	require.Equal(t, m.UID, member.UID)
	require.Equal(t, role, member.Role)
	require.Equal(t, datastore.UserMetadata{
		UserID:    user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, member.UserMetadata)
}

func TestDeleteOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db)
	org := seedOrg(t, db)

	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         org.OwnerID,
		Role:           auth.Role{Type: auth.RoleAdmin},
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

	user := generateUser(t)
	require.NoError(t, NewUserRepo(db).CreateUser(context.Background(), user))

	org := seedOrg(t, db)
	organisationMemberRepo := NewOrgMemberRepo(db)

	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin},
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	member, err := organisationMemberRepo.FetchOrganisationMemberByID(context.Background(), m.UID, m.OrganisationID)
	require.NoError(t, err)

	require.Equal(t, m.UID, member.UID)
	require.Equal(t, m.OrganisationID, member.OrganisationID)
	require.Equal(t, m.UserID, member.UserID)
	require.Equal(t, datastore.UserMetadata{
		UserID:    user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, member.UserMetadata)
}

func TestFetchOrganisationMemberByUserID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := generateUser(t)
	require.NoError(t, NewUserRepo(db).CreateUser(context.Background(), user))

	org := seedOrg(t, db)

	organisationMemberRepo := NewOrgMemberRepo(db)
	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin},
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	member, err := organisationMemberRepo.FetchOrganisationMemberByUserID(context.Background(), m.UserID, m.OrganisationID)
	require.NoError(t, err)

	require.Equal(t, m.UID, member.UID)
	require.Equal(t, m.OrganisationID, member.OrganisationID)
	require.Equal(t, m.UserID, member.UserID)
	require.Equal(t, datastore.UserMetadata{
		UserID:    user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, member.UserMetadata)
}
