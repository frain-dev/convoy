package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

var (
	ErrEndpointNotCreated = errors.New("endpoint could not be created")
	ErrEndpointNotFound   = errors.New("endpoint not found")
	ErrEndpointNotUpdated = errors.New("endpoint could not be updated")
)

const (
	createEndpoint = `
	INSERT INTO convoy.endpoints (
		id, title, status, owner_id, target_url, description, http_timeout, 
		rate_limit, rate_limit_duration, advanced_signatures, slack_webhook_url,
		support_email, app_id, project_id, authentication_type, authentication_type_api_key_header_name,
		authentication_type_api_key_header_value RETURNING id;
	)
	VALUES 
	  (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		$14, $15, $16, $17
	  ) RETURNING id;
	`

	createEndpointSecret = `
	INSERT INTO convoy.endpoint_secrets (id, value, expires_at, endpoint_id)
	VALUES ($1, $2, $3, $4) RETURNING id;
	`

	baseEndpointFetch = `
	SELECT * from convoy.endpoints as e
	LEFT JOIN convoy.endpoint_secrets as es ON
	e.id = es.endpoint_id
	`

	fetchEndpointById = baseEndpointFetch + `WHERE e.id = $1 AND e.deleted_at IS NULL;`

	fetchEndpointsById = baseEndpointFetch + `WHERE e.id IN (?) AND e.deleted_at IS NULL;`

	fetchEndpointsByAppId = baseEndpointFetch + `WHERE e.app_id = $1 AND e.deleted_at IS NULL;`

	fetchEndpointsByOwnerId = baseEndpointFetch + `WHERE e.project_id = $1 AND e.owner_id = $2 AND e.deleted_at IS NULL;`

	updateEndpoint = `
	UPDATE convoy.endpoints SET 
	title = $3,
	status = $4,
	owner_id = $5,
	target_url = $6,
	description = $7,
	http_timeout = $8,
	rate_limit = $9,
	rate_limit_duration = $10,
	advanced_signatures = $11,
	slack_webhook_url = $12,
	support_email = $13,
	app_id = $14,
	authentication_type = $15,
	authentication_type_api_key_header_name = $16,
	authentication_type_api_key_header_value = $17,
	updated_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at is NULL;
	`

	updateEndpointStatus = `
	UPDATE convoy.endpoints SET
	status = $3 WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	deleteEndpoint = `
	UPDATE convoy.endpoints SET deleted_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteEndpointSecrets = `
	UPDATE convoy.endpoint_secrets SET deleted_at = now()
	WHERE endpoint_id = $1 AND deleted_at IS NULL;
	`

	deleteEndpointSubscriptions = `
	UPDATE convoy.subscriptions SET deleted_at = now()
	WHERE endpoint_id = $1 AND deleted_at IS NULL;
	`

	countProjectEndpoints = `
	SELECT count(*) as count from convoy.endpoints 
	WHERE project_id = $1 AND deleted_at IS NULL;
	`

	fetchEndpointsPaginated = `
	SELECT count(*) as count OVER() * from convoy.endpoints as e 
	LEFT JOIN convoy.endpoint_secrets as es ON e.id = es.endpoint_id
	WHERE e.project_id = $3 AND (e.title = $4 OR $4 = '') AND e.deleted_at IS NULL
	ORDER BY id LIMIT = $1 OFFSET = $2;
	`

	expireEndpointSecret = `
	UPDATE convoy.endpoint_secrets SET expires_at = $3
	WHERE id = $1 AND endpoint_id = $2 AND deleted_at IS NULL;
	`
)

type endpointRepo struct {
	db *sqlx.DB
}

func NewEndpointRepo(db *sqlx.DB) datastore.EndpointRepository {
	return &endpointRepo{db: db}
}

func (e *endpointRepo) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	tx, err := e.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	var endpointID string
	args := []interface{}{
		ulid.Make().String(), endpoint.Title, endpoint.Status, endpoint.OwnerID, endpoint.TargetURL,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail, endpoint.AppID,
		endpoint.ProjectID, endpoint.Authentication.Type, endpoint.Authentication.ApiKey.HeaderName,
		endpoint.Authentication.ApiKey.HeaderValue,
	}

	err = tx.QueryRowxContext(ctx, createEndpoint, args...).Scan(&endpointID)
	if err != nil {
		return err
	}

	//fetch the most recent secret
	secret := endpoint.Secrets[len(endpoint.Secrets)-1]
	endpointResult, err := tx.ExecContext(ctx, createEndpointSecret, ulid.Make().String, secret.Value, secret.ExpiresAt, endpointID)
	if err != nil {
		return err
	}

	rowsAffected, err := endpointResult.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEndpointNotCreated
	}

	return tx.Commit()
}

