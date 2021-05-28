package datastore

import (
	"context"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type orgRepo struct {
	inner *mongo.Collection
}

const (
	orgCollection = "organisations"
)

func NewOrganisationRepo(client *mongo.Database) hookcamp.OrganisationRepository {
	return &orgRepo{
		inner: client.Collection(orgCollection),
	}
}

func (db *orgRepo) LoadOrganisations(ctx context.Context) ([]hookcamp.Organisation, error) {
	orgs := make([]hookcamp.Organisation, 0)

	cur, err := db.inner.Find(ctx, nil)
	if err != nil {
		return orgs, err
	}

	for cur.Next(ctx) {
		var org hookcamp.Organisation
		if err := cur.Decode(&org); err != nil {
			return orgs, err
		}
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
	if o.UID == uuid.Nil {
		o.UID = uuid.New()
	}

	_, err := db.inner.InsertOne(ctx, o)
	return err
}

func (db *orgRepo) FetchOrganisationByID(ctx context.Context, id uuid.UUID) (*hookcamp.Organisation, error) {
	org := new(hookcamp.Organisation)

	err := db.inner.FindOne(ctx, bson.M{"uid": id.String()}).
		Decode(&org)

	return org, err
}
