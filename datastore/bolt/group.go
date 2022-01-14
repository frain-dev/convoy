package bolt

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy"
	"go.etcd.io/bbolt"
)

type groupRepo struct {
	db         *bbolt.DB
	bucketName string
}

func NewGroupRepo(db *bbolt.DB) convoy.GroupRepository {
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

func (g *groupRepo) LoadGroups(ctx context.Context, filter *convoy.GroupFilter) ([]*convoy.Group, error) {
	var groups []*convoy.Group
	err := g.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(g.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var grp *convoy.Group
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

func (g *groupRepo) CreateGroup(ctx context.Context, group *convoy.Group) error {
	return g.createUpdateGroup(group)
}

func (g *groupRepo) UpdateGroup(_ context.Context, group *convoy.Group) error {
	return g.createUpdateGroup(group)
}

func (g *groupRepo) FetchGroupByID(ctx context.Context, gid string) (*convoy.Group, error) {
	var group *convoy.Group
	err := g.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(g.bucketName))

		grp := b.Get([]byte(gid))
		if grp == nil {
			return convoy.ErrGroupNotFound
		}

		var temp *convoy.Group
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

func (g *groupRepo) createUpdateGroup(group *convoy.Group) error {
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
