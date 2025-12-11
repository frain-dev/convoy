package portal_links

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/portal_links/models"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestGetPortalLinkByOwnerID_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create a portal link
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Fetch by owner ID
	portalLink, err := service.GetPortalLinkByOwnerID(ctx, project.UID, ownerID)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, createdPortalLink.UID, portalLink.UID)
	require.Equal(t, ownerID, portalLink.OwnerID)
	require.Equal(t, createdPortalLink.Name, portalLink.Name)
}

func TestGetPortalLinkByOwnerID_WithEndpointsAutoLinked(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create endpoints with owner_id
	_ = seedEndpoint(t, db, project, ownerID)
	_ = seedEndpoint(t, db, project, ownerID)
	_ = seedEndpoint(t, db, project, ownerID)

	// Create portal link without specifying endpoints (should auto-link by owner_id)
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Fetch by owner ID
	portalLink, err := service.GetPortalLinkByOwnerID(ctx, project.UID, ownerID)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, createdPortalLink.UID, portalLink.UID)
	require.Equal(t, 3, portalLink.EndpointCount)
}

func TestGetPortalLinkByOwnerID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Try to fetch with non-existent owner ID
	portalLink, err := service.GetPortalLinkByOwnerID(ctx, project.UID, "non-existent-owner-id")

	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "portal link not found")
}

func TestGetPortalLinkByOwnerID_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create a portal link
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	_, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Try to fetch with wrong project ID
	portalLink, err := service.GetPortalLinkByOwnerID(ctx, "wrong-project-id", ownerID)

	require.Error(t, err)
	require.Nil(t, portalLink)
}
