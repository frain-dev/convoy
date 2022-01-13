package bolt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/frain-dev/convoy/datastore"

	"go.etcd.io/bbolt"
)

const name string = "groups"

type groupRepo struct {
	db *bbolt.DB
}

func NewGroupRepo(db *bbolt.DB) datastore.GroupRepository {
	return &groupRepo{db: db}
}

func (g *groupRepo) LoadGroups(context.Context, *datastore.GroupFilter) ([]*datastore.Group, error) {
	var groups []*datastore.Group
	err := g.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket([]byte(bucketName)).Cursor()

		prefix := []byte(name)
		var grpSlice [][]byte

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			fmt.Printf("key=%s, value=%s\n", k, v)
			grpSlice = append(grpSlice, v)
		}

		for i := 0; i < len(grpSlice); i++ {
			var grp *datastore.Group
			mErr := json.Unmarshal(grpSlice[i], &grp)
			if mErr != nil {
				return mErr
			}

			groups = append(groups, grp)
		}

		return nil
	})

	return groups, err
}

func (g *groupRepo) CreateGroup(ctx context.Context, group *datastore.Group) error {
	return createUpdateGroup(g.db, group)
}

func (g *groupRepo) UpdateGroup(_ context.Context, group *datastore.Group) error {
	return createUpdateGroup(g.db, group)
}

func (g *groupRepo) FetchGroupByID(ctx context.Context, gid string) (*datastore.Group, error) {
	var group *datastore.Group
	err := g.db.View(func(tx *bbolt.Tx) error {
		id := name + ":" + gid

		grp := tx.Bucket([]byte(bucketName)).Get([]byte(id))
		if grp == nil {
			return fmt.Errorf("group with id (%s) does not exist", gid)
		}

		var _grp *datastore.Group
		mErr := json.Unmarshal(grp, &_grp)
		if mErr != nil {
			return mErr
		}
		group = _grp

		return nil
	})

	return group, err
}

func (g *groupRepo) DeleteGroup(ctx context.Context, gid string) error {
	return g.db.Update(func(tx *bbolt.Tx) error {
		id := name + ":" + gid

		grp := tx.Bucket([]byte(bucketName)).Delete([]byte(id))
		if grp == nil {
			return fmt.Errorf("group with id (%s) does not exist", gid)
		}

		return nil
	})

}

func createUpdateGroup(db *bbolt.DB, group *datastore.Group) error {
	return db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))

		grp, err := json.Marshal(group)
		if err != nil {
			return err
		}

		id := name + ":" + group.UID
		pErr := b.Put([]byte(id), grp)
		if pErr != nil {
			return pErr
		}

		return nil
	})
}
