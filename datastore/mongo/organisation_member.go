package mongo

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

type orgMemberRepo struct {
	innerDB *mongo.Database
	inner   *mongo.Collection
}

func NewOrgMemberRepo(db *mongo.Database) datastore.OrganisationMemberRepository {
	return &orgMemberRepo{
		innerDB: db,
		inner:   db.Collection(OrganisationMembersCollection),
	}
}

func (o *orgMemberRepo) LoadOrganisationMembersPaged(ctx context.Context, organisationID string, pageable datastore.Pageable) ([]datastore.OrganisationMember, datastore.PaginationData, error) {
	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}

	if !util.IsStringEmpty(organisationID) {
		filter["organisation_id"] = organisationID
	}

	organisations := make([]datastore.OrganisationMember, 0)
	paginatedData, err := pager.New(o.inner).
		Context(ctx).
		Limit(int64(pageable.PerPage)).
		Page(int64(pageable.Page)).
		Sort("created_at", pageable.Sort).
		Filter(filter).
		Decode(&organisations).
		Find()

	if err != nil {
		return organisations, datastore.PaginationData{}, err
	}

	return organisations, datastore.PaginationData(paginatedData.Pagination), nil
}

func (o *orgMemberRepo) CreateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	member.ID = primitive.NewObjectID()
	_, err := o.inner.InsertOne(ctx, member)
	return err
}

func (o *orgMemberRepo) UpdateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	member.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "role", Value: member.Role},
		primitive.E{Key: "updated_at", Value: member.UpdatedAt},
	}}}

	_, err := o.inner.UpdateOne(ctx, bson.M{"uid": member.UID}, update)
	return err
}

func (o *orgMemberRepo) DeleteOrganisationMember(ctx context.Context, uid string) error {
	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	_, err := o.inner.UpdateOne(ctx, bson.M{"uid": uid}, update)
	if err != nil {
		return err
	}

	return nil
}

func (o *orgMemberRepo) FetchOrganisationMemberByID(ctx context.Context, uid string) (*datastore.OrganisationMember, error) {
	member := new(datastore.OrganisationMember)

	filter := bson.M{
		"uid":             uid,
		"document_status": datastore.ActiveDocumentStatus,
	}

	err := o.inner.FindOne(ctx, filter).Decode(&member)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgMemberNotFound
	}

	return member, err
}

func (o *orgMemberRepo) FetchOrganisationMemberByUserID(ctx context.Context, userID string) (*datastore.OrganisationMember, error) {
	member := new(datastore.OrganisationMember)

	filter := bson.M{
		"user_id":         userID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	err := o.inner.FindOne(ctx, filter).Decode(&member)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgMemberNotFound
	}

	return member, err
}
