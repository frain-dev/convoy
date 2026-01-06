package organisation_members

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func Test_FetchOrganisationMemberByID_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "test@example.com")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Fetch the member
	fetchedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, member.UID, fetchedMember.UID)
	require.Equal(t, member.OrganisationID, fetchedMember.OrganisationID)
	require.Equal(t, member.UserID, fetchedMember.UserID)
	require.Equal(t, auth.RoleProjectAdmin, fetchedMember.Role.Type)

	// Verify user metadata
	require.Equal(t, user.UID, fetchedMember.UserMetadata.UserID)
	require.Equal(t, "Test", fetchedMember.UserMetadata.FirstName)
	require.Equal(t, "User", fetchedMember.UserMetadata.LastName)
	require.Equal(t, "test@example.com", fetchedMember.UserMetadata.Email)
}

func Test_FetchOrganisationMemberByID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	// Attempt to fetch non-existent member
	_, err := service.FetchOrganisationMemberByID(ctx, ulid.Make().String(), org.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_FetchOrganisationMemberByID_WrongOrganisation(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user1 := seedUser(t, db, "user1@example.com")
	user2 := seedUser(t, db, "user2@example.com")
	org1 := seedOrganisation(t, db, user1.UID)
	org2 := seedOrganisation(t, db, user2.UID)
	member := seedOrganisationMember(t, db, org1.UID, user1.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Attempt to fetch from wrong organisation
	_, err := service.FetchOrganisationMemberByID(ctx, member.UID, org2.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_FetchOrganisationMemberByID_Deleted(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Delete the member
	err := service.DeleteOrganisationMember(ctx, member.UID, org.UID)
	require.NoError(t, err)

	// Attempt to fetch deleted member
	_, err = service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_FetchOrganisationMemberByUserID_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "test-user@example.com")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Fetch by user ID
	fetchedMember, err := service.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, member.UID, fetchedMember.UID)
	require.Equal(t, user.UID, fetchedMember.UserID)
	require.Equal(t, org.UID, fetchedMember.OrganisationID)
}

func Test_FetchOrganisationMemberByUserID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	// Attempt to fetch non-existent user
	_, err := service.FetchOrganisationMemberByUserID(ctx, ulid.Make().String(), org.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_FetchOrganisationMemberByUserID_WrongOrganisation(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user1 := seedUser(t, db, "user1@example.com")
	user2 := seedUser(t, db, "user2@example.com")
	org1 := seedOrganisation(t, db, user1.UID)
	org2 := seedOrganisation(t, db, user2.UID)
	_ = seedOrganisationMember(t, db, org1.UID, user1.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Attempt to fetch from wrong organisation
	_, err := service.FetchOrganisationMemberByUserID(ctx, user1.UID, org2.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_FetchOrganisationMemberByUserID_WithProjectRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project := seedProject(t, db, org.UID)

	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: project.UID,
	})

	// Fetch by user ID
	fetchedMember, err := service.FetchOrganisationMemberByUserID(ctx, user.UID, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, member.UID, fetchedMember.UID)
	require.Equal(t, auth.RoleProjectAdmin, fetchedMember.Role.Type)
	require.Equal(t, project.UID, fetchedMember.Role.Project)
}
