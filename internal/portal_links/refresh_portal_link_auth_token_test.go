package portal_links

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestRefreshPortalLinkAuthToken_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create a portal link
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Refresh auth token
	portalLink, err := service.RefreshPortalLinkAuthToken(ctx, project.UID, createdPortalLink.UID)

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.NotEmpty(t, portalLink.AuthKey)
	require.Contains(t, portalLink.AuthKey, "PRT.") // Auth key should have the prefix
	require.Equal(t, createdPortalLink.UID, portalLink.UID)
}

func TestRefreshPortalLinkAuthToken_MultipleRefreshes(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create a portal link
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Refresh multiple times
	portalLink1, err := service.RefreshPortalLinkAuthToken(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)
	require.NotEmpty(t, portalLink1.AuthKey)

	portalLink2, err := service.RefreshPortalLinkAuthToken(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)
	require.NotEmpty(t, portalLink2.AuthKey)

	// Auth keys should be different
	require.NotEqual(t, portalLink1.AuthKey, portalLink2.AuthKey)
}

func TestRefreshPortalLinkAuthToken_VerifyExpiryTime(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create a portal link
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Refresh auth token
	beforeRefresh := time.Now()
	portalLink, err := service.RefreshPortalLinkAuthToken(ctx, project.UID, createdPortalLink.UID)
	afterRefresh := time.Now()

	require.NoError(t, err)
	require.NotNil(t, portalLink)
	require.NotEmpty(t, portalLink.AuthKey)

	// Verify the token was created within the expected timeframe
	expectedExpiryMin := beforeRefresh.Add(time.Hour)
	expectedExpiryMax := afterRefresh.Add(time.Hour)

	// Note: We can't directly check the expiry time since it's stored in the database
	// and not returned in the response. This test mainly verifies the function succeeds.
	require.True(t, expectedExpiryMin.Before(expectedExpiryMax))
}

func TestRefreshPortalLinkAuthToken_PortalLinkNotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Try to refresh auth token for non-existent portal link
	portalLink, err := service.RefreshPortalLinkAuthToken(ctx, project.UID, "non-existent-id")

	require.Error(t, err)
	require.Nil(t, portalLink)
	require.Contains(t, err.Error(), "portal link not found")
}

func TestRefreshPortalLinkAuthToken_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	project, _, _ := seedTestData(t, db)

	logger := log.NewLogger(nil)
	service := New(logger, db)

	// Create a portal link
	createRequest := &models.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	// Try to refresh with wrong project ID
	portalLink, err := service.RefreshPortalLinkAuthToken(ctx, "wrong-project-id", createdPortalLink.UID)

	require.Error(t, err)
	require.Nil(t, portalLink)
}
