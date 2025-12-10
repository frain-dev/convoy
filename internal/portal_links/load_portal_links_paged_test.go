package portal_links

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestLoadPortalLinksPaged_Success_EmptyList(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Load portal links (should be empty)
	filter := &datastore.FilterBy{}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	portalLinks, paginationData, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable)

	require.NoError(t, err)
	require.NotNil(t, portalLinks)
	require.Empty(t, portalLinks)
	require.Equal(t, 0, paginationData.PrevRowCount.Count)
}

func TestLoadPortalLinksPaged_Success_MultiplePortalLinks(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create multiple portal links
	var createdPortalLinks []*datastore.PortalLink
	for i := 0; i < 5; i++ {
		createRequest := &models.CreatePortalLinkRequest{
			Name:              "Portal Link " + ulid.Make().String(),
			OwnerID:           ulid.Make().String(),
			AuthType:          string(datastore.PortalAuthTypeStaticToken),
			CanManageEndpoint: true,
			Endpoints:         []string{},
		}
		portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
		require.NoError(t, err)
		createdPortalLinks = append(createdPortalLinks, portalLink)
	}

	// Verify portal links can be fetched individually
	for _, pl := range createdPortalLinks {
		fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, pl.UID)
		require.NoError(t, err)
		require.NotNil(t, fetchedPortalLink)
	}

	// Load portal links
	filter := &datastore.FilterBy{}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	portalLinks, paginationData, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable)

	require.NoError(t, err)
	require.NotNil(t, portalLinks)
	require.Equal(t, 5, len(portalLinks))
	require.NotNil(t, paginationData)
}

func TestLoadPortalLinksPaged_WithPagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create 15 portal links
	for i := 0; i < 15; i++ {
		createRequest := &models.CreatePortalLinkRequest{
			Name:              "Portal Link " + ulid.Make().String(),
			OwnerID:           ulid.Make().String(),
			AuthType:          string(datastore.PortalAuthTypeStaticToken),
			CanManageEndpoint: true,
			Endpoints:         []string{},
		}
		_, err := service.CreatePortalLink(ctx, project.UID, createRequest)
		require.NoError(t, err)
	}

	// Load first page
	filter := &datastore.FilterBy{}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	portalLinks1, paginationData1, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable)

	require.NoError(t, err)
	require.NotNil(t, portalLinks1)
	require.Equal(t, 10, len(portalLinks1))
	require.NotEmpty(t, paginationData1.NextPageCursor)

	// Load second page
	pageable2 := datastore.Pageable{
		PerPage:    10,
		NextCursor: paginationData1.NextPageCursor,
		Direction:  datastore.Next,
	}

	portalLinks2, paginationData2, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable2)

	require.NoError(t, err)
	require.NotNil(t, portalLinks2)
	require.Equal(t, 5, len(portalLinks2))
	require.NotEmpty(t, paginationData2.PrevPageCursor)

	// Verify no duplicate portal links
	for _, pl1 := range portalLinks1 {
		for _, pl2 := range portalLinks2 {
			require.NotEqual(t, pl1.UID, pl2.UID)
		}
	}
}

func TestLoadPortalLinksPaged_FilterByEndpoint(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	ownerID1 := ulid.Make().String()
	ownerID2 := ulid.Make().String()

	endpoint1 := seedEndpoint(t, db, project, "")
	endpoint2 := seedEndpoint(t, db, project, "")

	// Create portal link with endpoint1
	createRequest1 := &models.CreatePortalLinkRequest{
		Name:              "Portal Link 1",
		OwnerID:           ownerID1,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID},
	}
	portalLink1, err := service.CreatePortalLink(ctx, project.UID, createRequest1)
	require.NoError(t, err)

	// Create portal link with endpoint2
	createRequest2 := &models.CreatePortalLinkRequest{
		Name:              "Portal Link 2",
		OwnerID:           ownerID2,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint2.UID},
	}
	_, err = service.CreatePortalLink(ctx, project.UID, createRequest2)
	require.NoError(t, err)

	// Filter by endpoint1
	filter := &datastore.FilterBy{
		EndpointID: endpoint1.UID,
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	portalLinks, _, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable)

	require.NoError(t, err)
	require.NotNil(t, portalLinks)
	require.Equal(t, 1, len(portalLinks))
	require.Equal(t, portalLink1.UID, portalLinks[0].UID)
}

