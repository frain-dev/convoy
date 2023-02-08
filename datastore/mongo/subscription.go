package mongo

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/flatten"
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

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
	if projectID != subscription.ProjectID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.ID = primitive.NewObjectID()
	return s.store.Save(ctx, subscription, nil)
}

func (s *subscriptionRepo) UpdateSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
	if projectID != subscription.ProjectID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.M{
		"uid":        subscription.UID,
		"project_id": projectID,
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
			"rate_limit_config":         subscription.RateLimitConfig,
		},
	}

	err := s.store.UpdateOne(ctx, filter, update)
	return err
}

func (s *subscriptionRepo) LoadSubscriptionsPaged(ctx context.Context, projectID string, f *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	ctx = s.setCollectionInContext(ctx)
	var subscriptions []datastore.Subscription

	matchStage := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{Key: "project_id", Value: projectID},
				{Key: "deleted_at", Value: nil},
			},
		},
	}

	if len(f.EndpointIDs) > 0 {
		matchStage = bson.D{
			{
				Key: "$match",
				Value: bson.D{
					{Key: "project_id", Value: projectID},
					{Key: "endpoint_id", Value: bson.M{
						"$in": f.EndpointIDs,
					}},
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
				{Key: "localField", Value: "endpoint_id"},
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
									{Key: "project_id", Value: 1},
									{Key: "support_email", Value: 1},
									{Key: "target_url", Value: 1},
									{Key: "secrets", Value: 1},
								},
							},
						},
					},
				},
				{Key: "as", Value: "endpoint_metadata"},
			},
		},
	}

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
									{Key: "project_id", Value: 1},
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
	unwindEndpointStage := bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$endpoint_metadata"}, {Key: "preserveNullAndEmptyArrays", Value: true}}}}
	skipStage := bson.D{{Key: "$skip", Value: getSkip(pageable.Page, pageable.PerPage)}}
	sortStage := bson.D{{Key: "$sort", Value: bson.D{{Key: "created_at", Value: -1}}}}
	limitStage := bson.D{{Key: "$limit", Value: pageable.PerPage}}

	// pipeline definition
	pipeline := mongo.Pipeline{
		matchStage,
		sortStage,
		skipStage,
		limitStage,
		sourceStage,
		endpointStage,
		unwindSourceStage,
		unwindEndpointStage,
	}

	err := s.store.Aggregate(ctx, pipeline, &subscriptions, true)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	filter := bson.M{"project_id": projectID}
	if len(f.EndpointIDs) > 0 {
		filter["endpoint_id"] = bson.M{"$in": f.EndpointIDs}
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

func (s *subscriptionRepo) DeleteSubscription(ctx context.Context, projectID string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
	if projectID != subscription.ProjectID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	filter := bson.M{
		"uid":        subscription.UID,
		"project_id": projectID,
	}
	return s.store.DeleteOne(ctx, filter, false)
}

func (s *subscriptionRepo) FindSubscriptionByID(ctx context.Context, projectID string, uid string) (*datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)
	subscription := &datastore.Subscription{}

	filter := bson.M{"uid": uid, "project_id": projectID}
	err := s.store.FindOne(ctx, filter, nil, subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrSubscriptionNotFound
	}

	return subscription, err
}

func (s *subscriptionRepo) FindSubscriptionsByEventType(ctx context.Context, projectID string, endpointID string, eventType datastore.EventType) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)

	filter := bson.M{"project_id": projectID, "endpoint_id": endpointID, "filter_config.event_types": string(eventType)}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionsByEndpointID(ctx context.Context, projectID string, endpointID string) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)

	filter := bson.M{
		"endpoint_id": endpointID,
		"project_id":  projectID,
	}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)

	return subscriptions, err
}

func (s *subscriptionRepo) FindSubscriptionByDeviceID(ctx context.Context, projectID, deviceID string) (*datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)

	filter := bson.M{
		"device_id":  deviceID,
		"project_id": projectID,
	}

	subscription := &datastore.Subscription{}
	err := s.store.FindOne(ctx, filter, nil, &subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrSubscriptionNotFound
	}

	return subscription, nil
}

func (s *subscriptionRepo) FindSubscriptionsBySourceIDs(ctx context.Context, projectID string, sourceId string) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)
	filter := bson.M{"project_id": projectID, "source_id": sourceId}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) TestSubscriptionFilter(ctx context.Context, payload map[string]interface{}, filter map[string]interface{}) (bool, error) {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.FilterCollection)
	isValid := false

	err := s.store.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		f := datastore.SubscriptionFilter{
			ID:        primitive.NewObjectID(),
			UID:       uuid.NewString(),
			Filter:    payload,
			DeletedAt: nil,
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
