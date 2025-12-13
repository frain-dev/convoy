package portal_links

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/portal_links/models"
)

type PortalLinkRepository interface {
	CreatePortalLink(ctx context.Context, projectId string, request *models.CreatePortalLinkRequest) (*datastore.PortalLink, error)
	UpdatePortalLink(ctx context.Context, projectID string, portalLink *datastore.PortalLink, request *models.UpdatePortalLinkRequest) (*datastore.PortalLink, error)
	GetPortalLink(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error)
	GetPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error)
	GetPortalLinkByOwnerID(ctx context.Context, projectID, ownerID string) (*datastore.PortalLink, error)
	RefreshPortalLinkAuthToken(ctx context.Context, projectID, portalLinkID string) (*datastore.PortalLink, error)
	RevokePortalLink(ctx context.Context, projectID, portalLinkID string) error
	LoadPortalLinksPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error)
	FindPortalLinksByOwnerID(ctx context.Context, ownerID string) ([]datastore.PortalLink, error)
	FindPortalLinkByMaskId(ctx context.Context, maskId string) (*datastore.PortalLink, error)
}
