package mongo

import (
	"context"
	"errors"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type sourceRepo struct {
	cache cache.Cache
	store datastore.Store
}

func NewSourceRepo(store datastore.Store, cache cache.Cache) datastore.SourceRepository {
	return &sourceRepo{
		store: store,
		cache: cache,
	}
}

func (s *sourceRepo) CreateSource(ctx context.Context, source *datastore.Source) error {
	ctx = s.setCollectionInContext(ctx)
	source.ID = primitive.NewObjectID()

	err := s.store.Save(ctx, source, nil)
	if err != nil {
		return err
	}

	sourceCacheKey := convoy.SourceCacheKey.Get(source.MaskID).String()
	err = s.cache.Set(ctx, sourceCacheKey, &source, time.Hour*24)
	if err != nil {
		log.WithError(err).Error("failed to add source to cache")
	}

	return nil
}

func (s *sourceRepo) UpdateSource(ctx context.Context, groupId string, source *datastore.Source) error {
	ctx = s.setCollectionInContext(ctx)
	filter := bson.M{"uid": source.UID, "group_id": groupId, "document_status": datastore.ActiveDocumentStatus}

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
	if err != nil {
		return err
	}

	sourceCacheKey := convoy.SourceCacheKey.Get(source.MaskID).String()
	err = s.cache.Set(ctx, sourceCacheKey, &source, time.Hour*24)
	if err != nil {
		log.WithError(err).Error("failed to add source to cache")
	}

	return nil
}

func (s *sourceRepo) FindSourceByID(ctx context.Context, groupId string, id string) (*datastore.Source, error) {
	ctx = s.setCollectionInContext(ctx)
	source := &datastore.Source{}

	filter := bson.M{"uid": id, "group_id": groupId}

	err := s.store.FindOne(ctx, filter, nil, source)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return source, datastore.ErrSourceNotFound
	}

	return source, err
}

func (s *sourceRepo) FindSourceByMaskID(ctx context.Context, maskId string) (*datastore.Source, error) {
	sourceCacheKey := convoy.SourceCacheKey.Get(maskId).String()
	var source *datastore.Source
	err := s.cache.Get(ctx, sourceCacheKey, source)
	if err != nil {
		log.WithError(err).Error("failed to get source from cache")
	}

	if source != nil {
		return source, nil
	}

	ctx = s.setCollectionInContext(ctx)
	source = &datastore.Source{}

	filter := bson.M{"mask_id": maskId}
	err = s.store.FindOne(ctx, filter, nil, source)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return source, datastore.ErrSourceNotFound
	}

	return source, nil
}

func (s *sourceRepo) DeleteSourceByID(ctx context.Context, groupId string, id string) error {
	ctx = s.setCollectionInContext(ctx)
	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	err := s.store.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		srcfilter := bson.M{"uid": id, "group_id": groupId}
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

func (s *sourceRepo) LoadSourcesPaged(ctx context.Context, groupID string, f *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	ctx = s.setCollectionInContext(ctx)
	var sources []datastore.Source

	filter := bson.M{"document_status": datastore.ActiveDocumentStatus, "group_id": groupID, "type": f.Type, "provider": f.Provider}

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

func (s *sourceRepo) setCollectionInContext(ctx context.Context) context.Context {
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
