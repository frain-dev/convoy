package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type groupRepo struct {
	inner *mongo.Collection
}

const (
	GroupCollection = "groups"
)

func NewGroupRepo(client *mongo.Database) convoy.GroupRepository {
	return &groupRepo{
		inner: client.Collection(GroupCollection),
	}
}

func (db *groupRepo) LoadGroups(ctx context.Context, f *convoy.GroupFilter) ([]*convoy.Group, error) {
	groups := make([]*convoy.Group, 0)

	opts := &options.FindOptions{Collation: &options.Collation{Locale: "en", Strength: 2}}
	filter := bson.M{
		"document_status": bson.M{"$ne": convoy.DeletedDocumentStatus},
	}

	if len(f.Names) > 0 {
		filter["name"] = bson.M{"$in": f.Names}
	}

	cur, err := db.inner.Find(ctx, filter, opts)
	if err != nil {
		return groups, err
	}

	for cur.Next(ctx) {
		var group = new(convoy.Group)
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

func (db *groupRepo) CreateGroup(ctx context.Context, o *convoy.Group) error {

	o.ID = primitive.NewObjectID()

	_, err := db.inner.InsertOne(ctx, o)
	return err
}

func (db *groupRepo) UpdateGroup(ctx context.Context, o *convoy.Group) error {

	o.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.D{primitive.E{Key: "uid", Value: o.UID}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "name", Value: o.Name},
		primitive.E{Key: "logo_url", Value: o.LogoURL},
		primitive.E{Key: "updated_at", Value: o.UpdatedAt},
		primitive.E{Key: "config", Value: o.Config},
	}}}

	_, err := db.inner.UpdateOne(ctx, filter, update)
	return err
}

func (db *groupRepo) FetchGroupByID(ctx context.Context,
	id string) (*convoy.Group, error) {
	org := new(convoy.Group)

	filter := bson.D{
		primitive.E{
			Key:   "uid",
			Value: id,
		},
	}

	err := db.inner.FindOne(ctx, filter).
		Decode(&org)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = convoy.ErrGroupNotFound
	}

	return org, err
}

func (db *groupRepo) FetchGroupsByIDs(ctx context.Context, ids []string) ([]convoy.Group, error) {
	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
		"document_status": bson.M{
			"$ne": convoy.DeletedDocumentStatus,
		},
	}

	groups := make([]convoy.Group, 0)

	cur, err := db.inner.Find(ctx, filter, nil)
	if err != nil {
		return groups, err
	}

	for cur.Next(ctx) {
		var group convoy.Group
		if err := cur.Decode(&group); err != nil {
			return groups, err
		}

		groups = append(groups, group)
	}

	return groups, err
}
