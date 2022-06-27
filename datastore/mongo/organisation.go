package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type orgRepo struct {
	innerDB *mongo.Database
	inner   *mongo.Collection
	store   datastore.Store
}

func NewOrgRepo(db *mongo.Database, store datastore.Store) datastore.OrganisationRepository {
	return &orgRepo{
		innerDB: db,
		inner:   db.Collection(OrganisationCollection),
		store:   store,
	}
}

func (db *orgRepo) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}

	organisations := make([]datastore.Organisation, 0)
	paginatedData, err := pager.New(db.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", pageable.Sort).Filter(filter).Decode(&organisations).Find()
	if err != nil {
		return organisations, datastore.PaginationData{}, err
	}

	return organisations, datastore.PaginationData(paginatedData.Pagination), nil
}

func (db *orgRepo) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	org.ID = primitive.NewObjectID()
	err := db.store.Save(ctx, org, nil)
	return err
}

func (db *orgRepo) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	org.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
	update := bson.D{
		primitive.E{Key: "name", Value: org.Name},
		primitive.E{Key: "updated_at", Value: org.UpdatedAt},
	}

	err := db.store.UpdateOne(ctx, bson.M{"uid": org.UID}, update)
	return err
}

func (db *orgRepo) DeleteOrganisation(ctx context.Context, uid string) error {
	update := bson.M{
		"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
		"document_status": datastore.DeletedDocumentStatus,
	}

	err := db.store.UpdateOne(ctx, bson.M{"uid": uid}, update)
	if err != nil {
		return err
	}

	return nil
}

func (db *orgRepo) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	org := new(datastore.Organisation)

	err := db.store.FindByID(ctx, id, nil, org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgNotFound
	}

	return org, err
}
