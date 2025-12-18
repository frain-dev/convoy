package organisation_invites

import (
	"fmt"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestLoadOrganisationsInvitesPaged_EmptyResult(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	invites, paginationData, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Empty(t, invites)
	require.Equal(t, 0, len(invites))
	require.False(t, paginationData.HasNextPage)
	require.False(t, paginationData.HasPreviousPage)
}

func TestLoadOrganisationsInvitesPaged_SinglePage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create 5 invites
	for i := 0; i < 5; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	}

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	invites, paginationData, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, invites, 5)
	require.False(t, paginationData.HasNextPage)
}

func TestLoadOrganisationsInvitesPaged_MultiplePagesForward(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create 25 invites
	for i := 0; i < 25; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	}

	// First page
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	page1, pagination1, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, page1, 10)
	require.True(t, pagination1.HasNextPage)

	// Second page
	pageable.NextCursor = pagination1.NextPageCursor
	page2, pagination2, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, page2, 10)
	require.True(t, pagination2.HasNextPage)
	require.True(t, pagination2.HasPreviousPage)

	// Third page
	pageable.NextCursor = pagination2.NextPageCursor
	page3, pagination3, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, page3, 5)
	require.False(t, pagination3.HasNextPage)
	require.True(t, pagination3.HasPreviousPage)

	// Verify no duplicates across pages
	allIDs := make(map[string]bool)
	for _, invite := range page1 {
		allIDs[invite.UID] = true
	}
	for _, invite := range page2 {
		require.False(t, allIDs[invite.UID], "Duplicate invite found in page 2")
		allIDs[invite.UID] = true
	}
	for _, invite := range page3 {
		require.False(t, allIDs[invite.UID], "Duplicate invite found in page 3")
	}
}

func TestLoadOrganisationsInvitesPaged_BackwardPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create 25 invites
	for i := 0; i < 25; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	}

	// First get page 2 (forward)
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}
	page1, pagination1, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)

	pageable.NextCursor = pagination1.NextPageCursor
	page2, pagination2, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, page2, 10)

	// Now paginate backwards
	pageable.Direction = datastore.Prev
	pageable.PrevCursor = pagination2.PrevPageCursor
	pageBack, paginationBack, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, pageBack, 10)
	require.True(t, paginationBack.HasNextPage)

	// Verify we got the first page back
	require.Equal(t, page1[0].UID, pageBack[0].UID)
	require.Equal(t, page1[len(page1)-1].UID, pageBack[len(pageBack)-1].UID)
}

func TestLoadOrganisationsInvitesPaged_FilterByStatus(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create invites with different statuses
	for i := 0; i < 5; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	}
	for i := 0; i < 3; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusAccepted)
	}
	for i := 0; i < 2; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusDeclined)
	}

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	// Fetch pending invites
	pendingInvites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, pendingInvites, 5)
	for _, invite := range pendingInvites {
		require.Equal(t, datastore.InviteStatusPending, invite.Status)
	}

	// Fetch accepted invites
	acceptedInvites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusAccepted, pageable)
	require.NoError(t, err)
	require.Len(t, acceptedInvites, 3)
	for _, invite := range acceptedInvites {
		require.Equal(t, datastore.InviteStatusAccepted, invite.Status)
	}

	// Fetch declined invites
	declinedInvites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusDeclined, pageable)
	require.NoError(t, err)
	require.Len(t, declinedInvites, 2)
	for _, invite := range declinedInvites {
		require.Equal(t, datastore.InviteStatusDeclined, invite.Status)
	}

	// Fetch cancelled invites (should be empty)
	cancelledInvites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusCancelled, pageable)
	require.NoError(t, err)
	require.Empty(t, cancelledInvites)
}

