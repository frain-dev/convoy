package organisation_members

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
)

func Test_UpdateOrganisationMember_RoleChange(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Update member role
	member.Role = auth.Role{Type: auth.RoleProjectAdmin}
	err := service.UpdateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the role was updated
	updatedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectAdmin, updatedMember.Role.Type)
}

func Test_UpdateOrganisationMember_ToInstanceAdmin(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Update to instance admin
	member.Role = auth.Role{Type: auth.RoleInstanceAdmin}
	err := service.UpdateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the role was updated
	updatedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleInstanceAdmin, updatedMember.Role.Type)
}

func Test_UpdateOrganisationMember_ToOrganisationAdmin(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Update to organisation admin
	member.Role = auth.Role{Type: auth.RoleOrganisationAdmin}
	err := service.UpdateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the role was updated
	updatedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleOrganisationAdmin, updatedMember.Role.Type)
}

func Test_UpdateOrganisationMember_WithProjectRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project := seedProject(t, db, org.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Update with project role
	member.Role = auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: project.UID,
	}
	err := service.UpdateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the role was updated
	updatedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectAdmin, updatedMember.Role.Type)
	require.Equal(t, project.UID, updatedMember.Role.Project)
}

func Test_UpdateOrganisationMember_ChangeProjectRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project1 := seedProject(t, db, org.UID)
	project2 := seedProject(t, db, org.UID)

	// Create member with project1 role
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: project1.UID,
	})

	// Update to project2 role
	member.Role = auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: project2.UID,
	}
	err := service.UpdateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify the project role was updated
	updatedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.Equal(t, project2.UID, updatedMember.Role.Project)
}

func Test_UpdateOrganisationMember_ClearProjectRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project := seedProject(t, db, org.UID)

	// Create member with project role
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: project.UID,
	})

	// Update to clear project role
	member.Role = auth.Role{Type: auth.RoleProjectAdmin}
	err := service.UpdateOrganisationMember(ctx, member)
	require.NoError(t, err)

	// Verify project role was cleared
	updatedMember, err := service.FetchOrganisationMemberByID(ctx, member.UID, org.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectAdmin, updatedMember.Role.Type)
	require.Empty(t, updatedMember.Role.Project)
}

func Test_UpdateOrganisationMember_NilMember(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Attempt to update nil member
	err := service.UpdateOrganisationMember(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot be nil")
}

func Test_UpdateOrganisationMember_NonExistent(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	// Create a member that doesn't exist in DB
	nonExistentMember := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Delete the member
	err := service.DeleteOrganisationMember(ctx, nonExistentMember.UID, org.UID)
	require.NoError(t, err)

	// Attempt to update the deleted member
	nonExistentMember.Role = auth.Role{Type: auth.RoleProjectAdmin}
	err = service.UpdateOrganisationMember(ctx, nonExistentMember)
	// Should not return error for update (no rows affected)
	require.NoError(t, err)
}
