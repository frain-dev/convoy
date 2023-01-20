package mongo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func isDuplicateNameIndex(err error) bool {
	return strings.Contains(err.Error(), "name")
}

type projectRepo struct {
	store datastore.Store
}

func NewProjectRepo(store datastore.Store) datastore.ProjectRepository {
	return &projectRepo{
		store: store,
	}
}

func (db *projectRepo) CreateProject(ctx context.Context, o *datastore.Project) error {
	ctx = db.setCollectionInContext(ctx)

	// o.ID = primitive.NewObjectID()

	err := db.store.Save(ctx, o, nil)

	// check if the error string contains the index called "name"
	if mongo.IsDuplicateKeyError(err) && isDuplicateNameIndex(err) {
		return datastore.ErrDuplicateProjectName
	}

	return err
}

func (db *projectRepo) LoadProjects(ctx context.Context, f *datastore.ProjectFilter) ([]*datastore.Project, error) {
	ctx = db.setCollectionInContext(ctx)
	projects := make([]*datastore.Project, 0)

	filter := bson.M{}

	if !util.IsStringEmpty(f.OrgID) {
		filter["organisation_id"] = f.OrgID
	}

	sort := bson.M{"created_at": 1}
	err := db.store.FindAll(ctx, filter, sort, nil, &projects)

	return projects, err
}

func (db *projectRepo) UpdateProject(ctx context.Context, o *datastore.Project) error {
	ctx = db.setCollectionInContext(ctx)

	o.UpdatedAt = time.Now()
	update := bson.D{
		primitive.E{Key: "name", Value: o.Name},
		primitive.E{Key: "logo_url", Value: o.LogoURL},
		primitive.E{Key: "updated_at", Value: o.UpdatedAt},
		primitive.E{Key: "config", Value: o.Config},
		primitive.E{Key: "retained_events", Value: o.RetainedEvents},
	}

	err := db.store.UpdateByID(ctx, o.UID, bson.M{"$set": update})
	if mongo.IsDuplicateKeyError(err) && isDuplicateNameIndex(err) {
		return datastore.ErrDuplicateProjectName
	}

	return err
}

func (db *projectRepo) FetchProjectByID(ctx context.Context, id int) (*datastore.Project, error) {
	ctx = db.setCollectionInContext(ctx)

	project := new(datastore.Project)

	err := db.store.FindByID(ctx, fmt.Sprint(id), nil, project)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrProjectNotFound
	}

	return project, err
}

func (db *projectRepo) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	ctx = db.setCollectionInContext(ctx)

	matchStage := bson.D{
		{
			Key: "$match",
			Value: bson.D{
				{Key: "uid", Value: project.UID},
			},
		},
	}

	lookupStage1 := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.EndpointCollection},
			{Key: "localField", Value: "uid"},
			{Key: "foreignField", Value: "project_id"},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{
					{
						Key: "$match", Value: bson.D{
							{Key: "deleted_at", Value: nil},
						},
					},
				},
			}},
			{Key: "as", Value: "project_endpoints"},
		}},
	}

	lookupStage2 := bson.D{
		{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: datastore.EventCollection},
			{Key: "localField", Value: "uid"},
			{Key: "foreignField", Value: "project_id"},
			{Key: "pipeline", Value: mongo.Pipeline{
				bson.D{
					{
						Key: "$project", Value: bson.D{
							{Key: "uid", Value: 1},
						},
					},
				},
			}},
			{Key: "as", Value: "project_events"},
		}},
	}

	projectStage := bson.D{
		{
			Key: "$project",
			Value: bson.D{
				{Key: "project_id", Value: "$uid"},
				{Key: "total_endpoints", Value: bson.D{{Key: "$size", Value: "$project_endpoints"}}},
				{Key: "messages_sent", Value: bson.D{{Key: "$size", Value: "$project_events"}}},
			},
		},
	}
	var stats []*datastore.ProjectStatistics

	err := db.store.Aggregate(ctx, mongo.Pipeline{matchStage, lookupStage1, lookupStage2, projectStage}, &stats, false)
	if err != nil {
		log.WithError(err).Error("failed to run project statistics aggregation")
		return err
	}

	project.Statistics = stats[0]

	return nil
}

func (db *projectRepo) DeleteProject(ctx context.Context, uid string) error {
	ctx = db.setCollectionInContext(ctx)
	updateAsDeleted := bson.M{
		"$set": bson.M{
			"deleted_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	err := db.store.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		err := db.store.DeleteByID(sessCtx, uid, false)
		if err != nil {
			return err
		}

		var endpoints []datastore.Endpoint

		ctx := context.WithValue(sessCtx, datastore.CollectionCtx, datastore.EndpointCollection)
		filter := bson.M{"project_id": uid}
		err = db.store.FindAll(ctx, filter, nil, nil, &endpoints)
		if err != nil {
			return err
		}

		for _, endpoint := range endpoints {
			err = db.deleteEndpointEvents(sessCtx, endpoint.UID, updateAsDeleted)
			if err != nil {
				return err
			}

			err = db.deleteEndpointSubscriptions(sessCtx, endpoint.UID, updateAsDeleted)
			if err != nil {
				return err
			}

			err = db.deleteEndpoint(sessCtx, endpoint.UID, updateAsDeleted)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (db *projectRepo) FetchProjectsByIDs(ctx context.Context, ids []string) ([]datastore.Project, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"uid": bson.M{
			"$in": ids,
		},
	}

	projects := make([]datastore.Project, 0)
	sort := bson.M{"created_at": 1}
	err := db.store.FindAll(ctx, filter, sort, nil, &projects)
	if err != nil {
		return nil, err
	}

	return projects, err
}

func (db *projectRepo) deleteEndpointEvents(ctx context.Context, endpoint_id string, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.EventCollection)

	filter := bson.M{"endpoint_id": endpoint_id}
	return db.store.UpdateMany(ctx, filter, update, true)
}

func (db *projectRepo) deleteEndpoint(ctx context.Context, endpoint_id string, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.EndpointCollection)

	filter := bson.M{"uid": endpoint_id}
	return db.store.UpdateMany(ctx, filter, update, true)
}

func (db *projectRepo) deleteEndpointSubscriptions(ctx context.Context, endpoint_id string, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.SubscriptionCollection)

	filter := bson.M{"endpoint_id": endpoint_id}
	err := db.store.UpdateMany(ctx, filter, update, true)

	return err
}

func (db *projectRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.ProjectsCollection)
}
