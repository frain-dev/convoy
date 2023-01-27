package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type orgMemberRepo struct {
	store datastore.Store
}

func NewOrgMemberRepo(store datastore.Store) datastore.OrganisationMemberRepository {
	return &orgMemberRepo{
		store: store,
	}
}

func (o *orgMemberRepo) CreateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	ctx = o.setCollectionInContext(ctx)
	// member.ID = primitive.NewObjectID()
	return o.store.Save(ctx, member, nil)
}

func (o *orgMemberRepo) LoadOrganisationMembersPaged(ctx context.Context, organisationID string, pageable datastore.Pageable) ([]*datastore.OrganisationMember, datastore.PaginationData, error) {
	ctx = o.setCollectionInContext(ctx)

	filter := bson.M{"deleted_at": nil}

	if !util.IsStringEmpty(organisationID) {
		filter["organisation_id"] = organisationID
	}

	var members []*datastore.OrganisationMember

	pagination, err := o.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &members)
	if err != nil {
		return members, datastore.PaginationData{}, err
	}

	err = o.fillOrgMemberUserMetadata(ctx, members)
	if err != nil {
		return members, datastore.PaginationData{}, err
	}

	return members, pagination, nil
}

func (o *orgMemberRepo) LoadUserOrganisationsPaged(ctx context.Context, userID string, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	ctx = o.setCollectionInContext(ctx)

	matchStage1 := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{Key: "user_id", Value: userID},
				{Key: "deleted_at", Value: nil},
			},
		},
	}

	sortStage := bson.D{
		{
			Key: "$sort",
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
			{Key: "from", Value: datastore.OrganisationCollection},
			{Key: "localField", Value: "organisation_id"},
			{Key: "foreignField", Value: "uid"},
			{Key: "as", Value: "organisations"},
		}},
	}

	unwindStage := bson.D{
		{Key: "$unwind", Value: "$organisations"},
	}

	replaceRootStage := bson.D{
		{
			Key: "$replaceRoot",
			Value: bson.D{
				{Key: "newRoot", Value: "$organisations"},
			},
		},
	}

	matchStage2 := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{Key: "deleted_at", Value: nil},
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

func (o *orgMemberRepo) UpdateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	ctx = o.setCollectionInContext(ctx)
	member.UpdatedAt = time.Now()

	update := bson.M{
		"$set": bson.M{
			"role":       member.Role,
			"updated_at": member.UpdatedAt,
		},
	}

	return o.store.UpdateOne(ctx, bson.M{"uid": member.UID}, update)
}

func (o *orgMemberRepo) DeleteOrganisationMember(ctx context.Context, uid, orgID string) error {
	ctx = o.setCollectionInContext(ctx)
	update := bson.M{
		"$set": bson.M{
			"deleted_at": time.Now(),
		},
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
	ctx = o.setCollectionInContext(ctx)
	member := new(datastore.OrganisationMember)

	filter := bson.M{
		"uid":             uid,
		"organisation_id": orgID,
	}

	err := o.store.FindOne(ctx, filter, nil, member)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrOrgMemberNotFound
	}

	err = o.fillOrgMemberUserMetadata(ctx, []*datastore.OrganisationMember{member})
	return member, err
}

func (o *orgMemberRepo) FetchOrganisationMemberByUserID(ctx context.Context, userID, orgID string) (*datastore.OrganisationMember, error) {
	ctx = o.setCollectionInContext(ctx)
	filter := bson.M{
		"user_id":         userID,
		"organisation_id": orgID,
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
	ctx = o.setCollectionInContext(ctx)
	userIDs := make([]string, 0, len(members))
	orgIDs := make([]string, 0, len(members))
	for i := range members {
		userIDs = append(userIDs, members[i].UserID)
		orgIDs = append(orgIDs, members[i].OrganisationID)
	}

	matchStage := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{
					Key: "$and",
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
			{Key: "from", Value: datastore.UserCollection},
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
		{
			Key: "$replaceRoot",
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
			},
		},
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
		members[i].UserMetadata = *metaMap[members[i].UserID]
	}

	return nil
}

func (o *orgMemberRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.OrganisationMembersCollection)
}
