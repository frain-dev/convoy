package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type groupRepo struct {
	innerDB *mongo.Database
	inner   *mongo.Collection
}

func NewGroupRepo(db *mongo.Database) datastore.GroupRepository {
	return &groupRepo{
		innerDB: db,
		inner:   db.Collection(GroupCollection),
	}
}

func (db *groupRepo) LoadGroups(ctx context.Context, f *datastore.GroupFilter) ([]*datastore.Group, error) {
	groups := make([]*datastore.Group, 0)

	opts := &options.FindOptions{Collation: &options.Collation{Locale: "en", Strength: 2}}
	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}

	if len(f.Names) > 0 {
		filter["name"] = bson.M{"$in": f.Names}
	}

	cur, err := db.inner.Find(ctx, filter, opts)
	if err != nil {
		return groups, err
	}

	for cur.Next(ctx) {
		var group = new(datastore.Group)
		if err := cur.Decode(&group); err != nil {
			return groups, err
		}

		groups = append(groups, group)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	if err := cur.Close(ctx); err != nil {
		return groups, err
	}

	return groups, nil
}

func (db *groupRepo) CreateGroup(ctx context.Context, o *datastore.Group) error {

	o.ID = primitive.NewObjectID()

	_, err := db.inner.InsertOne(ctx, o)
	return err
}

func (db *groupRepo) UpdateGroup(ctx context.Context, o *datastore.Group) error {

	o.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.D{primitive.E{Key: "uid", Value: o.UID}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "name", Value: o.Name},
		primitive.E{Key: "logo_url", Value: o.LogoURL},
		primitive.E{Key: "updated_at", Value: o.UpdatedAt},
		primitive.E{Key: "config", Value: o.Config},
		primitive.E{Key: "rate_limit", Value: o.RateLimit},
		primitive.E{Key: "rate_limit_duration", Value: o.RateLimitDuration},
	}}}

	_, err := db.inner.UpdateOne(ctx, filter, update)
	return err
}

func (db *groupRepo) FetchGroupByID(ctx context.Context,
	id string) (*datastore.Group, error) {
	org := new(datastore.Group)

	filter := bson.D{
		primitive.E{
			Key:   "uid",
			Value: id,
		},
	}

	err := db.inner.FindOne(ctx, filter).
		Decode(&org)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrGroupNotFound
	}

	return org, err
}

func (db *groupRepo) DeleteGroup(ctx context.Context, uid string) error {
	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	_, err := db.inner.UpdateOne(ctx, bson.M{"uid": uid}, update)
	if err != nil {
		return err
	}

	return nil
}

func (db *groupRepo) FetchGroupsByIDs(ctx context.Context, ids []string) ([]datastore.Group, error) {
	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
		"document_status": datastore.ActiveDocumentStatus,
	}

	groups := make([]datastore.Group, 0)

	cur, err := db.inner.Find(ctx, filter, nil)
	if err != nil {
		return groups, err
	}

	for cur.Next(ctx) {
		var group datastore.Group
		if err := cur.Decode(&group); err != nil {
			return groups, err
		}

		groups = append(groups, group)
	}

	return groups, err
}
