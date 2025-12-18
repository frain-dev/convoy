package portal_links

import (
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestGetPortalLinkByToken_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	// Create a portal link
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Get the token from the database
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)
	require.NotEmpty(t, fetchedPortalLink.Token)

	// Fetch by token
	portalLink, err := service.GetPortalLinkByToken(ctx, fetchedPortalLink.Token)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, createdPortalLink.UID, portalLink.UID)
	require.Equal(t, createdPortalLink.Name, portalLink.Name)
	require.Equal(t, fetchedPortalLink.Token, portalLink.Token)
}

func TestGetPortalLinkByToken_WithEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()
	endpoint1 := seedEndpoint(t, db, project, "")

	// Create portal link with endpoints
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link With Endpoints",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{endpoint1.UID},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Get the token
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)

	// Fetch by token
	portalLink, err := service.GetPortalLinkByToken(ctx, fetchedPortalLink.Token)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, 1, portalLink.EndpointCount)
	require.Equal(t, datastore.PortalAuthTypeRefreshToken, portalLink.AuthType)
}

func TestGetPortalLinkByToken_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	// Try to fetch with non-existent token
	portalLink, err := service.GetPortalLinkByToken(ctx, "non-existent-token")

	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "portal link not found")
}
