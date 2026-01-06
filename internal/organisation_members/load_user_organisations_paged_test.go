package organisation_members

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/pkg/log"
)

func Test_LoadUserOrganisationsPaged_Forward_SingleOrg(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	orgs, paginationData, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, org.UID, orgs[0].UID)
	require.False(t, paginationData.HasNextPage)
	require.False(t, paginationData.HasPreviousPage)
}

func Test_LoadUserOrganisationsPaged_Forward_MultipleOrgs(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data - user is member of 3 organisations
	user := seedUser(t, db, "multi-org-user@example.com")
	org1 := seedOrganisation(t, db, user.UID)
	org2 := seedOrganisation(t, db, user.UID)
	org3 := seedOrganisation(t, db, user.UID)

	_ = seedOrganisationMember(t, db, org1.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org2.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org3.UID, user.UID, auth.Role{Type: auth.RoleAPI})

	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	orgs, _, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	require.NoError(t, err)
	require.Len(t, orgs, 3)
}

func Test_LoadUserOrganisationsPaged_Forward_WithPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data - user is member of 5 organisations
	user := seedUser(t, db, "pagination-user@example.com")
	for i := 0; i < 5; i++ {
		org := seedOrganisation(t, db, user.UID)
		_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	}

	// Load first page (2 items)
	pageable := datastore.Pageable{
		PerPage:   2,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	orgs, paginationData, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	require.NoError(t, err)
	require.Len(t, orgs, 2)
	require.True(t, paginationData.HasNextPage)
	require.False(t, paginationData.HasPreviousPage)
}

func Test_LoadUserOrganisationsPaged_Backward(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data - user is member of 4 organisations
	user := seedUser(t, db, "backward-pagination@example.com")
	for i := 0; i < 4; i++ {
		org := seedOrganisation(t, db, user.UID)
		_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	}

	// Load first page
	pageableFirst := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Next,
	}
	pageableFirst.SetCursors()

	_, paginationFirst, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageableFirst)
	require.NoError(t, err)

	// Load next page
	pageableNext := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Next,
	}
	pageableNext.NextCursor = paginationFirst.NextPageCursor
	pageableNext.SetCursors()

	_, paginationNext, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageableNext)
	require.NoError(t, err)

	// Load previous page (backward)
	pageablePrev := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Prev,
	}
	pageablePrev.PrevCursor = paginationNext.PrevPageCursor
	pageablePrev.SetCursors()

	orgsPrev, paginationPrev, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageablePrev)
	require.NoError(t, err)
	require.Len(t, orgsPrev, 2)
	require.True(t, paginationPrev.HasNextPage)
}

func Test_LoadUserOrganisationsPaged_Empty(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed user with no organisation memberships
	user := seedUser(t, db, "no-orgs@example.com")

	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	orgs, paginationData, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	require.NoError(t, err)
	require.Empty(t, orgs)
	require.False(t, paginationData.HasNextPage)
	require.False(t, paginationData.HasPreviousPage)
}

func Test_LoadUserOrganisationsPaged_ExcludesDeletedOrgs(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)
	orgService := createOrganisationService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org1 := seedOrganisation(t, db, user.UID)
	org2 := seedOrganisation(t, db, user.UID)

	_ = seedOrganisationMember(t, db, org1.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org2.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Delete one organisation
	err := orgService.DeleteOrganisation(ctx, org1.UID)
	require.NoError(t, err)

	// Load user organisations
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	orgs, _, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, org2.UID, orgs[0].UID)
}

func Test_LoadUserOrganisationsPaged_ExcludesDeletedMembers(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org1 := seedOrganisation(t, db, user.UID)
	org2 := seedOrganisation(t, db, user.UID)

	member1 := seedOrganisationMember(t, db, org1.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org2.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Delete one membership
	err := service.DeleteOrganisationMember(ctx, member1.UID, org1.UID)
	require.NoError(t, err)

	// Load user organisations
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	orgs, _, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	require.NoError(t, err)
	require.Len(t, orgs, 1)
	require.Equal(t, org2.UID, orgs[0].UID)
}

func Test_LoadUserOrganisationsPaged_UserInMultipleOrgsWithDifferentRoles(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data - user has different roles in different orgs
	user := seedUser(t, db, "multi-role@example.com")
	org1 := seedOrganisation(t, db, user.UID)
	org2 := seedOrganisation(t, db, user.UID)
	org3 := seedOrganisation(t, db, user.UID)

	_ = seedOrganisationMember(t, db, org1.UID, user.UID, auth.Role{Type: auth.RoleInstanceAdmin})
	_ = seedOrganisationMember(t, db, org2.UID, user.UID, auth.Role{Type: auth.RoleOrganisationAdmin})
	_ = seedOrganisationMember(t, db, org3.UID, user.UID, auth.Role{Type: auth.RoleAPI})

	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	orgs, _, err := service.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	require.NoError(t, err)
	require.Len(t, orgs, 3)
}

func createOrganisationService(t *testing.T, db database.Database) datastore.OrganisationRepository {
	t.Helper()
	return organisations.New(log.NewLogger(os.Stdout), db)
}
