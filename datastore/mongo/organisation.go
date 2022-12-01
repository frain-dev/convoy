package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type orgRepo struct {
	store datastore.Store
}

func NewOrgRepo(store datastore.Store) datastore.OrganisationRepository {
	return &orgRepo{
		store: store,
	}
}

func (db *orgRepo) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	ctx = db.setCollectionInContext(ctx)
	org.ID = primitive.NewObjectID()
	return db.store.Save(ctx, org, nil)
}

func (db *orgRepo) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)
	var organisations []datastore.Organisation

	pagination, err := db.store.FindMany(ctx, bson.M{}, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &organisations)
	if err != nil {
		return organisations, datastore.PaginationData{}, err
	}

	return organisations, pagination, nil
}

func (db *orgRepo) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	ctx = db.setCollectionInContext(ctx)
	org.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
	update := bson.M{
		"$set": bson.M{
			"name":       org.Name,
			"updated_at": org.UpdatedAt,
		},
	}

	err := db.store.UpdateOne(ctx, bson.M{"uid": org.UID}, update)
	return err
}

func (db *orgRepo) DeleteOrganisation(ctx context.Context, uid string) error {
	ctx = db.setCollectionInContext(ctx)
	update := bson.M{
		"$set": bson.M{
			"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	err := db.store.UpdateOne(ctx, bson.M{"uid": uid}, update)
	if err != nil {
		return err
	}

	return nil
}

func (db *orgRepo) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	ctx = db.setCollectionInContext(ctx)
	org := new(datastore.Organisation)

	err := db.store.FindByID(ctx, id, nil, org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgNotFound
	}

	return org, err
}

func (db *orgRepo) FetchOrganisationByCustomDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	ctx = db.setCollectionInContext(ctx)
	org := &datastore.Organisation{}

	filter := bson.M{"custom_domain": domain}

	err := db.store.FindOne(ctx, filter, nil, org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return org, datastore.ErrOrgNotFound
	}

	return org, nil
}

func (db *orgRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.OrganisationCollection)
}