func (e *endpointRepo) FindEndpointByID(ctx context.Context, id string) (*datastore.Endpoint, error) {
	endpoint := &datastore.Endpoint{}
	err := e.db.QueryRowxContext(ctx, fetchEndpointById, id).StructScan(endpoint)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEndpointNotFound
		}

		return nil, err
	}

	return endpoint, nil
}

func (e *endpointRepo) FindEndpointsByID(ctx context.Context, ids []string) ([]datastore.Endpoint, error) {
	query, args, err := sqlx.In(fetchEndpointsById, ids)
	if err != nil {
		return nil, err
	}

	query = e.db.Rebind(query)
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	var endpoints []datastore.Endpoint
	for rows.Next() {
		var endpoint datastore.Endpoint

		err = rows.StructScan(&endpoint)
		if err != nil {
			return nil, err
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

func (e *endpointRepo) FindEndpointsByAppID(ctx context.Context, appID string) ([]datastore.Endpoint, error) {
	rows, err := e.db.QueryxContext(ctx, fetchEndpointsByAppId, appID)
	if err != nil {
		return nil, err
	}

	var endpoints []datastore.Endpoint
	for rows.Next() {
		var endpoint datastore.Endpoint

		err = rows.StructScan(&endpoint)
		if err != nil {
			return nil, err
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

func (e *endpointRepo) FindEndpointsByOwnerID(ctx context.Context, projectID string, ownerID string) ([]datastore.Endpoint, error) {
	rows, err := e.db.QueryxContext(ctx, fetchEndpointsByOwnerId, projectID, ownerID)
	if err != nil {
		return nil, err
	}

	var endpoints []datastore.Endpoint
	for rows.Next() {
		var endpoint datastore.Endpoint

		err = rows.StructScan(&endpoint)
		if err != nil {
			return nil, err
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

func (e *endpointRepo) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	args := []interface{}{
		endpoint.Title, endpoint.Status, endpoint.OwnerID, endpoint.TargetURL,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail, endpoint.AppID,
		endpoint.ProjectID, endpoint.Authentication.Type, endpoint.Authentication.ApiKey.HeaderName,
		endpoint.Authentication.ApiKey.HeaderValue,
	}

	r, err := e.db.ExecContext(ctx, updateEndpoint, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEndpointNotUpdated
	}

	return nil
}

func (e *endpointRepo) UpdateEndpointStatus(ctx context.Context, projectID string, endpointID string, status datastore.EndpointStatus) error {
	r, err := e.db.ExecContext(ctx, updateEndpointStatus, endpointID, projectID, status)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEndpointNotUpdated
	}

	return nil
}

func (e *endpointRepo) DeleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint) error {
	tx, err := e.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteEndpoint, endpoint.UID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteEndpointSecrets, endpoint.UID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteEndpointSubscriptions, endpoint.UID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (e *endpointRepo) CountProjectEndpoints(ctx context.Context, projectID string) (int64, error) {
	var count int64

	err := e.db.QueryRowxContext(ctx, countProjectEndpoints, projectID).Scan(&count)
	if err != nil {
		return count, err
	}

	return count, nil
}

func (e *endpointRepo) LoadEndpointsPaged(ctx context.Context, projectId string, query string, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	rows, err := e.db.QueryxContext(ctx, fetchEndpointsPaginated, pageable.Limit(), pageable.Offset(), projectId, query)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	totalRecords := 0
	var endpoints []datastore.Endpoint
	for rows.Next() {
		var data EndpointPaginated

		err = rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		endpoints = append(endpoints, data.Endpoint)
		totalRecords = data.Count
	}

	pagination := calculatePaginationData(totalRecords, pageable.Page, pageable.PerPage)
	return endpoints, pagination, nil
}

func (e *endpointRepo) ExpireSecret(ctx context.Context, projectID string, endpointID string, expiredSecret, newSecret datastore.Secret) error {
	tx, err := e.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, expireEndpointSecret, expiredSecret.UID, endpointID, expiredSecret.ExpiresAt)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, createEndpointSecret, ulid.Make().String(), newSecret.Value, newSecret.ExpiresAt, endpointID)
	if err != nil {
		return err
	}


	return tx.Commit()
}

type EndpointPaginated struct {
	Count int
	datastore.Endpoint
}
