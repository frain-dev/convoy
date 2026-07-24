package portal_links

import (
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// TestFindPortalLinkByMaskId_RevokedLinkRejected verifies that once a portal link
// is revoked (soft-deleted), its refresh-token mask id no longer resolves. The
// portal realm authenticates refresh tokens via FindPortalLinkByMaskId, so a mask
// id that survives revocation would keep authenticating until token expiry.
func TestFindPortalLinkByMaskId_RevokedLinkRejected(t *testing.T) {
	db, ctx := setupTestDB(t)
	project := seedTestData(t, db)

	logger := log.New("convoy", log.LevelInfo)
	service := New(logger, db)

	createRequest := &datastore.CreatePortalLinkRequest{
		Name:              "Test Portal Link",
		OwnerID:           ulid.Make().String(),
		AuthType:          string(datastore.PortalAuthTypeRefreshToken),
		CanManageEndpoint: true,
		Endpoints:         []string{},
	}

	createdPortalLink, err := service.CreatePortalLink(ctx, project.UID, createRequest)
	require.NoError(t, err)

	portalLink, err := service.RefreshPortalLinkAuthToken(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)
	require.NotEmpty(t, portalLink.AuthKey)

	// AuthKey is "PRT.<maskID>.<secret>".
	parts := strings.Split(portalLink.AuthKey, ".")
	require.Len(t, parts, 3)
	maskID := parts[1]

	// Before revocation the mask id resolves to the link.
	found, err := service.FindPortalLinkByMaskId(ctx, maskID)
	require.NoError(t, err)
	require.Equal(t, createdPortalLink.UID, found.UID)

	// Revoke (soft-delete) the portal link.
	err = service.RevokePortalLink(ctx, project.UID, createdPortalLink.UID)
	require.NoError(t, err)

	// After revocation the mask id must no longer resolve.
	_, err = service.FindPortalLinkByMaskId(ctx, maskID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "portal link not found")
}
