package portal_links

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestGetPortalLink_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create a portal link first
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Fetch the portal link
	portalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, createdPortalLink.UID, portalLink.UID)
	require.Equal(t, createdPortalLink.Name, portalLink.Name)
	require.Equal(t, createdPortalLink.OwnerID, portalLink.OwnerID)
	require.Equal(t, project.UID, portalLink.ProjectID)
	require.Equal(t, createdPortalLink.AuthType, portalLink.AuthType)
	require.Equal(t, createdPortalLink.CanManageEndpoint, portalLink.CanManageEndpoint)
	require.NotNil(t, portalLink.Endpoints)
	require.NotEmpty(t, portalLink.Token)
	require.NotZero(t, portalLink.CreatedAt)
	require.NotZero(t, portalLink.UpdatedAt)
}

func TestGetPortalLink_WithEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	ownerID := ulid.Make().String()
	endpoint1 := seedEndpoint(t, db, project, "")
	endpoint2 := seedEndpoint(t, db, project, "")

	// Create portal link with endpoints
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Portal Link With Endpoints",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID, endpoint2.UID},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Fetch the portal link
	portalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, 2, portalLink.EndpointCount)
	require.NotNil(t, portalLink.Endpoints)
}

func TestGetPortalLink_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Try to fetch a non-existent portal link
	portalLink, err := service.GetPortalLink(ctx, project.UID, "non-existent-id")

	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "portal link not found")
}

func TestGetPortalLink_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create a portal link
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Try to fetch with wrong project ID
	portalLink, err := service.GetPortalLink(ctx, "wrong-project-id", createdPortalLink.UID)

	require.Error(t, err)
	require.Nil(t, portalLink)
}