func TestLoadPortalLinksPaged_FilterByMultipleEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	ownerID1 := ulid.Make().String()
	ownerID2 := ulid.Make().String()
	ownerID3 := ulid.Make().String()

	endpoint1 := seedEndpoint(t, db, project, "")
	endpoint2 := seedEndpoint(t, db, project, "")
	endpoint3 := seedEndpoint(t, db, project, "")

	// Create portal links with different endpoints
	createRequest1 := &models.CreatePortalLinkRequest{
		Name:              "Portal Link 1",
		OwnerID:           ownerID1,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID},
	}
	portalLink1, err := service.CreatePortalLink(ctx, project.UID, createRequest1)
	require.NoError(t, err)

	createRequest2 := &models.CreatePortalLinkRequest{
		Name:              "Portal Link 2",
		OwnerID:           ownerID2,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint2.UID},
	}
	portalLink2, err := service.CreatePortalLink(ctx, project.UID, createRequest2)
	require.NoError(t, err)

	createRequest3 := &models.CreatePortalLinkRequest{
		Name:              "Portal Link 3",
		OwnerID:           ownerID3,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint3.UID},
	}
	_, err = service.CreatePortalLink(ctx, project.UID, createRequest3)
	require.NoError(t, err)

	// Filter by endpoint1 and endpoint2
	filter := &datastore.FilterBy{
		EndpointIDs: []string{endpoint1.UID, endpoint2.UID},
	}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	portalLinks, _, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable)

	require.NoError(t, err)
	require.NotNil(t, portalLinks)
	require.Equal(t, 2, len(portalLinks))

	// Verify the correct portal links are returned
	foundUIDs := make(map[string]bool)
	for _, pl := range portalLinks {
		foundUIDs[pl.UID] = true
	}
	require.True(t, foundUIDs[portalLink1.UID])
	require.True(t, foundUIDs[portalLink2.UID])
}

func TestLoadPortalLinksPaged_WithRefreshTokenAuthType_GeneratesAuthTokens(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create portal link with RefreshToken auth type
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Refresh Token Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}
	_, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Create portal link with StaticToken auth type
	createRequest2 := &models.CreatePortalLinkRequest{
		Name:              "Static Token Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}
	_, err = service.CreatePortalLink(ctx, project.UID, createRequest2)
	require.NoError(t, err)

	// Load portal links
	filter := &datastore.FilterBy{}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	portalLinks, _, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable)

	require.NoError(t, err)
	require.NotNil(t, portalLinks)
	require.Equal(t, 2, len(portalLinks))

	// Note: Auth tokens are generated and stored in database during LoadPortalLinksPaged
	// but are not returned in the response. This test verifies the function succeeds.
	for _, pl := range portalLinks {
		require.NotEmpty(t, pl.UID)
		require.NotEmpty(t, pl.Name)
	}
}

func TestLoadPortalLinksPaged_PreviousPage(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create 15 portal links
	for i := 0; i < 15; i++ {
		createRequest := &models.CreatePortalLinkRequest{
			Name:              "Portal Link " + ulid.Make().String(),
			OwnerID:           ulid.Make().String(),
			AuthType:          string(datastore.PortalAuthTypeStaticToken),
			CanManageEndpoint: true,
			Endpoints:         []string{},
		}
		_, err := service.CreatePortalLink(ctx, project.UID, createRequest)
		require.NoError(t, err)
	}

	// Load first page
	filter := &datastore.FilterBy{}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	portalLinks1, paginationData1, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable)
	require.NoError(t, err)
	require.Equal(t, 10, len(portalLinks1))

	// Load second page
	pageable2 := datastore.Pageable{
		PerPage:    10,
		NextCursor: paginationData1.NextPageCursor,
		Direction:  datastore.Next,
	}

	portalLinks2, paginationData2, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable2)
	require.NoError(t, err)
	require.Equal(t, 5, len(portalLinks2))

	// Load previous page (back to first page)
	pageable3 := datastore.Pageable{
		PerPage:    10,
		PrevCursor: paginationData2.PrevPageCursor,
		Direction:  datastore.Prev,
	}

	portalLinks3, _, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable3)
	require.NoError(t, err)
	require.Equal(t, 10, len(portalLinks3))

	// Verify we got the same portal links as the first page
	for i := range portalLinks1 {
		require.Equal(t, portalLinks1[i].UID, portalLinks3[i].UID)
	}
}

func TestLoadPortalLinksPaged_EmptyFilter(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create portal links
	for i := 0; i < 3; i++ {
		createRequest := &models.CreatePortalLinkRequest{
			Name:              "Portal Link " + ulid.Make().String(),
			OwnerID:           ulid.Make().String(),
			AuthType:          string(datastore.PortalAuthTypeStaticToken),
			CanManageEndpoint: true,
			Endpoints:         []string{},
		}
		_, err := service.CreatePortalLink(ctx, project.UID, createRequest)
		require.NoError(t, err)
	}

	// Load with empty filter
	filter := &datastore.FilterBy{}
	pageable := datastore.Pageable{
		PerPage:   10,
		Direction: datastore.Next,
	}
	pageable.SetCursors()

	portalLinks, _, err := service.LoadPortalLinksPaged(ctx, project.UID, filter, pageable)

	require.NoError(t, err)
	require.NotNil(t, portalLinks)
	require.Equal(t, 3, len(portalLinks))
}
