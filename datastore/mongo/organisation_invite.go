package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type orgInviteRepo struct {
	store datastore.Store
}

func NewOrgInviteRepo(store datastore.Store) datastore.OrganisationInviteRepository {
	return &orgInviteRepo{
		store: store,
	}
}

func (db *orgInviteRepo) CreateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	ctx = db.setCollectionInContext(ctx)
	iv.ID = primitive.NewObjectID()
	return db.store.Save(ctx, iv, nil)
}

func (db *orgInviteRepo) LoadOrganisationsInvitesPaged(ctx context.Context, orgID string, inviteStatus datastore.InviteStatus, pageable datastore.Pageable) ([]datastore.OrganisationInvite, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{}

	if !util.IsStringEmpty(orgID) {
		filter["organisation_id"] = orgID
	}

	if !util.IsStringEmpty(inviteStatus.String()) {
		filter["status"] = inviteStatus
	}

	var invitations []datastore.OrganisationInvite
	pagination, err := db.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &invitations)
	if err != nil {
		return invitations, datastore.PaginationData{}, err
	}

	return invitations, pagination, nil
}

func (db *orgInviteRepo) UpdateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	ctx = db.setCollectionInContext(ctx)

	iv.UpdatedAt = time.Now()
	update := bson.M{
		"$set": bson.M{
			"role":       iv.Role,
			"status":     iv.Status,
			"updated_at": iv.UpdatedAt,
			"expires_at": iv.ExpiresAt,
		},
	}

	return db.store.UpdateOne(ctx, bson.M{"uid": iv.UID}, update)
}

func (db *orgInviteRepo) DeleteOrganisationInvite(ctx context.Context, uid string) error {
	ctx = db.setCollectionInContext(ctx)

	update := bson.M{
		"$set": bson.M{
			"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return db.store.UpdateOne(ctx, bson.M{"uid": uid}, update)
}

func (db *orgInviteRepo) FetchOrganisationInviteByID(ctx context.Context, id string) (*datastore.OrganisationInvite, error) {
	ctx = db.setCollectionInContext(ctx)

	org := &datastore.OrganisationInvite{}

	err := db.store.FindByID(ctx, id, nil, org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgInviteNotFound
	}

	return org, err
}

func (db *orgInviteRepo) FetchOrganisationInviteByToken(ctx context.Context, token string) (*datastore.OrganisationInvite, error) {
	ctx = db.setCollectionInContext(ctx)

	org := &datastore.OrganisationInvite{}

	filter := bson.M{
		"token": token,
	}

	err := db.store.FindOne(ctx, filter, nil, org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgInviteNotFound
	}

	return org, err
}

func (db *orgInviteRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.OrganisationInvitesCollection)
}
