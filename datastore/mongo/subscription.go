package mongo

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type subscriptionRepo struct {
	store datastore.Store
}

func NewSubscriptionRepo(store datastore.Store) datastore.SubscriptionRepository {
	return &subscriptionRepo{
		store: store,
	}
}

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.ID = primitive.NewObjectID()
	return s.store.Save(ctx, subscription, nil)
}

func (s *subscriptionRepo) UpdateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.M{
		"uid":        subscription.UID,
		"group_id":   groupId,
		"deleted_at": 0,
	}

	update := bson.M{
		"$set": bson.M{
			"name":        subscription.Name,
			"source_id":   subscription.SourceID,
			"endpoint_id": subscription.EndpointID,

			"filter_config.event_types": subscription.FilterConfig.EventTypes,
			"filter_config.filter":      subscription.FilterConfig.Filter,
			"alert_config":              subscription.AlertConfig,
			"retry_config":              subscription.RetryConfig,
			"disable_endpoint":          subscription.DisableEndpoint,
			"rate_limit_config":         subscription.RateLimitConfig,
		},
	}

	err := s.store.UpdateOne(ctx, filter, update)
	return err
}

func (s *subscriptionRepo) LoadSubscriptionsPaged(ctx context.Context, groupId string, f *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	ctx = s.setCollectionInContext(ctx)
	var subscriptions []datastore.Subscription

	filter := bson.M{"group_id": groupId, "deleted_at": 0}
	if !util.IsStringEmpty(f.AppID) {
		filter["app_id"] = f.AppID
	}

	matchStage := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{Key: "group_id", Value: groupId},
				{Key: "deleted_at", Value: 0},
			},
		},
	}

	if !util.IsStringEmpty(f.AppID) {
		matchStage = bson.D{
			{
				Key: "$match",
				Value: bson.D{
					{Key: "group_id", Value: groupId},
					{Key: "app_id", Value: f.AppID},
					{Key: "deleted_at", Value: 0},
				},
			},
		}
	}

	appStage := bson.D{
		{
			Key: "$lookup",
			Value: bson.D{
				{Key: "from", Value: "applications"},
				{Key: "localField", Value: "app_id"},
				{Key: "foreignField", Value: "uid"},
				{
					Key: "pipeline",
					Value: bson.A{
						bson.D{
							{
								Key: "$project",
								Value: bson.D{
									{Key: "uid", Value: 1},
									{Key: "title", Value: 1},
									{Key: "group_id", Value: 1},
									{Key: "support_email", Value: 1},
									{Key: "endpoints", Value: 1},
								},
							},
						},
					},
				},
				{Key: "as", Value: "app_metadata"},
			},
		},
	}
	unwindAppStage := bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$app_metadata"}, {Key: "preserveNullAndEmptyArrays", Value: true}}}}

	sourceStage := bson.D{
		{
			Key: "$lookup",
			Value: bson.D{
				{Key: "from", Value: "sources"},
				{Key: "localField", Value: "source_id"},
				{Key: "foreignField", Value: "uid"},
				{
					Key: "pipeline",
					Value: bson.A{
						bson.D{
							{
								Key: "$project",
								Value: bson.D{
									{Key: "uid", Value: 1},
									{Key: "name", Value: 1},
									{Key: "type", Value: 1},
									{Key: "mask_id", Value: 1},
									{Key: "group_id", Value: 1},
									{Key: "verifier", Value: 1},
									{Key: "is_disabled", Value: 1},
								},
							},
						},
					},
				},
				{Key: "as", Value: "source_metadata"},
			},
		},
	}
	unwindSourceStage := bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$source_metadata"}, {Key: "preserveNullAndEmptyArrays", Value: true}}}}

	endpointStage := bson.D{
		{
			Key: "$project",
			Value: bson.D{
				{Key: "_id", Value: 1},
				{Key: "uid", Value: 1},
				{Key: "name", Value: 1},
				{Key: "type", Value: 1},
				{Key: "status", Value: 1},
				{Key: "app_id", Value: 1},
				{Key: "group_id", Value: 1},
				{Key: "source_id", Value: 1},
				{Key: "endpoint_id", Value: 1},
				{Key: "alert_config", Value: 1},
				{Key: "retry_config", Value: 1},
				{Key: "filter_config", Value: 1},
				{Key: "created_at", Value: 1},
				{Key: "updated_at", Value: 1},
				{Key: "deleted_at", Value: 1},
				{Key: "app_metadata", Value: 1},
				{Key: "source_metadata", Value: 1},
				{
					Key: "endpoint_metadata",
					Value: bson.D{
						{
							Key: "$filter",
							Value: bson.D{
								{Key: "input", Value: "$app_metadata.endpoints"},
								{Key: "as", Value: "endpoint"},
								{
									Key: "cond",
									Value: bson.D{
										{
											Key: "$eq",
											Value: bson.A{
												"$$endpoint.uid",
												"$endpoint_id",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	unwindEndpointStage := bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$endpoint_metadata"}, {Key: "preserveNullAndEmptyArrays", Value: true}}}}
	skipStage := bson.D{{Key: "$skip", Value: getSkip(pageable.Page, pageable.PerPage)}}
	sortStage := bson.D{{Key: "$sort", Value: bson.D{{Key: "created_at", Value: -1}}}}
	limitStage := bson.D{{Key: "$limit", Value: pageable.PerPage}}

	// pipeline definition
	pipeline := mongo.Pipeline{
		matchStage,
		skipStage,
		sortStage,
		limitStage,
		appStage,
		sourceStage,
		endpointStage,
		unwindAppStage,
		unwindSourceStage,
		unwindEndpointStage,
	}

	err := s.store.Aggregate(ctx, pipeline, &subscriptions, true)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	count, err := s.store.Count(ctx, filter)
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

	return subscriptions, pagination, nil
}

func (s *subscriptionRepo) DeleteSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	filter := bson.M{
		"uid":      subscription.UID,
		"group_id": groupId,
	}
	return s.store.DeleteOne(ctx, filter, false)
}

func (s *subscriptionRepo) FindSubscriptionByID(ctx context.Context, groupId string, uid string) (*datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)
	subscription := &datastore.Subscription{}

	filter := bson.M{"uid": uid, "group_id": groupId, "deleted_at": 0}
	err := s.store.FindOne(ctx, filter, nil, subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrSubscriptionNotFound
	}

	return subscription, err
}

func (s *subscriptionRepo) FindSubscriptionsByEventType(ctx context.Context, groupId string, appId string, eventType datastore.EventType) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)

	filter := bson.M{"group_id": groupId, "app_id": appId, "filter_config.event_types": string(eventType), "deleted_at": 0}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionsByAppID(ctx context.Context, groupId string, appID string) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)

	filter := bson.M{
		"app_id":     appID,
		"group_id":   groupId,
		"deleted_at": 0,
	}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrSubscriptionNotFound
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionByDeviceID(ctx context.Context, groupId, deviceID string) (*datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)

	filter := bson.M{
		"device_id": deviceID,
		"group_id":  groupId,
	}

	subscription := &datastore.Subscription{}
	err := s.store.FindOne(ctx, filter, nil, &subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrSubscriptionNotFound
	}

	return subscription, nil
}

func (s *subscriptionRepo) FindSubscriptionsBySourceIDs(ctx context.Context, groupId string, sourceId string) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)
	filter := bson.M{"group_id": groupId, "source_id": sourceId, "deleted_at": 0}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) UpdateSubscriptionStatus(ctx context.Context, groupId string, subscriptionId string, status datastore.SubscriptionStatus) error {
	ctx = s.setCollectionInContext(ctx)

	filter := bson.M{
		"uid":        subscriptionId,
		"group_id":   groupId,
		"deleted_at": 0,
	}

	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	err := s.store.UpdateOne(ctx, filter, update)
	return err
}

func (s *subscriptionRepo) TestSubscriptionFilter(ctx context.Context, payload map[string]interface{}, filter map[string]interface{}) (bool, error) {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.FilterCollection)
	isValid := false

	err := s.store.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		f := datastore.SubscriptionFilter{
			ID:             primitive.NewObjectID(),
			UID:            uuid.NewString(),
			Filter:         payload,
			DocumentStatus: datastore.ActiveDocumentStatus,
		}

		// insert the desired request payload
		err := s.store.Save(sessCtx, f, nil)
		if err != nil {
			return err
		}

		// compare the filter with the test request payload
		var q map[string]interface{}
		if len(filter) == 0 {
			filter = nil
		}

		if filter != nil {
			q, err = flattenFilter(filter)
			if err != nil {
				return err
			}
		}

		var filters []datastore.SubscriptionFilter
		err = s.store.FindAll(sessCtx, q, nil, nil, &filters)
		if err != nil {
			return err
		}

		isValid = len(filters) > 0

		err = s.store.DeleteByID(sessCtx, f.UID, true)
		if err != nil {
			return err
		}

		return nil
	})

	return isValid, err
}

