package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrEndpointNotCreated       = errors.New("endpoint could not be created")
	ErrEndpointNotFound         = errors.New("endpoint not found")
	ErrEndpointNotUpdated       = errors.New("endpoint could not be updated")
	ErrEndpointSecretNotDeleted = errors.New("endpoint secret could not be deleted")
)

const (
	createEndpoint = `
	INSERT INTO convoy.endpoints (
		id, title, status, owner_id, target_url, description, http_timeout, 
		rate_limit, rate_limit_duration, advanced_signatures, slack_webhook_url,
		support_email, app_id, project_id, authentication_type, authentication_type_api_key_header_name,
		authentication_type_api_key_header_value
	)
	VALUES 
	  (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		$14, $15, $16, $17
	  );
	`

	createEndpointSecret = `
	INSERT INTO convoy.endpoint_secrets (id, value, endpoint_id)
	VALUES ($1, $2, $3) RETURNING id;
	`

	baseEndpointFetch = `
	SELECT e.id AS "endpoint.id", e.title AS "endpoint.title", e.status AS "endpoint.status", e.owner_id AS "endpoint.owner_id",
	e.target_url AS "endpoint.target_url", e.description AS "endpoint.description", e.http_timeout AS "endpoint.http_timeout", e.rate_limit AS "endpoint.rate_limit",
	e.rate_limit_duration AS "endpoint.rate_limit_duration", e.advanced_signatures AS "endpoint.advanced_signatures", e.slack_webhook_url AS "endpoint.slack_webhook_url",
	e.support_email AS "endpoint.support_email", e.app_id AS "endpoint.app_id", e.project_id AS "endpoint.project_id",
	e.authentication_type AS "endpoint.authentication.type",
	e.authentication_type_api_key_header_name AS "endpoint.authentication.api_key.header_name",
	e.authentication_type_api_key_header_value AS "endpoint.authentication.api_key.header_value",
	e.created_at AS "endpoint.created_at", e.updated_at AS "endpoint.updated_at",
	es.id AS "secret.id", es.value AS "secret.value", es.created_at AS "secret.created_at",
	es.updated_at AS "secret.updated_at", es.expires_at AS "secret.expires_at"
	from convoy.endpoints AS e LEFT JOIN convoy.endpoint_secrets AS es ON
	e.id = es.endpoint_id
	`

	fetchEndpointById = baseEndpointFetch + `WHERE e.id = $1 AND e.project_id = $2 AND e.deleted_at IS NULL AND es.deleted_at IS NULL;`

	fetchEndpointsById = baseEndpointFetch + `WHERE e.id IN (?) AND e.project_id = ? AND e.deleted_at IS NULL AND es.deleted_at IS NULL;`

	fetchEndpointsByAppId = baseEndpointFetch + `WHERE e.app_id = $1 AND e.project_id = $2 AND e.deleted_at IS NULL AND es.deleted_at IS NULL;`

	fetchEndpointsByOwnerId = baseEndpointFetch + `WHERE e.project_id = $1 AND e.owner_id = $2 AND e.deleted_at IS NULL AND es.deleted_at IS NULL;`

	updateEndpoint = `
	UPDATE convoy.endpoints SET 
	title = $3, status = $4, owner_id = $5,
	target_url = $6, description = $7, http_timeout = $8,
	rate_limit = $9, rate_limit_duration = $10, advanced_signatures = $11,
	slack_webhook_url = $12, support_email = $13, app_id = $14,
	authentication_type = $15, authentication_type_api_key_header_name = $16,
	authentication_type_api_key_header_value = $17,
	updated_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at is NULL;
	`

	updateEndpointStatus = `
	UPDATE convoy.endpoints SET status = $3 
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	deleteEndpoint = `
	UPDATE convoy.endpoints SET deleted_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	deleteEndpointSecrets = `
	UPDATE convoy.endpoint_secrets AS es 
	SET deleted_at = now()
	FROM convoy.endpoints AS e
	WHERE es.endpoint_id = $1 AND e.project_id = $2
	AND es.endpoint_id = e.id AND es.deleted_at IS NULL;
	`

	deleteEndpointSubscriptions = `
	UPDATE convoy.subscriptions SET deleted_at = now()
	WHERE endpoint_id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	countProjectEndpoints = `
	SELECT count(*) as count from convoy.endpoints 
	WHERE project_id = $1 AND deleted_at IS NULL;
	`

	fetchEndpointsPaginated = `
	SELECT e.id AS "endpoint.id", e.title AS "endpoint.title", e.status AS "endpoint.status", e.owner_id AS "endpoint.owner_id",
	e.target_url AS "endpoint.target_url", e.description AS "endpoint.description", e.http_timeout AS "endpoint.http_timeout", e.rate_limit AS "endpoint.rate_limit",
	e.rate_limit_duration AS "endpoint.rate_limit_duration", e.advanced_signatures AS "endpoint.advanced_signatures", e.slack_webhook_url AS "endpoint.slack_webhook_url",
	e.support_email AS "endpoint.support_email", e.app_id AS "endpoint.app_id", e.project_id AS "endpoint.project_id",
	e.authentication_type AS "endpoint.authentication.type",
	e.authentication_type_api_key_header_name AS "endpoint.authentication.api_key.header_name",
	e.authentication_type_api_key_header_value AS "endpoint.authentication.api_key.header_value",
	e.created_at AS "endpoint.created_at", e.updated_at AS "endpoint.updated_at",
	es.id AS "secret.id", es.value AS "secret.value", es.created_at AS "secret.created_at",
	es.updated_at AS "secret.updated_at", es.expires_at AS "secret.expires_at"
	from convoy.endpoints AS e LEFT JOIN convoy.endpoint_secrets AS es ON
	e.id = es.endpoint_id AND e.project_id = $3 AND (e.title = $4 OR $4 = '')
	WHERE e.deleted_at IS NULL AND es.deleted_at IS NULL
	ORDER BY e.id LIMIT $1 OFFSET $2;
	`

	expireEndpointSecret = `
	UPDATE convoy.endpoint_secrets SET expires_at = $4
	FROM convoy.endpoint_secrets es LEFT JOIN convoy.endpoints e
	ON es.endpoint_id = e.id WHERE es.id = $1 AND es.endpoint_id = $2 AND e.project_id = $3 AND es.deleted_at IS NULL;
	`

	deleteEndpointSecret = `
	UPDATE convoy.endpoint_secrets AS es 
	SET deleted_at = now()
	FROM convoy.endpoints AS e
	WHERE es.id = $1 AND es.endpoint_id = $2 AND e.project_id = $3
	AND es.endpoint_id = e.id AND es.deleted_at IS NULL;
	`

	countEndpoints = `
	SELECT count(id) from convoy.endpoints WHERE project_id = $1 AND deleted_at IS NULL;
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

	ac := endpoint.GetAuthConfig()
	args := []interface{}{
		endpoint.UID, endpoint.Title, endpoint.Status, endpoint.OwnerID, endpoint.TargetURL,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail, endpoint.AppID,
		projectID, ac.Type, ac.ApiKey.HeaderName, ac.ApiKey.HeaderValue,
	}

	_, err = tx.ExecContext(ctx, createEndpoint, args...)
	if err != nil {
		return err
	}

	//fetch the most recent secret
	if len(endpoint.Secrets) > 0 {
		secret := endpoint.Secrets[len(endpoint.Secrets)-1]
		endpointResult, err := tx.ExecContext(ctx, createEndpointSecret, secret.UID, secret.Value, endpoint.UID)
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
	}

	return tx.Commit()
}

