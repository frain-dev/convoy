package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type orgMemberRepo struct {
	innerDB *mongo.Database
	inner   *mongo.Collection
	store   datastore.Store
}

func NewOrgMemberRepo(db *mongo.Database, store datastore.Store) datastore.OrganisationMemberRepository {
	return &orgMemberRepo{
		innerDB: db,
		inner:   db.Collection(OrganisationMembersCollection),
		store:   store,
	}
}

func (o *orgMemberRepo) LoadOrganisationMembersPaged(ctx context.Context, organisationID string, pageable datastore.Pageable) ([]*datastore.OrganisationMember, datastore.PaginationData, error) {
	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}

	if !util.IsStringEmpty(organisationID) {
		filter["organisation_id"] = organisationID
	}

	members := make([]*datastore.OrganisationMember, 0)
	paginatedData, err := pager.New(o.inner).
		Context(ctx).
		Limit(int64(pageable.PerPage)).
		Page(int64(pageable.Page)).
		Sort("created_at", pageable.Sort).
		Filter(filter).
		Decode(&members).
		Find()

	if err != nil {
		return members, datastore.PaginationData{}, err
	}

	err = o.fillOrgMemberUserMetadata(ctx, members)
	if err != nil {
		return members, datastore.PaginationData{}, err
	}

	return members, datastore.PaginationData(paginatedData.Pagination), nil
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

	sortStage := bson.D{
		{Key: "$sort",
			Value: bson.D{
				{Key: "created_at", Value: pageable.Sort},
			},
		},
	}

	skip := 0
	if pageable.Page > 1 {
		skip = pageable.PerPage * pageable.Page
	}
	skipStage := bson.D{
		{Key: "$skip", Value: skip},
	}

	limitStage := bson.D{
		{Key: "$limit", Value: pageable.PerPage},
	}

	lookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: OrganisationCollection},
			{Key: "localField", Value: "organisation_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "organisations"},
		}},
	}

	unwindStage := bson.D{
		{Key: "$unwind", Value: "$organisations"},
	}

	replaceRootStage := bson.D{
		{Key: "$replaceRoot",
			Value: bson.D{
				{Key: "newRoot", Value: "$organisations"},
			},
		},
	}

	matchStage2 := bson.D{
		{Key: "$match",
			Value: bson.D{
				{Key: "document_status", Value: datastore.ActiveDocumentStatus},
			},
		},
	}
	organisations := make([]datastore.Organisation, 0)

	err := o.store.Aggregate(ctx, mongo.Pipeline{matchStage1, sortStage, skipStage, limitStage, lookupStage, unwindStage, replaceRootStage, matchStage2}, &organisations, false)
	if err != nil {
		log.WithError(err).Error("failed to run user organisations aggregation")
		return nil, datastore.PaginationData{}, err
	}

	return organisations, datastore.PaginationData{}, nil
}

func (o *orgMemberRepo) CreateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	member.ID = primitive.NewObjectID()
	err := o.store.Save(ctx, member, nil)
	return err
}

func (o *orgMemberRepo) UpdateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	member.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	update := bson.D{
		primitive.E{Key: "role", Value: member.Role},
		primitive.E{Key: "updated_at", Value: member.UpdatedAt},
	}

	err := o.store.UpdateOne(ctx, bson.M{"uid": member.UID}, update)
	return err
}

func (o *orgMemberRepo) DeleteOrganisationMember(ctx context.Context, uid, orgID string) error {
	update := bson.M{
		"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
		"document_status": datastore.DeletedDocumentStatus,
	}

	filter := bson.M{
		"uid":             uid,
		"organisation_id": orgID,
	}

	err := o.store.UpdateOne(ctx, filter, update)
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

	err := o.store.FindOne(ctx, filter, nil, member)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrOrgMemberNotFound
	}

	err = o.fillOrgMemberUserMetadata(ctx, []*datastore.OrganisationMember{member})
	return member, err
}

func (o *orgMemberRepo) FetchOrganisationMemberByUserID(ctx context.Context, userID, orgID string) (*datastore.OrganisationMember, error) {
	filter := bson.M{
		"user_id":         userID,
		"organisation_id": orgID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	member := new(datastore.OrganisationMember)
	err := o.store.FindOne(ctx, filter, nil, member)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrOrgMemberNotFound
	}

	err = o.fillOrgMemberUserMetadata(ctx, []*datastore.OrganisationMember{member})
	return member, err
}

func (o *orgMemberRepo) fillOrgMemberUserMetadata(ctx context.Context, members []*datastore.OrganisationMember) error {
	userIDs := make([]string, 0, len(members))
	orgIDs := make([]string, 0, len(members))
	for i := range members {
		userIDs = append(userIDs, members[i].UserID)
		orgIDs = append(orgIDs, members[i].OrganisationID)
	}

	matchStage := bson.D{
		{Key: "$match",
			Value: bson.D{
				{Key: "$and",
					Value: []bson.D{
						{{Key: "user_id", Value: bson.M{"$in": userIDs}}},
						{{Key: "organisation_id", Value: bson.M{"$in": orgIDs}}},
					},
				},
			},
		},
	}

	lookupStage := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: UserCollection},
			{Key: "localField", Value: "user_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "user"},
		}},
	}

	projectStage1 := bson.D{
		{
			Key: "$project",
			Value: bson.D{
				{Key: "user_info", Value: bson.M{"$arrayElemAt": []interface{}{"$user", 0}}},
			},
		},
	}

	replaceRootStage := bson.D{
		{Key: "$replaceRoot",
			Value: bson.D{
				{Key: "newRoot", Value: "$user_info"},
			},
		},
	}
	projectStage2 := bson.D{
		{
			Key: "$project",
			Value: bson.D{
				{Key: "user_id", Value: "$uid"},
				{Key: "first_name", Value: "$first_name"},
				{Key: "last_name", Value: "$last_name"},
				{Key: "email", Value: "$email"},
			}},
	}
	var userMetadata []datastore.UserMetadata

	err := o.store.Aggregate(ctx, mongo.Pipeline{matchStage, lookupStage, projectStage1, replaceRootStage, projectStage2}, &userMetadata, false)
	if err != nil {
		log.WithError(err).Error("failed to run user metadata for organisation members aggregation")
		return err
	}

	metaMap := map[string]*datastore.UserMetadata{}
	for i, s := range userMetadata {
		metaMap[s.UserID] = &userMetadata[i]
	}

	for i := range members {
		members[i].UserMetadata = metaMap[members[i].UserID]
	}

	return nil
}
