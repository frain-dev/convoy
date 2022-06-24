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

func TestLoadOrganisationMembersPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db, OrganisationMembersCollection)
	userStore := getStore(db, UserCollection)
	organisationMemberRepo := NewOrgMemberRepo(db, store)
	orgID := uuid.NewString()

	userMap := map[string]*datastore.UserMetadata{}

	for i := 1; i < 6; i++ {
		user := &datastore.User{
			UID:            uuid.NewString(),
			FirstName:      fmt.Sprintf("test-%s", uuid.NewString()),
			LastName:       fmt.Sprintf("test-%s", uuid.NewString()),
			Email:          fmt.Sprintf("test-%s", uuid.NewString()),
			Password:       fmt.Sprintf("test-%s", uuid.NewString()),
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			DocumentStatus: datastore.ActiveDocumentStatus,
		}
		require.NoError(t, NewUserRepo(db, userStore).CreateUser(context.Background(), user))

		member := &datastore.OrganisationMember{
			UID:            uuid.NewString(),
			OrganisationID: orgID,
			UserID:         user.UID,
			Role:           auth.Role{Type: auth.RoleAdmin},
			DocumentStatus: datastore.ActiveDocumentStatus,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
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

	members, _, err := organisationMemberRepo.LoadOrganisationMembersPaged(context.Background(), orgID, datastore.Pageable{
		Page:    2,
		PerPage: 2,
		Sort:    -1,
	})

	require.NoError(t, err)
	require.Equal(t, 2, len(members))

	for _, member := range members {
		m := userMap[member.UserID]
		require.Equal(t, *m, *member.UserMetadata)
	}
}

func TestLoadUserOrganisationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgMemberStore := getStore(db, OrganisationMembersCollection)
	orgStore := getStore(db, OrganisationCollection)
	organisationMemberRepo := NewOrgMemberRepo(db, orgMemberStore)

	userID := uuid.NewString()
	for i := 0; i < 7; i++ {
		status := datastore.ActiveDocumentStatus
		if i == 6 {
			status = datastore.DeletedDocumentStatus
		}
		org := &datastore.Organisation{UID: uuid.NewString(), DocumentStatus: status}

		err := NewOrgRepo(db, orgStore).CreateOrganisation(context.Background(), org)
		require.NoError(t, err)

		member := &datastore.OrganisationMember{
			UID:            uuid.NewString(),
			OrganisationID: org.UID,
			UserID:         userID,
			Role:           auth.Role{Type: auth.RoleAdmin},
			DocumentStatus: datastore.ActiveDocumentStatus,
			CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		}

		err = organisationMemberRepo.CreateOrganisationMember(context.Background(), member)
		require.NoError(t, err)
	}

	organisations, _, err := organisationMemberRepo.LoadUserOrganisationsPaged(context.Background(), userID, datastore.Pageable{
		Page:    1,
		PerPage: 10,
		Sort:    -1,
	})

	require.NoError(t, err)
	require.Equal(t, 6, len(organisations))
}

func TestCreateOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userStore := getStore(db, UserCollection)
	user := &datastore.User{
		UID:            uuid.NewString(),
		FirstName:      fmt.Sprintf("test-%s", uuid.NewString()),
		LastName:       fmt.Sprintf("test-%s", uuid.NewString()),
		Email:          fmt.Sprintf("test-%s", uuid.NewString()),
		Password:       fmt.Sprintf("test-%s", uuid.NewString()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}
	require.NoError(t, NewUserRepo(db, userStore).CreateUser(context.Background(), user))

	orgMemberStore := getStore(db, OrganisationMembersCollection)
	organisationMemberRepo := NewOrgMemberRepo(db, orgMemberStore)

	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         user.UID,
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
	require.Equal(t, datastore.UserMetadata{
		UserID:    user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, *member.UserMetadata)
}

func TestUpdateOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userStore := getStore(db, UserCollection)
	user := &datastore.User{
		UID:            uuid.NewString(),
		FirstName:      fmt.Sprintf("test-%s", uuid.NewString()),
		LastName:       fmt.Sprintf("test-%s", uuid.NewString()),
		Email:          fmt.Sprintf("test-%s", uuid.NewString()),
		Password:       fmt.Sprintf("test-%s", uuid.NewString()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}
	require.NoError(t, NewUserRepo(db, userStore).CreateUser(context.Background(), user))

	orgMemberStore := getStore(db, OrganisationMembersCollection)
	organisationMemberRepo := NewOrgMemberRepo(db, orgMemberStore)
	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         user.UID,
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
	require.Equal(t, datastore.UserMetadata{
		UserID:    user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, *member.UserMetadata)
}

func TestDeleteOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	orgMemberStore := getStore(db, OrganisationMembersCollection)
	organisationMemberRepo := NewOrgMemberRepo(db, orgMemberStore)

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

	userStore := getStore(db, UserCollection)
	user := &datastore.User{
		UID:            uuid.NewString(),
		FirstName:      fmt.Sprintf("test-%s", uuid.NewString()),
		LastName:       fmt.Sprintf("test-%s", uuid.NewString()),
		Email:          fmt.Sprintf("test-%s", uuid.NewString()),
		Password:       fmt.Sprintf("test-%s", uuid.NewString()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}
	require.NoError(t, NewUserRepo(db, userStore).CreateUser(context.Background(), user))
	orgMemberStore := getStore(db, OrganisationMembersCollection)
	organisationMemberRepo := NewOrgMemberRepo(db, orgMemberStore)

	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         user.UID,
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
	require.Equal(t, datastore.UserMetadata{
		UserID:    user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, *member.UserMetadata)
}

func TestFetchOrganisationMemberByUserID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userStore := getStore(db, UserCollection)
	user := &datastore.User{
		UID:            uuid.NewString(),
		FirstName:      fmt.Sprintf("test-%s", uuid.NewString()),
		LastName:       fmt.Sprintf("test-%s", uuid.NewString()),
		Email:          uuid.NewString(),
		Password:       fmt.Sprintf("test-%s", uuid.NewString()),
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}
	require.NoError(t, NewUserRepo(db, userStore).CreateUser(context.Background(), user))

	orgMemberStore := getStore(db, OrganisationMembersCollection)
	organisationMemberRepo := NewOrgMemberRepo(db, orgMemberStore)
	m := &datastore.OrganisationMember{
		UID:            uuid.NewString(),
		OrganisationID: uuid.NewString(),
		UserID:         user.UID,
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
	require.Equal(t, datastore.UserMetadata{
		UserID:    user.UID,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, *member.UserMetadata)
}
