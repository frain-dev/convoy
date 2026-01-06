package portal_links

import (
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestUpdatePortalLink_ValidRequest(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	// Create initial portal link
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Original Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Update portal link
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           portalLink.OwnerID,
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: false,
		Endpoints:         []string{},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.NoError(t, err)
	require.NotNil(t, updatedPortalLink)
	require.Equal(t, portalLink.UID, updatedPortalLink.UID)
	require.Equal(t, updateRequest.Name, updatedPortalLink.Name)
	require.Equal(t, updateRequest.OwnerID, updatedPortalLink.OwnerID)
	require.Equal(t, datastore.PortalAuthType(updateRequest.AuthType), updatedPortalLink.AuthType)
	require.Equal(t, updateRequest.CanManageEndpoint, updatedPortalLink.CanManageEndpoint)
}

func TestUpdatePortalLink_WithNewEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create initial portal link with one endpoint
	endpoint1 := seedEndpoint(t, db, project, "")
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Create new endpoints to replace the old one
	endpoint2 := seedEndpoint(t, db, project, "")
	endpoint3 := seedEndpoint(t, db, project, "")

	// Update portal link with different endpoints
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint2.UID, endpoint3.UID},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.NoError(t, err)
	require.NotNil(t, updatedPortalLink)
	require.Equal(t, 2, len(updatedPortalLink.Endpoints))
	require.Contains(t, updatedPortalLink.Endpoints, endpoint2.UID)
	require.Contains(t, updatedPortalLink.Endpoints, endpoint3.UID)
	require.NotContains(t, updatedPortalLink.Endpoints, endpoint1.UID)

	// Verify endpoints have correct owner_id
	endpointRepo := postgres.NewEndpointRepo(db)
	updatedEndpoint2, err := endpointRepo.FindEndpointByID(ctx, endpoint2.UID, project.UID)
	require.NoError(t, err)
	require.Equal(t, ownerID, updatedEndpoint2.OwnerID)

	updatedEndpoint3, err := endpointRepo.FindEndpointByID(ctx, endpoint3.UID, project.UID)
	require.NoError(t, err)
	require.Equal(t, ownerID, updatedEndpoint3.OwnerID)
}

func TestUpdatePortalLink_WithEndpoints_AlreadyHaveOwnerID(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create initial portal link
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Seed endpoint that already has the same owner_id
	endpoint1 := seedEndpoint(t, db, project, ownerID)

	// Update portal link with endpoint that has matching owner_id
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.NoError(t, err)
	require.NotNil(t, updatedPortalLink)
	require.Contains(t, updatedPortalLink.Endpoints, endpoint1.UID)
}

func TestUpdatePortalLink_WithEndpoints_DifferentOwnerID_ShouldFail(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()
	differentOwnerID := ulid.Make().String()

	// Create initial portal link
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Seed endpoint that has a different owner_id
	endpoint1 := seedEndpoint(t, db, project, differentOwnerID)

	// Try to update portal link with endpoint that has different owner_id
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.Error(t, err)
	require.Nil(t, updatedPortalLink)
	require.Contains(t, err.Error(), "already has owner_id")
}

func TestUpdatePortalLink_RemoveAllEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create portal link with endpoints
	endpoint1 := seedEndpoint(t, db, project, ownerID)
	endpoint2 := seedEndpoint(t, db, project, ownerID)
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID, endpoint2.UID},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Update to remove all endpoints - but since owner_id is provided and endpoints have this owner_id,
	// the system will automatically re-link them
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{}, // Empty endpoints
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.NoError(t, err)
	require.NotNil(t, updatedPortalLink)
	require.Empty(t, updatedPortalLink.Endpoints) // Response has empty endpoints

	// Verify by fetching from database - endpoints are auto-linked by owner_id
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, portalLink.UID)
	require.NoError(t, err)
	require.Equal(t, 2, fetchedPortalLink.EndpointCount) // Endpoints are re-linked
}

