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
}

func NewOrgRepo(db *mongo.Database) datastore.OrganisationRepository {
	return &orgRepo{
		innerDB: db,
		inner:   db.Collection(OrganisationCollection),
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
	_, err := db.inner.InsertOne(ctx, org)
	return err
}

func (db *orgRepo) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	org.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "name", Value: org.Name},
		primitive.E{Key: "updated_at", Value: org.UpdatedAt},
	}}}

	_, err := db.inner.UpdateOne(ctx, bson.M{"uid": org.UID}, update)
	return err
}

func (db *orgRepo) DeleteOrganisation(ctx context.Context, uid string) error {
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

func (db *orgRepo) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	org := new(datastore.Organisation)

	filter := bson.M{"uid": id, "document_status": datastore.ActiveDocumentStatus}

	err := db.inner.FindOne(ctx, filter).Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgNotFound
	}

	return org, err
}
