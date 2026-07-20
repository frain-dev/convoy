package portal_links

import (
	"encoding/json"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func TestGetPortalLink_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.New("convoy", log.LevelInfo)
	service := New(logger, db)

	// Create a portal link first
	createRequest := &datastore.CreatePortalLinkRequest{
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

	logger := log.New("convoy", log.LevelInfo)
	service := New(logger, db)

	ownerID := ulid.Make().String()
	endpoint1 := seedEndpoint(t, db, project, "")
	endpoint2 := seedEndpoint(t, db, project, "")

	// Create portal link with endpoints
	createRequest := &datastore.CreatePortalLinkRequest{
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

func TestGetPortalLink_EndpointsMetadata_EmptyWhenNoEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.New("convoy", log.LevelInfo)
	service := New(logger, db)

	// Create a portal link with no endpoints
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link Without Endpoints",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Fetch by id
	portalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)
	require.NotNil(t, portalLink.EndpointsMetadata)
	require.Empty(t, portalLink.EndpointsMetadata, "endpoints_metadata must be [] not [null] for a link with no endpoints")

	metadataJSON, err := json.Marshal(portalLink.EndpointsMetadata)
	require.NoError(t, err)
	require.Equal(t, "[]", string(metadataJSON))

	// Fetch by token
	byToken, err := service.GetPortalLinkByToken(ctx, portalLink.Token)
	require.NoError(t, err)
	require.NotNil(t, byToken.EndpointsMetadata)
	require.Empty(t, byToken.EndpointsMetadata)

	// Fetch by owner id
	byOwner, err := service.GetPortalLinkByOwnerID(ctx, project.UID, createRequest.OwnerID)
	require.NoError(t, err)
	require.NotNil(t, byOwner.EndpointsMetadata)
	require.Empty(t, byOwner.EndpointsMetadata)

	// Fetch many by owner id
	byOwnerMany, err := service.FindPortalLinksByOwnerID(ctx, createRequest.OwnerID)
	require.NoError(t, err)
	require.Len(t, byOwnerMany, 1)
	require.NotNil(t, byOwnerMany[0].EndpointsMetadata)
	require.Empty(t, byOwnerMany[0].EndpointsMetadata)
}

func TestGetPortalLink_EndpointsMetadata_WithEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.New("convoy", log.LevelInfo)
	service := New(logger, db)

	ownerID := ulid.Make().String()
	endpoint1 := seedEndpoint(t, db, project, "")
	endpoint2 := seedEndpoint(t, db, project, "")

	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link Metadata With Endpoints",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeStaticToken),
		CanManageEndpoint: false,
		Endpoints:         []string{endpoint1.UID, endpoint2.UID},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	portalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)
	require.Len(t, portalLink.EndpointsMetadata, 2)

	metadataUIDs := make([]string, 0, len(portalLink.EndpointsMetadata))
	for _, endpoint := range portalLink.EndpointsMetadata {
		require.NotNil(t, endpoint, "endpoints_metadata must not contain null elements")
		metadataUIDs = append(metadataUIDs, endpoint.UID)
	}
	require.ElementsMatch(t, []string{endpoint1.UID, endpoint2.UID}, metadataUIDs)
}

func TestGetPortalLink_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.New("convoy", log.LevelInfo)
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

	logger := log.New("convoy", log.LevelInfo)
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

	// Try to fetch with wrong project ID
	portalLink, err := service.GetPortalLink(ctx, "wrong-project-id", createdPortalLink.UID)

	require.Error(t, err)
	require.Nil(t, portalLink)
}

func TestGetPortalLink_WithRefreshTokenAuthType_GeneratesAuthToken(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.New("convoy", log.LevelInfo)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create a portal link with refresh token auth type
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link With Refresh Token",
		OwnerID:           ownerID,
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Fetch the portal link - should provision a new auth token
	portalLink, err := service.GetPortalLink(ctx, project.UID, createdPortalLink.UID)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.Equal(t, datastore.PortalAuthTypeRefreshToken, portalLink.AuthType)

	// Verify that auth_key is provisioned when fetching a refresh token portal link
	require.NotEmpty(t, portalLink.AuthKey, "auth_key should be provisioned for refresh token auth type")
	require.Contains(t, portalLink.AuthKey, "PRT.", "auth_key should have the correct prefix")
}

func TestGetPortalLink_WithStaticTokenAuthType_NoAuthKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.New("convoy", log.LevelInfo)
	service := New(logger, db)

	ownerID := ulid.Make().String()

	// Create a portal link with static token auth type
	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Portal Link With Static Token",
		OwnerID:           ownerID,
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
	require.Equal(t, datastore.PortalAuthTypeStaticToken, portalLink.AuthType)

	// Verify that auth_key is NOT provisioned for static token types
	require.Empty(t, portalLink.AuthKey, "auth_key should not be provisioned for static token auth type")
}
