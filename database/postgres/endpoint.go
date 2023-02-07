package postgres

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrEndpointNotCreated = errors.New("endpoint could not be created")
)

type endpointRepo struct {
	db *sqlx.DB
}

func NewEndpointRepo(db *sqlx.DB) datastore.EndpointRepository {
	return &endpointRepo{db: db}
}

func (e *endpointRepo) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	return nil
}

func (e *endpointRepo) FindEndpointByID(ctx context.Context, id string) (*datastore.Endpoint, error) {
	return nil, nil
}

func (e *endpointRepo) FindEndpointsByID(ctx context.Context, ids []string) ([]datastore.Endpoint, error) {
	return nil, nil
}

func (e *endpointRepo) FindEndpointsByAppID(ctx context.Context, appID string) ([]datastore.Endpoint, error) {
	return nil, nil
}

func (e *endpointRepo) FindEndpointsByOwnerID(ctx context.Context, projectID string, ownerID string) ([]datastore.Endpoint, error) {
	return nil, nil
}

func (e *endpointRepo) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	return nil
}

func (e *endpointRepo) UpdateEndpointStatus(ctx context.Context, projectID string, endpointID string, status datastore.EndpointStatus) error {
	return nil
}

func (e *endpointRepo) DeleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint) error {
	return nil
}

func (e *endpointRepo) CountProjectEndpoints(ctx context.Context, projectID string) (int64, error) {
	return 0, nil
}

func (e *endpointRepo) DeleteProjectEndpoints(ctx context.Context, projectID string) error {
	return nil
}

func (e *endpointRepo) LoadEndpointsPaged(ctx context.Context, projectId string, query string, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}

func (e *endpointRepo) LoadEndpointsPagedByProjectId(ctx context.Context, projectID string, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}

func (e *endpointRepo) SearchEndpointsByProjectId(ctx context.Context, projectID string, param datastore.SearchParams) ([]datastore.Endpoint, error) {
	return nil, nil
}

func (e *endpointRepo) ExpireSecret(ctx context.Context, projectID string, endpointID string, secrets []datastore.Secret) error {
	return nil
}
