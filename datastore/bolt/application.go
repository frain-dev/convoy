package bolt

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"go.etcd.io/bbolt"
)

type appRepo struct {
	db         *bbolt.DB
	bucketName string
}

func NewApplicationRepo(db *bbolt.DB) convoy.ApplicationRepository {
	bucketName := "applications"
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	if err != nil {
		return nil
	}
	return &appRepo{db: db, bucketName: bucketName}
}

func (a *appRepo) CreateApplication(ctx context.Context, app *convoy.Application) error {
	return a.createUpdateApplication(app)
}

func (a *appRepo) LoadApplicationsPaged(ctx context.Context, groupID string, pageable models.Pageable) ([]convoy.Application, models.PaginationData, error) {
	return make([]convoy.Application, 0), models.PaginationData{}, nil
}

func (a *appRepo) LoadApplicationsPagedByGroupId(ctx context.Context, groupID string, pageable models.Pageable) ([]convoy.Application, models.PaginationData, error) {
	return make([]convoy.Application, 0), models.PaginationData{}, nil
}

func (a *appRepo) SearchApplicationsByGroupId(ctx context.Context, gid string, searchParams models.SearchParams) ([]convoy.Application, error) {
	return nil, nil
}

func (a *appRepo) FindApplicationByID(ctx context.Context, aid string) (*convoy.Application, error) {
	var group *convoy.Application
	err := a.db.View(func(tx *bbolt.Tx) error {
		grp := tx.Bucket([]byte(a.bucketName)).Get([]byte(aid))
		if grp == nil {
			return fmt.Errorf("application with id (%s) does not exist", aid)
		}

		var _grp *convoy.Application
		mErr := json.Unmarshal(grp, &_grp)
		if mErr != nil {
			return mErr
		}
		group = _grp

		return nil
	})

	return group, err
}

func (a *appRepo) FindApplicationEndpointByID(ctx context.Context, appID string, endpointID string) (*convoy.Endpoint, error) {
	return nil, nil
}

func (a *appRepo) UpdateApplication(ctx context.Context, app *convoy.Application) error {
	return a.createUpdateApplication(app)
}

func (a *appRepo) DeleteApplication(ctx context.Context, app *convoy.Application) error {
	return a.db.Update(func(tx *bbolt.Tx) error {
		grp := tx.Bucket([]byte(a.bucketName)).Delete([]byte(app.UID))
		if grp == nil {
			return fmt.Errorf("application with id (%s) does not exist", app.UID)
		}

		return nil
	})
}

func (a *appRepo) UpdateApplicationEndpointsStatus(ctx context.Context, appId string, endpointIds []string, status convoy.EndpointStatus) error {
	return nil
}

func (a *appRepo) DeleteGroupApps(context.Context, string) error {
	return nil
}

func (a *appRepo) CountGroupApplications(tx context.Context, groupID string) (int64, error) {
	return 0, nil
}

func (a *appRepo) createUpdateApplication(app *convoy.Application) error {
	return a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		aJson, err := json.Marshal(app)
		if err != nil {
			return err
		}

		id := a.bucketName + ":" + app.UID
		pErr := b.Put([]byte(id), aJson)
		if pErr != nil {
			return pErr
		}

		return nil
	})
}
