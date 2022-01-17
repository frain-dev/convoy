package bolt

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy/datastore"
	"go.etcd.io/bbolt"
)

type appRepo struct {
	db *bbolt.DB
}

func NewApplicationRepo(db *bbolt.DB) datastore.ApplicationRepository {
	return &appRepo{db: db}
}

func (a *appRepo) CreateApplication(ctx context.Context, app *datastore.Application) error {
	tx, err := a.db.Begin(true)
	if err != nil {
		return err
	}

	bucket := tx.Bucket([]byte(bucketName))

	groupJson, err := json.Marshal(app)
	if err != nil {
		return err
	}
	return bucket.Put([]byte(app.Title), groupJson)
}

func (a *appRepo) LoadApplicationsPaged(ctx context.Context, groupID string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	return make([]datastore.Application, 0), datastore.PaginationData{}, nil
}

func (a *appRepo) LoadApplicationsPagedByGroupId(ctx context.Context, groupID string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	return make([]datastore.Application, 0), datastore.PaginationData{}, nil
}

func (a *appRepo) SearchApplicationsByGroupId(ctx context.Context, groupId string, searchParams datastore.SearchParams) ([]datastore.Application, error) {
	return nil, nil
}

func (a *appRepo) FindApplicationByID(ctx context.Context, id string) (*datastore.Application, error) {
	return nil, nil
}

func (a *appRepo) FindApplicationEndpointByID(ctx context.Context, appID string, endpointID string) (*datastore.Endpoint, error) {
	return nil, nil
}

func (a *appRepo) UpdateApplication(ctx context.Context, app *datastore.Application) error {
	return nil
}

func (a *appRepo) DeleteApplication(ctx context.Context, app *datastore.Application) error {
	return nil
}

func (a *appRepo) UpdateApplicationEndpointsStatus(ctx context.Context, appId string, endpointIds []string, status datastore.EndpointStatus) error {
	return nil
}

func (a *appRepo) DeleteGroupApps(context.Context, string) error {
	return nil
}

func (a *appRepo) CountGroupApplications(ctx context.Context, groupID string) (int64, error) {
	return 0, nil
}
