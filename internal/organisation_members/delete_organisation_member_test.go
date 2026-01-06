package organisation_members

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func Test_DeleteOrganisationMember_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Delete the member
	err := service.DeleteOrganisationMember(ctx, member.UID, org.UID)
	require.NoError(t, err)

	// Verify the member was soft deleted
	_, err = service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_DeleteOrganisationMember_NonExistent(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	// Attempt to delete non-existent member
	err := service.DeleteOrganisationMember(ctx, "non-existent-id", org.UID)
	// Should not return error (no rows affected)
	require.NoError(t, err)
}

func Test_DeleteOrganisationMember_WrongOrganisation(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user1 := seedUser(t, db, "user1@example.com")
	user2 := seedUser(t, db, "user2@example.com")
	org1 := seedOrganisation(t, db, user1.UID)
	org2 := seedOrganisation(t, db, user2.UID)
	member := seedOrganisationMember(t, db, org1.UID, user1.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Attempt to delete member from wrong organisation
	err := service.DeleteOrganisationMember(ctx, member.UID, org2.UID)
	require.NoError(t, err)

	// Verify member still exists in correct organisation
	fetchedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org1.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
}

func Test_DeleteOrganisationMember_AlreadyDeleted(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Delete the member first time
	err := service.DeleteOrganisationMember(ctx, member.UID, org.UID)
	require.NoError(t, err)

	// Delete the member second time (idempotent)
	err = service.DeleteOrganisationMember(ctx, member.UID, org.UID)
	require.NoError(t, err)

	// Verify the member is still not found
	_, err = service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_DeleteOrganisationMember_MultipleMembers(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user1 := seedUser(t, db, "user1@example.com")
	user2 := seedUser(t, db, "user2@example.com")
	user3 := seedUser(t, db, "user3@example.com")
	org := seedOrganisation(t, db, user1.UID)

	member1 := seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleProjectAdmin})
	member2 := seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleProjectAdmin})
	member3 := seedOrganisationMember(t, db, org.UID, user3.UID, auth.Role{Type: auth.RoleAPI})

	// Delete only member2
	err := service.DeleteOrganisationMember(ctx, member2.UID, org.UID)
	require.NoError(t, err)

	// Verify member1 and member3 still exist
	_, err = service.FetchOrganisationMemberByID(ctx, member1.UID, org.UID)
	require.NoError(t, err)

	_, err = service.FetchOrganisationMemberByID(ctx, member3.UID, org.UID)
	require.NoError(t, err)

	// Verify member2 is deleted
	_, err = service.FetchOrganisationMemberByID(ctx, member2.UID, org.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}
