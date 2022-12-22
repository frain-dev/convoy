package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type sourceRepo struct {
	store datastore.Store
}

func NewSourceRepo(store datastore.Store) datastore.SourceRepository {
	return &sourceRepo{
		store: store,
	}
}

func (s *sourceRepo) CreateSource(ctx context.Context, source *datastore.Source) error {
	ctx = s.setCollectionInContext(ctx)
	source.ID = primitive.NewObjectID()

	err := s.store.Save(ctx, source, nil)
	return err
}

func (s *sourceRepo) UpdateSource(ctx context.Context, projectId string, source *datastore.Source) error {
	ctx = s.setCollectionInContext(ctx)
	filter := bson.M{"uid": source.UID, "project_id": projectId}

	update := bson.M{
		"$set": bson.M{
			"name":            source.Name,
			"type":            source.Type,
			"is_disabled":     source.IsDisabled,
			"verifier":        source.Verifier,
			"updated_at":      primitive.NewDateTimeFromTime(time.Now()),
			"provider_config": source.ProviderConfig,
		},
	}

	err := s.store.UpdateOne(ctx, filter, update)
	return err
}

func (s *sourceRepo) FindSourceByID(ctx context.Context, projectId string, id string) (*datastore.Source, error) {
	ctx = s.setCollectionInContext(ctx)
	source := &datastore.Source{}

	filter := bson.M{"uid": id, "project_id": projectId}

	err := s.store.FindOne(ctx, filter, nil, source)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return source, datastore.ErrSourceNotFound
	}

	return source, err
}

func (s *sourceRepo) FindSourceByMaskID(ctx context.Context, maskId string) (*datastore.Source, error) {
	ctx = s.setCollectionInContext(ctx)
	source := &datastore.Source{}

	filter := bson.M{"mask_id": maskId}

	err := s.store.FindOne(ctx, filter, nil, source)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return source, datastore.ErrSourceNotFound
	}

	return source, nil
}

func (s *sourceRepo) DeleteSourceByID(ctx context.Context, projectId string, id string) error {
	ctx = s.setCollectionInContext(ctx)
	update := bson.M{
		"$set": bson.M{
			"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	err := s.store.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		srcfilter := bson.M{"uid": id, "project_id": projectId}
		err := s.store.UpdateOne(sessCtx, srcfilter, update)
		if err != nil {
			return err
		}

		err = s.deleteSubscription(sessCtx, id, update)
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

func (s *sourceRepo) deleteSubscription(ctx context.Context, sourceId string, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.SubscriptionCollection)

	filter := bson.M{"source_id": sourceId}
	err := s.store.UpdateMany(ctx, filter, update, true)

	return err
}

func (s *sourceRepo) LoadSourcesPaged(ctx context.Context, projectID string, f *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	ctx = s.setCollectionInContext(ctx)
	var sources []datastore.Source

	filter := bson.M{"project_id": projectID, "type": f.Type, "provider": f.Provider}

	removeUnusedFields(filter)
	pagination, err := s.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &sources)
	if err != nil {
		return sources, datastore.PaginationData{}, err
	}

	if sources == nil {
		sources = make([]datastore.Source, 0)
	}

	return sources, pagination, nil
}

func (db *sourceRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.SourceCollection)
}

func removeUnusedFields(filter map[string]interface{}) {
	for k, v := range filter {
		item, ok := v.(string)
		if !ok {
			continue
		}

		if util.IsStringEmpty(item) {
			delete(filter, k)
		}

	}
}
