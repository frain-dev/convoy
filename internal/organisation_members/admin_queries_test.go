package organisation_members

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func Test_FetchInstanceAdminByUserID_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "instance-admin@example.com")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleInstanceAdmin})

	// Fetch instance admin
	fetchedMember, err := service.FetchInstanceAdminByUserID(ctx, user.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, member.UID, fetchedMember.UID)
	require.Equal(t, auth.RoleInstanceAdmin, fetchedMember.Role.Type)
}

func Test_FetchInstanceAdminByUserID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data with non-admin user
	user := seedUser(t, db, "regular-user@example.com")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Attempt to fetch as instance admin
	_, err := service.FetchInstanceAdminByUserID(ctx, user.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_FetchInstanceAdminByUserID_MultipleOrgs(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data - user is instance admin in multiple orgs
	user := seedUser(t, db, "multi-admin@example.com")
	org1 := seedOrganisation(t, db, user.UID)
	org2 := seedOrganisation(t, db, user.UID)

	member1 := seedOrganisationMember(t, db, org1.UID, user.UID, auth.Role{Type: auth.RoleInstanceAdmin})
	_ = seedOrganisationMember(t, db, org2.UID, user.UID, auth.Role{Type: auth.RoleInstanceAdmin})

	// Fetch should return one (LIMIT 1 in query)
	fetchedMember, err := service.FetchInstanceAdminByUserID(ctx, user.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	// Should match the first one created
	require.Equal(t, member1.UID, fetchedMember.UID)
}

func Test_FetchAnyOrganisationAdminByUserID_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "org-admin@example.com")
	org := seedOrganisation(t, db, user.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleOrganisationAdmin})

	// Fetch organisation admin
	fetchedMember, err := service.FetchAnyOrganisationAdminByUserID(ctx, user.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedMember)
	require.Equal(t, member.UID, fetchedMember.UID)
	require.Equal(t, auth.RoleOrganisationAdmin, fetchedMember.Role.Type)
}

func Test_FetchAnyOrganisationAdminByUserID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data with non-org-admin user
	user := seedUser(t, db, "regular-user@example.com")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Attempt to fetch as organisation admin
	_, err := service.FetchAnyOrganisationAdminByUserID(ctx, user.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrOrgMemberNotFound)
}

func Test_CountInstanceAdminUsers_Zero(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// No instance admins
	count, err := service.CountInstanceAdminUsers(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func Test_CountInstanceAdminUsers_Single(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed single instance admin
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleInstanceAdmin})

	count, err := service.CountInstanceAdminUsers(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func Test_CountInstanceAdminUsers_Multiple(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed multiple instance admins
	user1 := seedUser(t, db, "admin1@example.com")
	user2 := seedUser(t, db, "admin2@example.com")
	user3 := seedUser(t, db, "admin3@example.com")
	org := seedOrganisation(t, db, user1.UID)

	_ = seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleInstanceAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleInstanceAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user3.UID, auth.Role{Type: auth.RoleInstanceAdmin})

	count, err := service.CountInstanceAdminUsers(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func Test_CountInstanceAdminUsers_ExcludesDeleted(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed instance admins
	user1 := seedUser(t, db, "admin1@example.com")
	user2 := seedUser(t, db, "admin2@example.com")
	org := seedOrganisation(t, db, user1.UID)

	member1 := seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleInstanceAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleInstanceAdmin})

	// Delete one member
	err := service.DeleteOrganisationMember(ctx, member1.UID, org.UID)
	require.NoError(t, err)

	// Count should only include active members
	count, err := service.CountInstanceAdminUsers(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func Test_CountOrganisationAdminUsers_Zero(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// No organisation admins
	count, err := service.CountOrganisationAdminUsers(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func Test_CountOrganisationAdminUsers_Multiple(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed multiple organisation admins
	user1 := seedUser(t, db, "orgadmin1@example.com")
	user2 := seedUser(t, db, "orgadmin2@example.com")
	org := seedOrganisation(t, db, user1.UID)

	_ = seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleOrganisationAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleOrganisationAdmin})

	count, err := service.CountOrganisationAdminUsers(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

func Test_HasInstanceAdminAccess_UserIsInstanceAdmin(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed instance admin
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleInstanceAdmin})

	hasAccess, err := service.HasInstanceAdminAccess(ctx, user.UID)
	require.NoError(t, err)
	require.True(t, hasAccess)
}

func Test_HasInstanceAdminAccess_UserIsNotInstanceAdmin_ButNoOtherInstanceAdmins(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed regular user with no instance admins in the system
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	hasAccess, err := service.HasInstanceAdminAccess(ctx, user.UID)
	require.NoError(t, err)
	require.True(t, hasAccess) // True because no other instance admins exist
}

func Test_HasInstanceAdminAccess_UserIsNotInstanceAdmin_WithOtherInstanceAdmins(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed users
	user1 := seedUser(t, db, "admin@example.com")
	user2 := seedUser(t, db, "regular@example.com")
	org := seedOrganisation(t, db, user1.UID)

	// user1 is instance admin, user2 is not
	_ = seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleInstanceAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleProjectAdmin})

	hasAccess, err := service.HasInstanceAdminAccess(ctx, user2.UID)
	require.NoError(t, err)
	require.False(t, hasAccess)
}

func Test_IsFirstInstanceAdmin_UserIsFirst(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed first instance admin
	user := seedUser(t, db, "first-admin@example.com")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleInstanceAdmin})

	isFirst, err := service.IsFirstInstanceAdmin(ctx, user.UID)
	require.NoError(t, err)
	require.True(t, isFirst)
}

func Test_IsFirstInstanceAdmin_UserIsNotFirst(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed multiple instance admins
	user1 := seedUser(t, db, "first-admin@example.com")
	user2 := seedUser(t, db, "second-admin@example.com")
	org := seedOrganisation(t, db, user1.UID)

	_ = seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleInstanceAdmin})
	// Small delay to ensure different created_at timestamps
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleInstanceAdmin})

	isFirst, err := service.IsFirstInstanceAdmin(ctx, user2.UID)
	require.NoError(t, err)
	require.False(t, isFirst)
}

func Test_IsFirstInstanceAdmin_UserIsNotAdmin(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed regular user
	user := seedUser(t, db, "regular@example.com")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	isFirst, err := service.IsFirstInstanceAdmin(ctx, user.UID)
	require.NoError(t, err)
	require.False(t, isFirst)
}
