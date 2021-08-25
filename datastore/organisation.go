package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/hookcamp/hookcamp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type orgRepo struct {
	inner *mongo.Collection
}

const (
	OrgCollection = "organisations"
)

func NewOrganisationRepo(client *mongo.Database) hookcamp.OrganisationRepository {
	return &orgRepo{
		inner: client.Collection(OrgCollection),
	}
}

func (db *orgRepo) LoadOrganisations(ctx context.Context) ([]*hookcamp.Organisation, error) {
	orgs := make([]*hookcamp.Organisation, 0)

	cur, err := db.inner.Find(ctx, bson.D{{}})
	if err != nil {
		return orgs, err
	}

	for cur.Next(ctx) {
		var org = new(hookcamp.Organisation)
		if err := cur.Decode(&org); err != nil {
			return orgs, err
		}

		orgs = append(orgs, org)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	if err := cur.Close(ctx); err != nil {
		return orgs, err
	}

	return orgs, nil
}

func (db *orgRepo) CreateOrganisation(ctx context.Context, o *hookcamp.Organisation) error {

	o.ID = primitive.NewObjectID()

	_, err := db.inner.InsertOne(ctx, o)
	return err
}

func (db *orgRepo) UpdateOrganisation(ctx context.Context, o *hookcamp.Organisation) error {

	o.UpdatedAt = time.Now().Unix()

	filter := bson.D{primitive.E{Key: "uid", Value: o.UID}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "org_name", Value: o.OrgName},
		primitive.E{Key: "updated_at", Value: o.UpdatedAt},
	}}}

	_, err := db.inner.UpdateOne(ctx, filter, update)
	return err
}

func (db *orgRepo) FetchOrganisationByID(ctx context.Context,
	id string) (*hookcamp.Organisation, error) {
	org := new(hookcamp.Organisation)

	filter := bson.D{
		primitive.E{
			Key:   "uid",
			Value: id,
		},
	}

	err := db.inner.FindOne(ctx, filter).
		Decode(&org)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = hookcamp.ErrOrganisationNotFound
	}

	return org, err
}
