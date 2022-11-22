package mongo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func isDuplicateNameIndex(err error) bool {
	return strings.Contains(err.Error(), "name")
}

type groupRepo struct {
	store datastore.Store
}

func NewGroupRepo(store datastore.Store) datastore.GroupRepository {
	return &groupRepo{
		store: store,
	}
}

func (db *groupRepo) CreateGroup(ctx context.Context, o *datastore.Group) error {
	ctx = db.setCollectionInContext(ctx)

	o.ID = primitive.NewObjectID()

	err := db.store.Save(ctx, o, nil)

	// check if the error string contains the index called "name"
	if mongo.IsDuplicateKeyError(err) && isDuplicateNameIndex(err) {
		return datastore.ErrDuplicateGroupName
	}

	return err
}

func (db *groupRepo) LoadGroups(ctx context.Context, f *datastore.GroupFilter) ([]*datastore.Group, error) {
	ctx = db.setCollectionInContext(ctx)
	groups := make([]*datastore.Group, 0)

	filter := bson.M{}

	if !util.IsStringEmpty(f.OrgID) {
		filter["organisation_id"] = f.OrgID
	}

	f = f.WithNamesTrimmed()
	if len(f.Names) > 0 {
		filter["name"] = bson.M{"$in": f.Names}
	}

	sort := bson.M{"created_at": 1}
	err := db.store.FindAll(ctx, filter, sort, nil, &groups)

	return groups, err
}

func (db *groupRepo) UpdateGroup(ctx context.Context, o *datastore.Group) error {
	ctx = db.setCollectionInContext(ctx)

	o.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
	update := bson.D{
		primitive.E{Key: "name", Value: o.Name},
		primitive.E{Key: "logo_url", Value: o.LogoURL},
		primitive.E{Key: "updated_at", Value: o.UpdatedAt},
		primitive.E{Key: "config", Value: o.Config},
		primitive.E{Key: "rate_limit", Value: o.RateLimit},
		primitive.E{Key: "metadata", Value: o.Metadata},
		primitive.E{Key: "rate_limit_duration", Value: o.RateLimitDuration},
	}

	err := db.store.UpdateByID(ctx, o.UID, bson.M{"$set": update})
	if mongo.IsDuplicateKeyError(err) && isDuplicateNameIndex(err) {
		return datastore.ErrDuplicateGroupName
	}

	return err
}

func (db *groupRepo) FetchGroupByID(ctx context.Context, id string) (*datastore.Group, error) {
	ctx = db.setCollectionInContext(ctx)

	group := new(datastore.Group)

	err := db.store.FindByID(ctx, id, nil, group)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrGroupNotFound
	}

	return group, err
}

func (db *groupRepo) FillGroupsStatistics(ctx context.Context, groups []*datastore.Group) error {
	ctx = db.setCollectionInContext(ctx)

	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.UID)
	}

	matchStage := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{Key: "uid", Value: bson.M{"$in": ids}},
			},
		},
	}

	lookupStage1 := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.AppCollection},
			{Key: "localField", Value: "uid"},
			{Key: "foreignField", Value: "group_id"},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{
					{
						Key: "$match", Value: bson.D{
							{Key: "deleted_at", Value: nil},
						},
					},
				},
			}},
			{Key: "as", Value: "group_apps"},
		}},
	}

	lookupStage2 := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.EventCollection},
			{Key: "localField", Value: "uid"},
			{Key: "foreignField", Value: "group_id"},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{
					{
						Key: "$project", Value: bson.D{
							{Key: "_id", Value: "$uid"},
						},
					},
				},
			}},
			{Key: "as", Value: "group_events"},
		}},
	}

	projectStage := bson.D{
		{
			Key: "$project",
			Value: bson.D{
				{Key: "group_id", Value: "$uid"},
				{Key: "total_apps", Value: bson.D{{Key: "$size", Value: "$group_apps"}}},
				{Key: "messages_sent", Value: bson.D{{Key: "$size", Value: "$group_events"}}},
			},
		},
	}
	var stats []datastore.GroupStatistics

	err := db.store.Aggregate(ctx, mongo.Pipeline{matchStage, lookupStage1, lookupStage2, projectStage}, &stats, false)
	if err != nil {
		log.WithError(err).Error("failed to run group statistics aggregation")
		return err
	}

	statsMap := map[string]*datastore.GroupStatistics{}
	for i, s := range stats {
		statsMap[s.GroupID] = &stats[i]
	}

	for i := range groups {
		groups[i].Statistics = statsMap[groups[i].UID]
	}

	return nil
}

func (db *groupRepo) DeleteGroup(ctx context.Context, uid string) error {
	ctx = db.setCollectionInContext(ctx)
	updateAsDeleted := bson.M{
		"$set": bson.M{
			"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	err := db.store.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		err := db.store.DeleteByID(sessCtx, uid, false)
		if err != nil {
			return err
		}

		var apps []datastore.Application

		ctx := context.WithValue(sessCtx, datastore.CollectionCtx, datastore.AppCollection)
		filter := bson.M{"group_id": uid}
		err = db.store.FindAll(ctx, filter, nil, nil, &apps)
		if err != nil {
			return err
		}

		for _, app := range apps {
			err = db.deleteAppEvents(sessCtx, uid, updateAsDeleted)
			if err != nil {
				return err
			}

			err = db.deleteAppSubscriptions(sessCtx, uid, updateAsDeleted)
			if err != nil {
				return err
			}

			err = db.deleteApp(sessCtx, app.UID, updateAsDeleted)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (db *groupRepo) FetchGroupsByIDs(ctx context.Context, ids []string) ([]datastore.Group, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
	}

	groups := make([]datastore.Group, 0)
	sort := bson.M{"created_at": 1}
	err := db.store.FindAll(ctx, filter, sort, nil, &groups)
	if err != nil {
		return nil, err
	}

	return groups, err
}

func (db *groupRepo) deleteAppEvents(ctx context.Context, groupId string, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.EventCollection)

	filter := bson.M{"group_id": groupId}
	return db.store.UpdateMany(ctx, filter, update, true)
}

func (db *groupRepo) deleteApp(ctx context.Context, app_id string, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.AppCollection)

	filter := bson.M{"uid": app_id}
	return db.store.UpdateMany(ctx, filter, update, true)
}

func (db *groupRepo) deleteAppSubscriptions(ctx context.Context, app_id string, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.SubscriptionCollection)

	filter := bson.M{"app_id": app_id}
	err := db.store.UpdateMany(ctx, filter, update, true)

	return err
}

func (db *groupRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.GroupCollection)
}
