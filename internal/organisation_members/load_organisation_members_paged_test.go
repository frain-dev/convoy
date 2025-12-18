package organisation_members

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
)

func Test_LoadOrganisationMembersPaged_Forward_NoFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user1 := seedUser(t, db, "user1@example.com")
	user2 := seedUser(t, db, "user2@example.com")
	user3 := seedUser(t, db, "user3@example.com")
	org := seedOrganisation(t, db, user1.UID)

	_ = seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user3.UID, auth.Role{Type: auth.RoleAPI})

	// Load first page
	pageable := datastore.Pageable{
		PerPage:   2,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	members, paginationData, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageable)
	require.NoError(t, err)
	require.Len(t, members, 2)
	require.True(t, paginationData.HasNextPage)
	require.False(t, paginationData.HasPreviousPage)
}

func Test_LoadOrganisationMembersPaged_Forward_WithUserFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user1 := seedUser(t, db, "user1@example.com")
	user2 := seedUser(t, db, "user2@example.com")
	user3 := seedUser(t, db, "user3@example.com")
	org := seedOrganisation(t, db, user1.UID)

	_ = seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user3.UID, auth.Role{Type: auth.RoleAPI})

	// Load with user filter
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	members, _, err := service.LoadOrganisationMembersPaged(ctx, org.UID, user2.UID, pageable)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, user2.UID, members[0].UserID)
}

func Test_LoadOrganisationMembersPaged_Backward(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user1 := seedUser(t, db, "user1@example.com")
	user2 := seedUser(t, db, "user2@example.com")
	user3 := seedUser(t, db, "user3@example.com")
	user4 := seedUser(t, db, "user4@example.com")
	org := seedOrganisation(t, db, user1.UID)

	_ = seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user3.UID, auth.Role{Type: auth.RoleAPI})
	_ = seedOrganisationMember(t, db, org.UID, user4.UID, auth.Role{Type: auth.RoleAPI})

	// Load first page to get cursor
	pageableFirst := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Next,
	}
	pageableFirst.SetCursors()

	membersFirst, paginationFirst, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageableFirst)
	require.NoError(t, err)
	require.Len(t, membersFirst, 2)

	// Load next page
	pageableNext := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Next,
	}
	pageableNext.NextCursor = paginationFirst.NextPageCursor
	pageableNext.SetCursors()

	membersNext, paginationNext, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageableNext)
	require.NoError(t, err)
	require.NotEmpty(t, membersNext)

	// Load previous page (backward)
	pageablePrev := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Prev,
	}
	pageablePrev.PrevCursor = paginationNext.PrevPageCursor
	pageablePrev.SetCursors()

	membersPrev, paginationPrev, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageablePrev)
	require.NoError(t, err)
	require.Len(t, membersPrev, 2)
	require.True(t, paginationPrev.HasNextPage)
}

func Test_LoadOrganisationMembersPaged_Empty(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data without members
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)

	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	members, paginationData, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageable)
	require.NoError(t, err)
	require.Empty(t, members)
	require.False(t, paginationData.HasNextPage)
	require.False(t, paginationData.HasPreviousPage)
}

func Test_LoadOrganisationMembersPaged_ExcludesDeleted(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user1 := seedUser(t, db, "user1@example.com")
	user2 := seedUser(t, db, "user2@example.com")
	user3 := seedUser(t, db, "user3@example.com")
	org := seedOrganisation(t, db, user1.UID)

	member1 := seedOrganisationMember(t, db, org.UID, user1.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user2.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org.UID, user3.UID, auth.Role{Type: auth.RoleAPI})

	// Delete one member
	err := service.DeleteOrganisationMember(ctx, member1.UID, org.UID)
	require.NoError(t, err)

	// Load members
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	members, _, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageable)
	require.NoError(t, err)
	require.Len(t, members, 2) // Only 2 active members
}

func Test_LoadOrganisationMembersPaged_IncludesUserMetadata(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "test-with-metadata@example.com")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
		Sort:      "",
	}
	pageable.SetCursors()

	members, _, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageable)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.NotEmpty(t, members[0].UserMetadata.Email)
	require.Equal(t, "test-with-metadata@example.com", members[0].UserMetadata.Email)
	require.Equal(t, "Test", members[0].UserMetadata.FirstName)
}

func Test_LoadOrganisationMembersPaged_MultiplePages(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data with 5 members
	org := seedOrganisation(t, db, seedUser(t, db, "owner@example.com").UID)
	for i := 0; i < 5; i++ {
		user := seedUser(t, db, "")
		_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	}

	// Load page 1 (2 items)
	pageable1 := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Next,
	}
	pageable1.SetCursors()

	members1, pagination1, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageable1)
	require.NoError(t, err)
	require.Len(t, members1, 2)
	require.True(t, pagination1.HasNextPage)
	require.False(t, pagination1.HasPreviousPage)

	// Load page 2
	pageable2 := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Next,
	}
	pageable2.NextCursor = pagination1.NextPageCursor
	pageable2.SetCursors()

	members2, pagination2, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageable2)
	require.NoError(t, err)
	require.Len(t, members2, 2)
	require.True(t, pagination2.HasNextPage)
	require.True(t, pagination2.HasPreviousPage)

	// Load page 3 (last page with 1 item)
	pageable3 := datastore.Pageable{
		PerPage:   2,
		Sort:      "",
		Direction: datastore.Next,
	}
	pageable3.NextCursor = pagination2.NextPageCursor
	pageable3.SetCursors()

	members3, pagination3, err := service.LoadOrganisationMembersPaged(ctx, org.UID, "", pageable3)
	require.NoError(t, err)
	require.Len(t, members3, 1)
	require.False(t, pagination3.HasNextPage)
	require.True(t, pagination3.HasPreviousPage)
}
