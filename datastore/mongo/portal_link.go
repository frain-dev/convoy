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

func (p *portalLinkRepo) UpdatePortalLink(ctx context.Context, projectID string, portal *datastore.PortalLink) error {
	ctx = p.setCollectionInContext(ctx)
	filter := bson.M{"uid": portal.UID, "project_id": portal.ProjectID}

	update := bson.M{
		"$set": bson.M{
			"name":       portal.Name,
			"endpoints":  portal.Endpoints,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return p.store.UpdateOne(ctx, filter, update)
}

func (p *portalLinkRepo) FindPortalLinkByID(ctx context.Context, projectID string, id string) (*datastore.PortalLink, error) {
	ctx = p.setCollectionInContext(ctx)
	portalLink := &datastore.PortalLink{}

	filter := bson.M{"uid": id, "project_id": projectID}

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

func (p *portalLinkRepo) LoadPortalLinksPaged(ctx context.Context, projectID string, f *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	ctx = p.setCollectionInContext(ctx)
	filter := bson.M{"project_id": projectID, "deleted_at": nil}

	matchStage := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{Key: "project_id", Value: projectID},
				{Key: "deleted_at", Value: nil},
			},
		},
	}

	if !util.IsStringEmpty(f.EndpointID) {
		filter["endpoints"] = f.EndpointID

		matchStage = bson.D{
			{
				Key: "$match",
				Value: bson.D{
					{Key: "project_id", Value: projectID},
					{Key: "endpoints", Value: f.EndpointID},
					{Key: "deleted_at", Value: nil},
				},
			},
		}
	}

	endpointStage := bson.D{
		{
			Key: "$lookup",
			Value: bson.D{
				{Key: "from", Value: "endpoints"},
				{Key: "localField", Value: "endpoints"},
				{Key: "foreignField", Value: "uid"},
				{Key: "as", Value: "endpoints_metadata"},
			},
		},
	}

	skipStage := bson.D{{Key: "$skip", Value: getSkip(pageable.Page, pageable.PerPage)}}
	sortStage := bson.D{{Key: "$sort", Value: bson.D{{Key: "created_at", Value: -1}}}}
	limitStage := bson.D{{Key: "$limit", Value: pageable.PerPage}}

	pipeline := mongo.Pipeline{
		matchStage,
		sortStage,
		limitStage,
		skipStage,
		endpointStage,
	}

	portalLinks := make([]datastore.PortalLink, 0)
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

func (p *portalLinkRepo) DeletePortalLink(ctx context.Context, projectID string, id string) error {
	ctx = p.setCollectionInContext(ctx)

	filter := bson.M{"uid": id, "project_id": projectID}
	update := bson.M{
		"$set": bson.M{
			"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return p.store.UpdateOne(ctx, filter, update)
}

func (p *portalLinkRepo) RevokePortalLink(ctx context.Context, projectID string, id string) error {
	ctx = p.setCollectionInContext(ctx)

	filter := bson.M{"uid": id, "project_id": projectID}
	update := bson.M{
		"$set": bson.M{
			"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return p.store.UpdateOne(ctx, filter, update)
}

func (p *portalLinkRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.PortalLinkCollection)
}
