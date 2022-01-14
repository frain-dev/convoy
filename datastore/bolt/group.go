package bolt

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy/datastore"
	"go.etcd.io/bbolt"
)

type groupRepo struct {
	db         *bbolt.DB
	bucketName string
}

func NewGroupRepo(db *bbolt.DB) datastore.GroupRepository {
	bucketName := "groups"
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	if err != nil {
		return nil
	}

	return &groupRepo{db: db, bucketName: bucketName}
}

func (g *groupRepo) LoadGroups(ctx context.Context, filter *datastore.GroupFilter) ([]*datastore.Group, error) {
	var groups []*datastore.Group
	err := g.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(g.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var grp *datastore.Group
			err := json.Unmarshal(v, &grp)
			if err != nil {
				return err
			}

			if len(filter.Names) > 0 {
				for _, grpName := range filter.Names {
					if grpName == grp.Name {
						groups = append(groups, grp)
					}
				}
			} else {
				groups = append(groups, grp)
			}

			return nil
		})
	})

	return groups, err
}

func (g *groupRepo) CreateGroup(ctx context.Context, group *datastore.Group) error {
	return g.createUpdateGroup(group)
}

func (g *groupRepo) UpdateGroup(_ context.Context, group *datastore.Group) error {
	return g.createUpdateGroup(group)
}

func (g *groupRepo) FetchGroupByID(ctx context.Context, gid string) (*datastore.Group, error) {
	var group *datastore.Group
	err := g.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(g.bucketName))

		grp := b.Get([]byte(gid))
		if grp == nil {
			return datastore.ErrGroupNotFound
		}

		var temp *datastore.Group
		err := json.Unmarshal(grp, &temp)
		if err != nil {
			return err
		}
		group = temp

		return nil
	})

	return group, err
}

func (g *groupRepo) DeleteGroup(ctx context.Context, gid string) error {
	return g.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(g.bucketName))
		return b.Delete([]byte(gid))
	})
}

func (g *groupRepo) createUpdateGroup(group *datastore.Group) error {
	return g.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(g.bucketName))

		grp, err := json.Marshal(group)
		if err != nil {
			return err
		}

		pErr := b.Put([]byte(group.UID), grp)
		if pErr != nil {
			return pErr
		}

		return nil
	})
}