func TestUpdatePortalLink_ChangeOwnerID_AutoLinkEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID1 := ulid.Make().String()
	ownerID2 := ulid.Make().String()

	// Create portal link with first owner_id
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ownerID1,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Seed endpoints with the new owner_id
	_ = seedEndpoint(t, db, project, ownerID2)
	_ = seedEndpoint(t, db, project, ownerID2)

	// Update portal link with new owner_id (should auto-link endpoints)
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           ownerID2,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{}, // No endpoints provided
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.NoError(t, err)
	require.NotNil(t, updatedPortalLink)
	require.Equal(t, ownerID2, updatedPortalLink.OwnerID)
	require.Empty(t, updatedPortalLink.Endpoints) // Response has empty endpoints

	// Verify by fetching from database - should have linked the endpoints
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, portalLink.UID)
	require.NoError(t, err)
	require.Equal(t, 2, fetchedPortalLink.EndpointCount)
}

func TestUpdatePortalLink_InvalidRequest_InvalidAuthType(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	// Create initial portal link
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Original Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Try to update with invalid auth type
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           portalLink.OwnerID,
		AuthType:          "invalid_auth_type", // Invalid auth type
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.Error(t, err)
	require.Nil(t, updatedPortalLink)
	require.Contains(t, err.Error(), "invalid auth type")
}

func TestUpdatePortalLink_EndpointNotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	// Create initial portal link
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Original Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Try to update with non-existent endpoint
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           portalLink.OwnerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{"non-existent-endpoint-id"},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.Error(t, err)
	require.Nil(t, updatedPortalLink)
	require.Contains(t, err.Error(), "failed to find endpoint")
}

func TestUpdatePortalLink_MultipleEndpoints_SomeInvalid(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create initial portal link
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Original Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Seed one valid endpoint
	endpoint1 := seedEndpoint(t, db, project, "")

	// Try to update with mixed valid and invalid endpoints
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Updated Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID, "non-existent-endpoint-id"},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	// Should fail because one endpoint doesn't exist
	require.Error(t, err)
	require.Nil(t, updatedPortalLink)
}

func TestUpdatePortalLink_VerifyDatabasePersistence(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	// Create initial portal link
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Original Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Update portal link
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Persistence Test Updated",
		OwnerID:           portalLink.OwnerID,
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: false,
		Endpoints:         []string{},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)
	require.NoError(t, err)

	// Fetch from database to verify persistence
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, portalLink.UID)
	require.NoError(t, err)
	require.Equal(t, updatedPortalLink.UID, fetchedPortalLink.UID)
	require.Equal(t, updatedPortalLink.Name, fetchedPortalLink.Name)
	require.Equal(t, updatedPortalLink.OwnerID, fetchedPortalLink.OwnerID)
	require.Equal(t, updatedPortalLink.AuthType, fetchedPortalLink.AuthType)
	require.Equal(t, updatedPortalLink.CanManageEndpoint, fetchedPortalLink.CanManageEndpoint)
}

func TestUpdatePortalLink_ChangeAuthType(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	// Create portal link with StaticToken auth type
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)
	require.Equal(t, datastore.PortalAuthTypeStaticToken, portalLink.AuthType)

	// Update to RefreshToken auth type
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           portalLink.OwnerID,
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)
	require.NoError(t, err)
	require.Equal(t, datastore.PortalAuthTypeRefreshToken, updatedPortalLink.AuthType)

	// Verify in database
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, portalLink.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.PortalAuthTypeRefreshToken, fetchedPortalLink.AuthType)
}

func TestUpdatePortalLink_AddEndpointsToExisting(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.NewLogger(os.Stdout)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create portal link without endpoints
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{},
	}

	portalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Create endpoints to add
	endpoint1 := seedEndpoint(t, db, project, "")
	endpoint2 := seedEndpoint(t, db, project, "")

	// Update to add endpoints
	updateRequest := &datastore.UpdatePortalLinkRequest{
		Name:              "Portal Link",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID, endpoint2.UID},
	}

	updatedPortalLink, err := service.UpdatePortalLink(ctx, project.UID, portalLink, updateRequest)

	require.NoError(t, err)
	require.NotNil(t, updatedPortalLink)
	require.Equal(t, 2, len(updatedPortalLink.Endpoints))
	require.Contains(t, updatedPortalLink.Endpoints, endpoint1.UID)
	require.Contains(t, updatedPortalLink.Endpoints, endpoint2.UID)

	// Verify in database
	fetchedPortalLink, err := service.GetPortalLink(ctx, project.UID, portalLink.UID)
	require.NoError(t, err)
	require.Equal(t, 2, fetchedPortalLink.EndpointCount)
}
