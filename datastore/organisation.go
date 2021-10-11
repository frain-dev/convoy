package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type orgRepo struct {
	inner *mongo.Collection
}

const (
	OrgCollection = "organisations"
)

func NewOrganisationRepo(client *mongo.Database) convoy.OrganisationRepository {
	return &orgRepo{
		inner: client.Collection(OrgCollection),
	}
}

func (db *orgRepo) LoadOrganisations(ctx context.Context, f *convoy.OrganisationFilter) ([]*convoy.Organisation, error) {
	orgs := make([]*convoy.Organisation, 0)

	query := make(bson.D, 0)
	var opts *options.FindOptions = nil

	if !util.IsStringEmpty(f.Name) {
		query = append(query, bson.E{Key: "org_name", Value: f.Name})
		opts = &options.FindOptions{Collation: &options.Collation{Locale: "en", Strength: 2}}
	}

	cur, err := db.inner.Find(ctx, query, opts)
	if err != nil {
		return orgs, err
	}

	for cur.Next(ctx) {
		var org = new(convoy.Organisation)
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

func (db *orgRepo) CreateOrganisation(ctx context.Context, o *convoy.Organisation) error {

	o.ID = primitive.NewObjectID()

	_, err := db.inner.InsertOne(ctx, o)
	return err
}

func (db *orgRepo) UpdateOrganisation(ctx context.Context, o *convoy.Organisation) error {

	o.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.D{primitive.E{Key: "uid", Value: o.UID}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "org_name", Value: o.OrgName},
		primitive.E{Key: "logo_url", Value: o.LogoURL},
		primitive.E{Key: "updated_at", Value: o.UpdatedAt},
	}}}

	_, err := db.inner.UpdateOne(ctx, filter, update)
	return err
}

func (db *orgRepo) FetchOrganisationByID(ctx context.Context,
	id string) (*convoy.Organisation, error) {
	org := new(convoy.Organisation)

	filter := bson.D{
		primitive.E{
			Key:   "uid",
			Value: id,
		},
	}

	err := db.inner.FindOne(ctx, filter).
		Decode(&org)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = convoy.ErrOrganisationNotFound
	}

	return org, err
}
