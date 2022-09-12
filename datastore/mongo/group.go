package mongo

import (
	"context"
	"errors"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

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
	var filter primitive.M
	if f.OrgID == "" {
		filter = bson.M{
			"document_status": datastore.ActiveDocumentStatus,
		}
	} else {
		filter = bson.M{
			"document_status": datastore.ActiveDocumentStatus,
			"organisation_id": f.OrgID,
		}
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
	update := bson.D{primitive.E{Key: "name", Value: o.Name},
		primitive.E{Key: "logo_url", Value: o.LogoURL},
		primitive.E{Key: "updated_at", Value: o.UpdatedAt},
		primitive.E{Key: "config", Value: o.Config},
		primitive.E{Key: "rate_limit", Value: o.RateLimit},
		primitive.E{Key: "metadata", Value: o.Metadata},
		primitive.E{Key: "rate_limit_duration", Value: o.RateLimitDuration},
	}

	err := db.store.UpdateByID(ctx, o.UID, update)
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
		{Key: "$match",
			Value: bson.D{
				{Key: "uid", Value: bson.M{"$in": ids}},
			},
		},
	}

	lookupStage1 := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: AppCollection},
			{Key: "localField", Value: "uid"},
			{Key: "foreignField", Value: "group_id"},
			{Key: "as", Value: "group_apps"},
		}},
	}

	lookupStage2 := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: EventCollection},
			{Key: "localField", Value: "uid"},
			{Key: "foreignField", Value: "group_id"},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{{
					Key: "$project", Value: bson.D{
						{Key: "_id", Value: "$uid"},
					}},
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
			}},
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

	err := db.store.DeleteByID(ctx, uid, false)
	if err != nil {
		return err
	}

	return nil
}

func (db *groupRepo) FetchGroupsByIDs(ctx context.Context, ids []string) ([]datastore.Group, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
		"document_status": datastore.ActiveDocumentStatus,
	}

	groups := make([]datastore.Group, 0)
	sort := bson.M{"created_at": 1}
	err := db.store.FindAll(ctx, filter, sort, nil, &groups)
	if err != nil {
		return nil, err
	}

	return groups, err
}

func (db *groupRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, GroupCollection)
}