func TestLoadOrganisationsInvitesPaged_IsolationBetweenOrganisations(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Create two organisations
	org1 := seedOrganisation(t, db)
	org2 := seedOrganisation(t, db)

	// Create invites for each organisation
	for i := 0; i < 5; i++ {
		seedOrganisationInvite(t, db, org1, datastore.InviteStatusPending)
	}
	for i := 0; i < 3; i++ {
		seedOrganisationInvite(t, db, org2, datastore.InviteStatusPending)
	}

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	// Fetch invites for org1
	org1Invites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org1.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, org1Invites, 5)
	for _, invite := range org1Invites {
		require.Equal(t, org1.UID, invite.OrganisationID)
	}

	// Fetch invites for org2
	org2Invites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org2.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, org2Invites, 3)
	for _, invite := range org2Invites {
		require.Equal(t, org2.UID, invite.OrganisationID)
	}
}

func TestLoadOrganisationsInvitesPaged_DeletedInvitesExcluded(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create 10 invites
	var invites []*datastore.OrganisationInvite
	for i := 0; i < 10; i++ {
		invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
		invites = append(invites, invite)
	}

	// Delete 3 invites
	for i := 0; i < 3; i++ {
		err := service.DeleteOrganisationInvite(ctx, invites[i].UID)
		require.NoError(t, err)
	}

	pageable := datastore.Pageable{
		PerPage:    20,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	// Fetch invites - should only get 7
	fetchedInvites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, fetchedInvites, 7)

	// Verify deleted invites are not in results
	deletedIDs := map[string]bool{
		invites[0].UID: true,
		invites[1].UID: true,
		invites[2].UID: true,
	}

	for _, invite := range fetchedInvites {
		require.False(t, deletedIDs[invite.UID], "Deleted invite should not be in results")
	}
}

func TestLoadOrganisationsInvitesPaged_VerifyPaginationMetadata(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create 15 invites
	for i := 0; i < 15; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	}

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	// First page
	_, pagination1, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.True(t, pagination1.HasNextPage)
	require.False(t, pagination1.HasPreviousPage)
	require.NotEmpty(t, pagination1.NextPageCursor)
	require.Equal(t, 0, pagination1.PrevRowCount.Count)

	// Second page
	pageable.NextCursor = pagination1.NextPageCursor
	_, pagination2, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.False(t, pagination2.HasNextPage)
	require.True(t, pagination2.HasPreviousPage)
	require.NotEmpty(t, pagination2.PrevPageCursor)
	require.Greater(t, pagination2.PrevRowCount.Count, 0)
}

func TestLoadOrganisationsInvitesPaged_ConsistentOrdering(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create 20 invites
	for i := 0; i < 20; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	}

	pageable := datastore.Pageable{
		PerPage:    20,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	// Fetch twice
	invites1, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)

	invites2, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)

	// Verify same order
	require.Len(t, invites1, len(invites2))
	for i := range invites1 {
		require.Equal(t, invites1[i].UID, invites2[i].UID, fmt.Sprintf("Order mismatch at position %d", i))
	}
}

func TestLoadOrganisationsInvitesPaged_LargeDataset(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create 100 invites
	for i := 0; i < 100; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	}

	pageable := datastore.Pageable{
		PerPage:    25,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	totalFetched := 0
	allIDs := make(map[string]bool)

	// Paginate through all
	for i := 0; i < 10; i++ {
		invites, pagination, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
		require.NoError(t, err)

		totalFetched += len(invites)

		// Check for duplicates
		for _, invite := range invites {
			require.False(t, allIDs[invite.UID], "Duplicate invite found")
			allIDs[invite.UID] = true
		}

		if !pagination.HasNextPage {
			break
		}

		pageable.NextCursor = pagination.NextPageCursor
	}

	require.Equal(t, 100, totalFetched)
	require.Len(t, allIDs, 100)
}

func TestLoadOrganisationsInvitesPaged_VerifyFieldsPopulated(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create invite
	seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	invites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.Len(t, invites, 1)

	invite := invites[0]
	require.NotEmpty(t, invite.UID)
	require.NotEmpty(t, invite.OrganisationID)
	require.NotEmpty(t, invite.InviteeEmail)
	require.NotEmpty(t, invite.Status)
	require.NotEmpty(t, invite.Role.Type)
	require.NotZero(t, invite.CreatedAt)
	require.NotZero(t, invite.UpdatedAt)
	require.NotZero(t, invite.ExpiresAt)
	// Note: Token should be empty in paginated results for security
	require.Empty(t, invite.Token)
}