func (s *subscriptionRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.SubscriptionCollection)
}

func flattenFilter(f map[string]interface{}) (map[string]interface{}, error) {
	isAndOr := false
	var operator string

	for k := range f {
		if k == "$or" {
			if len(f) > 1 {
				return nil, flatten.ErrTopLevelElementOr
			}
			operator = k
			isAndOr = true
			break
		}

		if k == "$and" {
			if len(f) > 1 {
				return nil, flatten.ErrTopLevelElementAnd
			}
			isAndOr = true
			break
		}
	}

	if isAndOr {
		if a, ok := f[operator].([]interface{}); ok {
			if !ok {
				return nil, flatten.ErrOrAndMustBeArray
			}

			for i := range a {
				t, err := flatten.FlattenWithPrefix("filter", a[i].(map[string]interface{}))
				if err != nil {
					return nil, err
				}

				a[i] = t
			}

			f[operator] = a
			return f, nil
		}
	}

	query := map[string]interface{}{"filter": f}
	q, err := flatten.Flatten(query)
	if err != nil {
		return nil, err
	}

	return q, nil
}

// getSkip returns calculated skip value for the query
func getSkip(page, limit int) int {
	skip := (page - 1) * limit

	if skip <= 0 {
		skip = 0
	}

	return skip
}

// getPrevPage returns calculated value for the prev page
func getPrevPage(page int) int {
	if page == 0 {
		return 1
	}

	prev := 0
	if page-1 <= 0 {
		prev = page
	} else {
		prev = page - 1
	}

	return prev
}