func (e *endpointRepo) FindEndpointByID(ctx context.Context, id, projectID string) (*datastore.Endpoint, error) {
	rows, err := e.db.QueryxContext(ctx, fetchEndpointById, id, projectID)
	if err != nil {
		return nil, err
	}

	var endpoint *datastore.Endpoint
	var data EndpointSecret
	secrets := make([]datastore.Secret, 0)
	for rows.Next() {
		err = rows.StructScan(&data)
		if err != nil {
			return nil, err
		}

		secrets = append(secrets, data.Secret)
		endpoint = &data.Endpoint
	}

	// We're doing this because QueryxContext doesn't return an
	// error if the row is empty
	if endpoint == nil {
		return nil, ErrEndpointNotFound
	}

	endpoint.Secrets = secrets
	return endpoint, nil
}

func (e *endpointRepo) FindEndpointsByID(ctx context.Context, ids []string, projectID string) ([]datastore.Endpoint, error) {
	query, args, err := sqlx.In(fetchEndpointsById, ids, projectID)
	if err != nil {
		return nil, err
	}

	query = e.db.Rebind(query)
	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return e.baseFetch(rows)
}

func (e *endpointRepo) FindEndpointsByAppID(ctx context.Context, appID, projectID string) ([]datastore.Endpoint, error) {
	rows, err := e.db.QueryxContext(ctx, fetchEndpointsByAppId, appID, projectID)
	if err != nil {
		return nil, err
	}

	return e.baseFetch(rows)
}

