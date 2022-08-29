package mongo

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/mongo"
)

type projectStatsRepo struct {
	db    *mongo.Database
	coll  *mongo.Collection
	store datastore.Store
}

func NewProjectStatsRepo(db *mongo.Database, store datastore.Store) datastore.ProjectStatsRepository {
	return &projectStatsRepo{
		db:    db,
		coll:  db.Collection(ProjectStatsCollection),
		store: store,
	}
}

func (db *projectStatsRepo) FetchGroupsStatistics(ctx context.Context, groups []*datastore.Group) error {
	stats := make([]datastore.ProjectStatistics, 0)

	err := db.store.FindAll(ctx, nil, nil, nil, &stats)
	if err != nil {
		return err
	}

	statsMap := map[string]*datastore.ProjectStatistics{}
	for i, s := range stats {
		statsMap[s.ID] = &stats[i]
	}

	for i := range groups {
		if sts, ok := statsMap[groups[i].UID]; ok {
			groups[i].Statistics = &datastore.GroupStatistics{
				GroupID:      sts.ID,
				MessagesSent: sts.MessagesSent,
				TotalApps:    sts.TotalApps,
			}
		}
	}

	return nil
}
