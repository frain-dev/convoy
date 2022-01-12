package bolt

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"go.etcd.io/bbolt"
)

type appRepo struct {
	db *bbolt.DB
}

func NewApplicationRepo(db *bbolt.DB) convoy.ApplicationRepository {
	return &appRepo{db: db}
}

func (a *appRepo) CreateApplication(ctx context.Context, app *convoy.Application) error {
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

func (a *appRepo) LoadApplicationsPaged(ctx context.Context, groupID string, pageable models.Pageable) ([]convoy.Application, models.PaginationData, error) {
	return make([]convoy.Application, 0), models.PaginationData{}, nil
}

func (a *appRepo) LoadApplicationsPagedByGroupId(ctx context.Context, groupID string, pageable models.Pageable) ([]convoy.Application, models.PaginationData, error) {
	return make([]convoy.Application, 0), models.PaginationData{}, nil
}

func (a *appRepo) SearchApplicationsByGroupId(ctx context.Context, groupId string, searchParams models.SearchParams) ([]convoy.Application, error) {
	return nil, nil
}

func (a *appRepo) FindApplicationByID(ctx context.Context, id string) (*convoy.Application, error) {
	return nil, nil
}

func (a *appRepo) FindApplicationEndpointByID(ctx context.Context, appID string, endpointID string) (*convoy.Endpoint, error) {
	return nil, nil
}

func (a *appRepo) UpdateApplication(ctx context.Context, app *convoy.Application) error {
	return nil
}

func (a *appRepo) DeleteApplication(ctx context.Context, app *convoy.Application) error {
	return nil
}

func (a *appRepo) UpdateApplicationEndpointsStatus(ctx context.Context, appId string, endpointIds []string, status convoy.EndpointStatus) error {
	return nil
}
