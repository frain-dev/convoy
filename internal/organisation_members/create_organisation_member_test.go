package organisation_members

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func Test_CreateOrganisationMember_SuperUser(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	// Create super user member
	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role: auth.Role{
			Type: auth.RoleProjectAdmin,
		},
	}

	err := service.CreateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the member was created
	fetchedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, member.UID, fetchedMember.UID)
	require.Equal(t, member.OrganisationID, fetchedMember.OrganisationID)
	require.Equal(t, member.UserID, fetchedMember.UserID)
	require.Equal(t, auth.RoleProjectAdmin, fetchedMember.Role.Type)
}

func Test_CreateOrganisationMember_InstanceAdmin(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	// Create instance admin member
	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role: auth.Role{
			Type: auth.RoleInstanceAdmin,
		},
	}

	err := service.CreateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the member was created
	fetchedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, auth.RoleInstanceAdmin, fetchedMember.Role.Type)
}

func Test_CreateOrganisationMember_OrganisationAdmin(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	// Create organisation admin member
	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role: auth.Role{
			Type: auth.RoleOrganisationAdmin,
		},
	}

	err := service.CreateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the member was created
	fetchedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, auth.RoleOrganisationAdmin, fetchedMember.Role.Type)
}

func Test_CreateOrganisationMember_WithProjectRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project := seedProject(t, db, org.UID)

	// Create member with project role
	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role: auth.Role{
			Type:    auth.RoleProjectAdmin,
			Project: project.UID,
		},
	}

	err := service.CreateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the member was created with project role
	fetchedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, auth.RoleProjectAdmin, fetchedMember.Role.Type)
	require.Equal(t, project.UID, fetchedMember.Role.Project)
}

func Test_CreateOrganisationMember_NilMember(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Attempt to create nil member
	err := service.CreateOrganisationMember(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")
}

func Test_CreateOrganisationMember_DuplicateMember(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	// Create first member
	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role: auth.Role{
			Type: auth.RoleProjectAdmin,
		},
	}

	err := service.CreateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Attempt to create duplicate member with same UID
	err = service.CreateOrganisationMember(ctx, member)
	require.Error(t, err)
}

func Test_CreateOrganisationMember_WithEndpointRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project := seedProject(t, db, org.UID)
	endpoint := seedEndpoint(t, db, project.UID)

	// Create member with endpoint role
	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role: auth.Role{
			Type:     auth.RoleProjectAdmin,
			Project:  project.UID,
			Endpoint: endpoint.UID,
		},
	}

	err := service.CreateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the member was created with endpoint role
	fetchedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, auth.RoleProjectAdmin, fetchedMember.Role.Type)
	require.Equal(t, project.UID, fetchedMember.Role.Project)
	require.Equal(t, endpoint.UID, fetchedMember.Role.Endpoint)
}

func Test_CreateOrganisationMember_WithUserMetadata(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "test-metadata@example.com")
	org := seedOrganisation(t, db, user.UID)

	// Create member
	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role: auth.Role{
			Type: auth.RoleProjectAdmin,
		},
	}

	err := service.CreateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify user metadata is populated
	fetchedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, user.UID, fetchedMember.UserMetadata.UserID)
	require.Equal(t, "Test", fetchedMember.UserMetadata.FirstName)
	require.Equal(t, "User", fetchedMember.UserMetadata.LastName)
	require.Equal(t, "test-metadata@example.com", fetchedMember.UserMetadata.Email)
}
