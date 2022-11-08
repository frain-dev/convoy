package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type portalLinkRepo struct {
	store datastore.Store
}

func NewPortalLinkRepo(store datastore.Store) datastore.PortalLinkRepository {
	return &portalLinkRepo{
		store: store,
	}
}

func (p *portalLinkRepo) CreatePortalLink(ctx context.Context, portal *datastore.PortalLink) error {
	ctx = p.setCollectionInContext(ctx)
	portal.ID = primitive.NewObjectID()

	err := p.store.Save(ctx, portal, nil)
	return err
}

func (p *portalLinkRepo) UpdatePortalLink(ctx context.Context, groupID string, portal *datastore.PortalLink) error {
	ctx = p.setCollectionInContext(ctx)
	filter := bson.M{"uid": portal.UID, "group_id": portal.GroupID, "document_status": datastore.ActiveDocumentStatus}

	update := bson.M{
		"$set": bson.M{
			"endpoints":  portal.Endpoints,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return p.store.UpdateOne(ctx, filter, update)
}

func (p *portalLinkRepo) FindPortalLinkByID(ctx context.Context, groupID string, id string) (*datastore.PortalLink, error) {
	ctx = p.setCollectionInContext(ctx)
	portalLink := &datastore.PortalLink{}

	filter := bson.M{"uid": id, "group_id": groupID}

	err := p.store.FindOne(ctx, filter, nil, portalLink)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return portalLink, datastore.ErrPortalLinkNotFound
	}

	return portalLink, err
}

func (p *portalLinkRepo) LoadPortalLinksPaged(ctx context.Context, groupID string, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	ctx = p.setCollectionInContext(ctx)
	portalLinks := make([]datastore.PortalLink, 0)

	filter := bson.M{"group_id": groupID}
	pagination, err := p.store.FindMany(ctx, filter, nil, nil, int64(pageable.Page), int64(pageable.PerPage), &portalLinks)

	if err != nil {
		return portalLinks, datastore.PaginationData{}, err
	}

	return portalLinks, pagination, nil
}

func (p *portalLinkRepo) DeletePortalLink(ctx context.Context, groupID string, id string) error {
	ctx = p.setCollectionInContext(ctx)

	filter := bson.M{"uid": id, "group_id": groupID}
	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	return p.store.UpdateOne(ctx, filter, update)
}

func (p *portalLinkRepo) RevokePortalLink(ctx context.Context, groupID string, id string) error {
	ctx = p.setCollectionInContext(ctx)

	filter := bson.M{"uid": id, "group_id": groupID}
	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	return p.store.UpdateOne(ctx, filter, update)
}

func (p *portalLinkRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.PortalLinkCollection)
}
