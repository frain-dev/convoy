package mongo

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
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
			"name":       portal.Name,
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

func (p *portalLinkRepo) FindPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error) {
	ctx = p.setCollectionInContext(ctx)
	portalLink := &datastore.PortalLink{}

	filter := bson.M{"token": token}

	err := p.store.FindOne(ctx, filter, nil, portalLink)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return portalLink, datastore.ErrPortalLinkNotFound
	}

	return portalLink, err
}

func (p *portalLinkRepo) LoadPortalLinksPaged(ctx context.Context, groupID string, f *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	ctx = p.setCollectionInContext(ctx)
	portalLinks := make([]datastore.PortalLink, 0)

	filter := bson.M{"group_id": groupID}

	matchStage := bson.D{
		{Key: "$match",
			Value: bson.D{
				{Key: "group_id", Value: groupID},
				{Key: "document_status", Value: datastore.ActiveDocumentStatus},
			},
		},
	}

	if !util.IsStringEmpty(f.EndpointID) {
		filter["endpoints"] = f.EndpointID

		matchStage = bson.D{
			{Key: "$match",
				Value: bson.D{
					{Key: "group_id", Value: groupID},
					{Key: "endpoints", Value: f.EndpointID},
					{Key: "document_status", Value: datastore.ActiveDocumentStatus},
				},
			},
		}
	}

	endpointStage := bson.D{
		{Key: "$lookup",
			Value: bson.D{
				{Key: "from", Value: "endpoints"},
				{Key: "localField", Value: "endpoints"},
				{Key: "foreignField", Value: "uid"},
				{Key: "as", Value: "endpoints_metadata"},
			},
		},
	}

	sortAndLimitStages := []bson.D{
		{{Key: "$sort", Value: bson.D{{Key: "created_at", Value: pageable.Sort}}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
		{{Key: "$skip", Value: getSkip(pageable.Page, pageable.PerPage)}},
		{{Key: "$limit", Value: pageable.PerPage}},
	}

	pipeline := mongo.Pipeline{
		matchStage,
		endpointStage,
	}

	pipeline = append(pipeline, sortAndLimitStages...)
	err := p.store.Aggregate(ctx, pipeline, &portalLinks, true)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	count, err := p.store.Count(ctx, filter)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := datastore.PaginationData{
		Total:     count,
		Page:      int64(pageable.Page),
		PerPage:   int64(pageable.PerPage),
		Prev:      int64(getPrevPage(pageable.Page)),
		Next:      int64(pageable.Page + 1),
		TotalPage: int64(math.Ceil(float64(count) / float64(pageable.PerPage))),
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
