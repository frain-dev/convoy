package portal_links

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestRevokePortalLink_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

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

	// Revoke the portal link
	err = service.RevokePortalLink(ctx, project.UID, createdPortalLink.UID)

	require.NoError(t, err)

	// Verify it no longer exists
	portalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)
	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "portal link not found")
}

func TestRevokePortalLink_WithEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	ownerID := ulid.Make().String()
	endpoint1 := seedEndpoint(t, db, project, "")

	// Create portal link with endpoints
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Portal Link With Endpoints",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Verify it was created
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)
	require.Equal(t, 1, fetchedPortalLink.EndpointCount)

	// Revoke the portal link
	err = service.RevokePortalLink(ctx, project.UID, createdPortalLink.UID)

	require.NoError(t, err)

	// Verify it no longer exists
	portalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)
	require.Error(t, err)
	require.Nil(t, portalLink)

	// Verify endpoint still exists (should not be deleted)
	endpointRepo := postgres.NewEndpointRepo(db)
	endpoint, err := endpointRepo.FindEndpointByID(ctx, endpoint1.UID, project.UID)
	require.NoError(t, err)
	require.NotNil(t, endpoint)
}

func TestRevokePortalLink_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Try to revoke non-existent portal link (should not error)
	err := service.RevokePortalLink(ctx, project.UID, "non-existent-id")

	// The implementation doesn't error on non-existent portal links
	require.NoError(t, err)
}

func TestRevokePortalLink_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

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

	// Try to revoke with wrong project ID (should not delete)
	err = service.RevokePortalLink(ctx, "wrong-project-id", createdPortalLink.UID)
	require.NoError(t, err)

	// Verify it still exists in the correct project
	portalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)
	require.NotNil(t, portalLink)
}
