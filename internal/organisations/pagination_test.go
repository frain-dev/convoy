package organisations

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestLoadOrganisationsPaged_ForwardPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed 15 organisations
	for i := 0; i < 15; i++ {
		org := seedOrganisation(t, db, "", "")
		_ = org
	}

	// First page
	page1, pagination1, err := service.LoadOrganisationsPaged(ctx, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	})
	require.NoError(t, err)
	require.Len(t, page1, 5)
	require.True(t, pagination1.HasNextPage)
	require.False(t, pagination1.HasPreviousPage)

	// Second page using cursor
	page2, pagination2, err := service.LoadOrganisationsPaged(ctx, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: pagination1.NextPageCursor,
	})
	require.NoError(t, err)
	require.Len(t, page2, 5)
	require.True(t, pagination2.HasNextPage)
	require.True(t, pagination2.HasPreviousPage)

	// Verify no overlap
	for _, p1 := range page1 {
		for _, p2 := range page2 {
			require.NotEqual(t, p1.UID, p2.UID)
		}
	}
}

func TestLoadOrganisationsPaged_BackwardPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed 10 organisations
	for i := 0; i < 10; i++ {
		seedOrganisation(t, db, "", "")
	}

	// Get last page (forward)
	page1, pagination1, err := service.LoadOrganisationsPaged(ctx, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	})
	require.NoError(t, err)

	// Navigate to next page
	_, pagination2, err := service.LoadOrganisationsPaged(ctx, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: pagination1.NextPageCursor,
	})
	require.NoError(t, err)

	// Go back using previous cursor
	pageBack, paginationBack, err := service.LoadOrganisationsPaged(ctx, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Prev,
		PrevCursor: pagination2.PrevPageCursor,
	})
	require.NoError(t, err)

	// Should get same items as page1 (in same order)
	require.Len(t, pageBack, len(page1))
	for i := range page1 {
		require.Equal(t, page1[i].UID, pageBack[i].UID)
	}

	require.True(t, paginationBack.HasNextPage)
}

func TestLoadOrganisationsPaged_EmptyResults(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Don't seed any organisations, just query
	orgs, pagination, err := service.LoadOrganisationsPaged(ctx, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	})
	require.NoError(t, err)
	require.Empty(t, orgs)
	require.False(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
}

func TestLoadOrganisationsPaged_SinglePage(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed only 3 organisations
	for i := 0; i < 3; i++ {
		seedOrganisation(t, db, "", "")
	}

	// Request page of 5 (more than available)
	orgs, pagination, err := service.LoadOrganisationsPaged(ctx, datastore.Pageable{
		PerPage:    5,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	})
	require.NoError(t, err)
	require.Len(t, orgs, 3)
	require.False(t, pagination.HasNextPage)
	require.False(t, pagination.HasPreviousPage)
}

func TestLoadOrganisationsPagedWithSearch_NameMatch(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisations with specific names
	org1 := seedOrganisation(t, db, "", "")
	service.UpdateOrganisation(ctx, &datastore.Organisation{
		UID:     org1.UID,
		Name:    "Acme Corporation",
		OwnerID: org1.OwnerID,
	})

	org2 := seedOrganisation(t, db, "", "")
	service.UpdateOrganisation(ctx, &datastore.Organisation{
		UID:     org2.UID,
		Name:    "Acme Industries",
		OwnerID: org2.OwnerID,
	})

	org3 := seedOrganisation(t, db, "", "")
	service.UpdateOrganisation(ctx, &datastore.Organisation{
		UID:     org3.UID,
		Name:    "Different Company",
		OwnerID: org3.OwnerID,
	})

	// Search for "Acme"
	orgs, _, err := service.LoadOrganisationsPagedWithSearch(ctx, datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}, "Acme")

	require.NoError(t, err)
	require.GreaterOrEqual(t, len(orgs), 2) // At least our 2 Acme orgs

	// Verify search results contain "Acme"
	foundOrg1, foundOrg2 := false, false
	for _, org := range orgs {
		if org.UID == org1.UID {
			foundOrg1 = true
		}
		if org.UID == org2.UID {
			foundOrg2 = true
		}
	}
	require.True(t, foundOrg1)
	require.True(t, foundOrg2)
}

func TestLoadOrganisationsPagedWithSearch_IDMatch(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation
	org := seedOrganisation(t, db, "", "")

	// Search by partial ID (first few characters)
	searchTerm := org.UID[:6]
	orgs, _, err := service.LoadOrganisationsPagedWithSearch(ctx, datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}, searchTerm)

	require.NoError(t, err)
	require.NotEmpty(t, orgs)

	// Verify our organisation is in results
	found := false
	for _, o := range orgs {
		if o.UID == org.UID {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestLoadOrganisationsPagedWithSearch_NoResults(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed some organisations
	for i := 0; i < 5; i++ {
		seedOrganisation(t, db, "", "")
	}

	// Search for non-existent term
	orgs, pagination, err := service.LoadOrganisationsPagedWithSearch(ctx, datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}, "NonExistentSearchTerm12345")

	require.NoError(t, err)
	require.Empty(t, orgs)
	require.False(t, pagination.HasNextPage)
}

func TestLoadOrganisationsPagedWithSearch_PaginationWithSearch(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed 10 organisations with "SearchTest" in name
	for i := 0; i < 10; i++ {
		org := seedOrganisation(t, db, "", "")
		service.UpdateOrganisation(ctx, &datastore.Organisation{
			UID:     org.UID,
			Name:    fmt.Sprintf("SearchTest Organisation %d", i),
			OwnerID: org.OwnerID,
		})
	}

	// First page with search
	page1, pagination1, err := service.LoadOrganisationsPagedWithSearch(ctx, datastore.Pageable{
		PerPage:    3,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}, "SearchTest")

	require.NoError(t, err)
	require.Len(t, page1, 3)
	require.True(t, pagination1.HasNextPage)

	// Second page with same search
	page2, pagination2, err := service.LoadOrganisationsPagedWithSearch(ctx, datastore.Pageable{
		PerPage:    3,
		Direction:  datastore.Next,
		NextCursor: pagination1.NextPageCursor,
	}, "SearchTest")

	require.NoError(t, err)
	require.Len(t, page2, 3)
	require.True(t, pagination2.HasNextPage)

	// Verify no overlap
	for _, p1 := range page1 {
		for _, p2 := range page2 {
			require.NotEqual(t, p1.UID, p2.UID)
		}
	}
}

func TestLoadOrganisationsPaged_ExcludesDeletedOrganisations(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed 5 organisations
	orgs := make([]*datastore.Organisation, 5)
	for i := 0; i < 5; i++ {
		orgs[i] = seedOrganisation(t, db, "", "")
	}

	// Delete 2 organisations
	service.DeleteOrganisation(ctx, orgs[1].UID)
	service.DeleteOrganisation(ctx, orgs[3].UID)

	// Load all organisations
	allOrgs, _, err := service.LoadOrganisationsPaged(ctx, datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	})

	require.NoError(t, err)

	// Verify deleted organisations are not in results
	for _, org := range allOrgs {
		require.NotEqual(t, orgs[1].UID, org.UID)
		require.NotEqual(t, orgs[3].UID, org.UID)
	}
}