func (e *endpointRepo) FindEndpointsByOwnerID(ctx context.Context, projectID string, ownerID string) ([]datastore.Endpoint, error) {
	rows, err := e.db.QueryxContext(ctx, fetchEndpointsByOwnerId, projectID, ownerID)
	if err != nil {
		return nil, err
	}

	return e.baseFetch(rows)
}

func (e *endpointRepo) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	ac := endpoint.GetAuthConfig()
	args := []interface{}{
		endpoint.UID, projectID, endpoint.Title, endpoint.Status, endpoint.OwnerID, endpoint.TargetURL,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail, endpoint.AppID,
		ac.Type, ac.ApiKey.HeaderName, ac.ApiKey.HeaderValue,
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

func (e *endpointRepo) DeleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	tx, err := e.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteEndpoint, endpoint.UID, projectID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteEndpointSecrets, endpoint.UID, projectID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteEndpointSubscriptions, endpoint.UID, projectID)
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

	endpointMap := make(map[string]*datastore.Endpoint, 0)
	for rows.Next() {
		var data EndpointPaginated

		err := rows.StructScan(&data)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		endpoint, exists := endpointMap[data.Endpoint.UID]
		if exists {
			endpoint.Secrets = append(endpoint.Secrets, data.Secret)
		} else {
			e := data.Endpoint
			e.Secrets = []datastore.Secret{data.Secret}
			endpointMap[e.UID] = &e
		}
	}

	var endpoints []datastore.Endpoint
	for _, endpoint := range endpointMap {
		endpoints = append(endpoints, *endpoint)
	}
	var count int
	err = e.db.Get(&count, countEndpoints, projectId)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return endpoints, pagination, nil
}

func (e *endpointRepo) ExpireSecret(ctx context.Context, projectID string, endpointID string, expiredSecret, newSecret datastore.Secret) error {
	tx, err := e.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, expireEndpointSecret, expiredSecret.UID, endpointID, projectID, expiredSecret.ExpiresAt)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, createEndpointSecret, newSecret.UID, newSecret.Value, endpointID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (e *endpointRepo) DeleteSecret(ctx context.Context, endpoint *datastore.Endpoint, secretID, projectID string) error {
	r, err := e.db.ExecContext(ctx, deleteEndpointSecret, secretID, endpoint.UID, projectID)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEndpointSecretNotDeleted
	}

	return nil
}

func (e *endpointRepo) baseFetch(rows *sqlx.Rows) ([]datastore.Endpoint, error) {
	endpointMap := make(map[string]*datastore.Endpoint, 0)
	for rows.Next() {
		var data EndpointSecret

		err := rows.StructScan(&data)
		if err != nil {
			return nil, err
		}

		endpoint, exists := endpointMap[data.Endpoint.UID]
		if exists {
			endpoint.Secrets = append(endpoint.Secrets, data.Secret)
		} else {
			e := data.Endpoint
			e.Secrets = []datastore.Secret{data.Secret}
			endpointMap[e.UID] = &e
		}
	}

	var endpoints []datastore.Endpoint
	for _, endpoint := range endpointMap {
		endpoints = append(endpoints, *endpoint)
	}

	return endpoints, nil
}

type EndpointPaginated struct {
	EndpointSecret
}

type EndpointSecret struct {
	Endpoint datastore.Endpoint `json:"endpoint"`
	Secret   datastore.Secret   `db:"secret"`
}
