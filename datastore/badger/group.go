package badger

import (
	"context"
	"errors"
	_ "fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
)

type groupRepo struct {
	db *badgerhold.Store
}

func NewGroupRepo(db *badgerhold.Store) datastore.GroupRepository {
	return &groupRepo{db: db}
}

func (g *groupRepo) LoadGroups(ctx context.Context, filter *datastore.GroupFilter) ([]*datastore.Group, error) {
	var groups []*datastore.Group

	err := g.db.Find(&groups, badgerhold.Where("Name").In(badgerhold.Slice(filter.Names)...).Or(&badgerhold.Query{}))

	return groups, err
}

func (g *groupRepo) CreateGroup(ctx context.Context, group *datastore.Group) error {
	return g.db.Upsert(group.UID, group)
}

func (g *groupRepo) UpdateGroup(_ context.Context, group *datastore.Group) error {
	return g.db.Update(group.UID, group)
}

func (g *groupRepo) FetchGroupByID(ctx context.Context, gid string) (*datastore.Group, error) {
	var group *datastore.Group

	err := g.db.Get(gid, &group)

	if err != nil && errors.Is(err, badgerhold.ErrNotFound) {
		return group, datastore.ErrGroupNotFound
	}

	return group, err
}

func (g *groupRepo) FetchGroupsByIDs(ctx context.Context, ids []string) ([]datastore.Group, error) {
	var groups []datastore.Group

	err := g.db.Find(&groups, badgerhold.Where("UID").In(badgerhold.Slice(ids)...))

	return groups, err
}

func (g *groupRepo) DeleteGroup(ctx context.Context, gid string) error {
	return g.db.DeleteMatching(&datastore.Group{}, badgerhold.Where("UID").Eq(gid))
}
