package mongo

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
	log "github.com/sirupsen/logrus"
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

func (o *orgMemberRepo) LoadUserOrganisationsPaged(ctx context.Context, userID string, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	matchStage1 := bson.D{
		{Key: "$match",
			Value: bson.D{
				{Key: "user_id", Value: userID},
				{Key: "document_status", Value: datastore.ActiveDocumentStatus},
			},
		},
	}

	lookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: OrganisationCollection},
			{Key: "localField", Value: "organisation_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "organisations"},
		}},
	}

	sortStage := bson.D{
		{Key: "$sort",
			Value: bson.D{
				{Key: "created_at", Value: pageable.Sort},
			},
		},
	}

	limitStage := bson.D{
		{Key: "$limit", Value: pageable.PerPage * pageable.Page},
	}

	projectStage := bson.D{
		{
			Key: "$project",
			Value: bson.D{
				{Key: "orgs",
					Value: bson.D{
						{Key: "$filter",
							Value: bson.D{
								{Key: "input", Value: "$organisations"},
								{Key: "as", Value: "organisations_field"},
								{Key: "cond",
									Value: bson.D{
										{Key: "$eq",
											Value: []interface{}{"$$organisations_field.document_status", datastore.ActiveDocumentStatus},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	data, err := o.inner.Aggregate(ctx, mongo.Pipeline{matchStage1, lookupStage, sortStage, limitStage, projectStage})
	if err != nil {
		log.WithError(err).Error("failed to run user organisations aggregation")
		return nil, datastore.PaginationData{}, err
	}
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	organisations := make([]datastore.Organisation, 0)

	err = data.All(ctx, &organisations)
	if err != nil {
		log.WithError(err).Error("failed to run user organisations aggregation")
		return nil, datastore.PaginationData{}, err
	}

	return organisations, datastore.PaginationData{}, nil
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

func (o *orgMemberRepo) DeleteOrganisationMember(ctx context.Context, uid, orgID string) error {
	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	filter := bson.M{
		"uid":             uid,
		"organisation_id": orgID,
	}

	_, err := o.inner.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (o *orgMemberRepo) FetchOrganisationMemberByID(ctx context.Context, uid, orgID string) (*datastore.OrganisationMember, error) {
	member := new(datastore.OrganisationMember)

	filter := bson.M{
		"uid":             uid,
		"organisation_id": orgID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	err := o.inner.FindOne(ctx, filter).Decode(&member)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgMemberNotFound
	}

	return member, err
}

func (o *orgMemberRepo) FetchOrganisationMemberByUserID(ctx context.Context, userID, orgID string) (*datastore.OrganisationMember, error) {
	filter := bson.M{
		"user_id":         userID,
		"organisation_id": orgID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	member := new(datastore.OrganisationMember)
	err := o.inner.FindOne(ctx, filter).Decode(member)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrOrgMemberNotFound
	}

	return member, err
}
