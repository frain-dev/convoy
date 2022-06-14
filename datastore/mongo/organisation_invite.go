package mongo

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/util"
	"time"

	"github.com/frain-dev/convoy/datastore"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type orgInviteRepo struct {
	innerDB *mongo.Database
	inner   *mongo.Collection
}

func NewOrgInviteRepo(db *mongo.Database) datastore.OrganisationInviteRepository {
	return &orgInviteRepo{
		innerDB: db,
		inner:   db.Collection(OrganisationInvitesCollection),
	}
}

func (db *orgInviteRepo) LoadOrganisationsInvitesPaged(ctx context.Context, orgID string, inviteStatus datastore.InviteStatus, pageable datastore.Pageable) ([]datastore.OrganisationInvite, datastore.PaginationData, error) {
	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}

	if !util.IsStringEmpty(orgID) {
		filter["organisation_id"] = orgID
	}

	if !util.IsStringEmpty(inviteStatus.String()) {
		filter["status"] = inviteStatus
	}

	organisations := make([]datastore.OrganisationInvite, 0)
	paginatedData, err := pager.New(db.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", pageable.Sort).Filter(filter).Decode(&organisations).Find()
	if err != nil {
		return organisations, datastore.PaginationData{}, err
	}

	return organisations, datastore.PaginationData(paginatedData.Pagination), nil
}

func (db *orgInviteRepo) CreateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	iv.ID = primitive.NewObjectID()
	_, err := db.inner.InsertOne(ctx, iv)
	return err
}

func (db *orgInviteRepo) UpdateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	iv.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())
	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "role", Value: iv.Role},
		primitive.E{Key: "status", Value: iv.Status},
		primitive.E{Key: "updated_at", Value: iv.UpdatedAt},
	}}}

	_, err := db.inner.UpdateOne(ctx, bson.M{"uid": iv.UID}, update)
	return err
}

func (db *orgInviteRepo) DeleteOrganisationInvite(ctx context.Context, uid string) error {
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

func (db *orgInviteRepo) FetchOrganisationInviteByID(ctx context.Context, id string) (*datastore.OrganisationInvite, error) {
	org := &datastore.OrganisationInvite{}

	filter := bson.M{
		"uid":             id,
		"document_status": datastore.ActiveDocumentStatus,
	}

	err := db.inner.FindOne(ctx, filter).Decode(org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgInviteNotFound
	}

	return org, err
}

func (db *orgInviteRepo) FetchOrganisationInviteByToken(ctx context.Context, token string) (*datastore.OrganisationInvite, error) {
	org := &datastore.OrganisationInvite{}

	filter := bson.M{
		"token":           token,
		"document_status": datastore.ActiveDocumentStatus,
	}

	err := db.inner.FindOne(ctx, filter).Decode(org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgInviteNotFound
	}

	return org, err
}
