//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func TestLoadOrganisationMembersPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	organisationMemberRepo := NewOrgMemberRepo(db, nil)
	org := seedOrg(t, db)
	project := seedProject(t, db)

	userMap := map[string]*datastore.UserMetadata{}
	userRepo := NewUserRepo(db, nil)

	for i := 1; i < 6; i++ {
		user := generateUser(t)

		require.NoError(t, userRepo.CreateUser(context.Background(), user))

		member := &datastore.OrganisationMember{
			UID:            ulid.Make().String(),
			OrganisationID: org.UID,
			UserID:         user.UID,
			Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
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

	members, _, err := organisationMemberRepo.LoadOrganisationMembersPaged(context.Background(), org.UID, "", datastore.Pageable{
		PerPage: 2,
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

	organisationMemberRepo := NewOrgMemberRepo(db, nil)
	orgRepo := NewOrgRepo(db, nil)
	project := seedProject(t, db)

	user := seedUser(t, db)
	for i := 0; i < 7; i++ {

		org := &datastore.Organisation{
			UID:     ulid.Make().String(),
			OwnerID: user.UID,
		}

		err := orgRepo.CreateOrganisation(context.Background(), org)
		require.NoError(t, err)

		member := &datastore.OrganisationMember{
			UID:            ulid.Make().String(),
			OrganisationID: org.UID,
			UserID:         user.UID,
			Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
		}

		err = organisationMemberRepo.CreateOrganisationMember(context.Background(), member)
		require.NoError(t, err)
	}

	organisations, _, err := organisationMemberRepo.LoadUserOrganisationsPaged(context.Background(), user.UID, datastore.Pageable{
		PerPage: 10,
	})

	require.NoError(t, err)
	require.Equal(t, 7, len(organisations))
}

func TestCreateOrganisationMember(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := generateUser(t)
	require.NoError(t, NewUserRepo(db, nil).CreateUser(context.Background(), user))
	org := seedOrg(t, db)
	project := seedProject(t, db)

	organisationMemberRepo := NewOrgMemberRepo(db, nil)

	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
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
	require.NoError(t, NewUserRepo(db, nil).CreateUser(context.Background(), user))
	project := seedProject(t, db)

	organisationMemberRepo := NewOrgMemberRepo(db, nil)
	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	role := auth.Role{
		Type:     auth.RoleSuperUser,
		Project:  project.UID,
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

	organisationMemberRepo := NewOrgMemberRepo(db, nil)
	org := seedOrg(t, db)
	project := seedProject(t, db)

	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         org.OwnerID,
		Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
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
	require.NoError(t, NewUserRepo(db, nil).CreateUser(context.Background(), user))

	org := seedOrg(t, db)
	project := seedProject(t, db)
	organisationMemberRepo := NewOrgMemberRepo(db, nil)

	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
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
	require.NoError(t, NewUserRepo(db, nil).CreateUser(context.Background(), user))

	org := seedOrg(t, db)
	project := seedProject(t, db)

	organisationMemberRepo := NewOrgMemberRepo(db, nil)
	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
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

func TestFetchUserProjects(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	user := generateUser(t)
	ctx := context.Background()

	require.NoError(t, NewUserRepo(db, nil).CreateUser(ctx, user))

	org := seedOrg(t, db)
	project := seedProject(t, db)

	organisationMemberRepo := NewOrgMemberRepo(db, nil)
	projectRepo := NewProjectRepo(db, nil)
	m := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           auth.Role{Type: auth.RoleAdmin, Project: project.UID},
	}

	err := organisationMemberRepo.CreateOrganisationMember(context.Background(), m)
	require.NoError(t, err)

	project1 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "project1",
		Config:         &datastore.DefaultProjectConfig,
		OrganisationID: org.UID,
	}

	project2 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "project2",
		Config:         &datastore.DefaultProjectConfig,
		OrganisationID: org.UID,
	}

	err = projectRepo.CreateProject(context.Background(), project1)
	require.NoError(t, err)

	err = projectRepo.CreateProject(context.Background(), project2)
	require.NoError(t, err)

	projects, err := organisationMemberRepo.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)

	require.Equal(t, 2, len(projects))
}
