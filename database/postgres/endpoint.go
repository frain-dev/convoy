package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"
)

var (
	ErrEndpointNotCreated       = errors.New("endpoint could not be created")
	ErrEndpointNotUpdated       = errors.New("endpoint could not be updated")
	ErrEndpointSecretNotDeleted = errors.New("endpoint secret could not be deleted")
)

const (
	createEndpoint = `
	INSERT INTO convoy.endpoints (
		id, title, status, secrets, owner_id, target_url, description, http_timeout,
		rate_limit, rate_limit_duration, advanced_signatures, slack_webhook_url,
		support_email, app_id, project_id, authentication_type, authentication_type_api_key_header_name,
		authentication_type_api_key_header_value
	)
	VALUES
	  (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		$14, $15, $16, $17, $18
	  );
	`

	baseEndpointFetch = `
	SELECT
	id, title, status, owner_id,
	target_url, description, http_timeout,
	rate_limit, rate_limit_duration, advanced_signatures,
	slack_webhook_url, support_email, app_id,
	project_id, secrets, created_at, updated_at,
	authentication_type AS "authentication.type",
	authentication_type_api_key_header_name AS "authentication.api_key.header_name",
	authentication_type_api_key_header_value AS "authentication.api_key.header_value"
	FROM convoy.endpoints
	`

	fetchEndpointById = baseEndpointFetch + `WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;`

	fetchEndpointsById = baseEndpointFetch + `WHERE id IN (?) AND project_id = ? AND deleted_at IS NULL;`

	fetchEndpointsByAppId = baseEndpointFetch + `WHERE app_id = $1 AND project_id = $2 AND deleted_at IS NULL;`

	fetchEndpointsByOwnerId = baseEndpointFetch + `WHERE project_id = $1 AND owner_id = $2 AND deleted_at IS NULL;`

	updateEndpoint = `
	UPDATE convoy.endpoints SET
	title = $3, status = $4, owner_id = $5,
	target_url = $6, description = $7, http_timeout = $8,
	rate_limit = $9, rate_limit_duration = $10, advanced_signatures = $11,
	slack_webhook_url = $12, support_email = $13,
	authentication_type = $14, authentication_type_api_key_header_name = $15,
	authentication_type_api_key_header_value = $16, secrets = $17,
	updated_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at is NULL;
	`

	updateEndpointStatus = `
	UPDATE convoy.endpoints SET status = $3
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	updateEndpointSecrets = `
	UPDATE convoy.endpoints SET
	    secrets = $3, updated_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	deleteEndpoint = `
	UPDATE convoy.endpoints SET deleted_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
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
	SELECT
	id, title, status, owner_id,
	target_url, description, http_timeout,
	rate_limit, rate_limit_duration, advanced_signatures,
	slack_webhook_url, support_email, app_id,
	project_id, secrets, created_at, updated_at,
	authentication_type AS "authentication.type",
	authentication_type_api_key_header_name AS "authentication.api_key.header_name",
	authentication_type_api_key_header_value AS "authentication.api_key.header_value"
	FROM convoy.endpoints WHERE deleted_at IS NULL AND project_id = $3 AND (title ILIKE $4 OR $4 = '')
	ORDER BY id LIMIT $1 OFFSET $2;
	`

	countEndpoints = `
	SELECT count(id) from convoy.endpoints WHERE project_id = $1 AND deleted_at IS NULL;
	`
)

type endpointRepo struct {
	db *sqlx.DB
}

func NewEndpointRepo(db database.Database) datastore.EndpointRepository {
	return &endpointRepo{db: db.GetDB()}
}

func (e *endpointRepo) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	tx, err := e.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	ac := endpoint.GetAuthConfig()
	args := []interface{}{
		endpoint.UID, endpoint.Title, endpoint.Status, endpoint.Secrets, endpoint.OwnerID, endpoint.TargetURL,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail, endpoint.AppID,
		projectID, ac.Type, ac.ApiKey.HeaderName, ac.ApiKey.HeaderValue,
	}

	result, err := tx.ExecContext(ctx, createEndpoint, args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEndpointNotCreated
	}

	return tx.Commit()
}

func (e *endpointRepo) FindEndpointByID(ctx context.Context, id, projectID string) (*datastore.Endpoint, error) {
	endpoint := &datastore.Endpoint{}
	err := e.db.QueryRowxContext(ctx, fetchEndpointById, id, projectID).StructScan(endpoint)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrEndpointNotFound
		}
		return nil, err
	}

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

	return e.scanEndpoints(rows)
}

func (e *endpointRepo) FindEndpointsByAppID(ctx context.Context, appID, projectID string) ([]datastore.Endpoint, error) {
	rows, err := e.db.QueryxContext(ctx, fetchEndpointsByAppId, appID, projectID)
	if err != nil {
		return nil, err
	}

	return e.scanEndpoints(rows)
}

func (e *endpointRepo) FindEndpointsByOwnerID(ctx context.Context, projectID string, ownerID string) ([]datastore.Endpoint, error) {
	rows, err := e.db.QueryxContext(ctx, fetchEndpointsByOwnerId, projectID, ownerID)
	if err != nil {
		return nil, err
	}

	return e.scanEndpoints(rows)
}

func (e *endpointRepo) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	ac := endpoint.GetAuthConfig()

	r, err := e.db.ExecContext(ctx, updateEndpoint, endpoint.UID, projectID, endpoint.Title, endpoint.Status, endpoint.OwnerID, endpoint.TargetURL,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail,
		ac.Type, ac.ApiKey.HeaderName, ac.ApiKey.HeaderValue, endpoint.Secrets,
	)
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
	defer rollbackTx(tx)

	_, err = tx.ExecContext(ctx, deleteEndpoint, endpoint.UID, projectID)
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
	if !util.IsStringEmpty(query) {
		query = fmt.Sprintf("%%%s%%", query)
	}

	rows, err := e.db.QueryxContext(ctx, fetchEndpointsPaginated, pageable.Limit(), pageable.Offset(), projectId, query)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	endpoints, err := e.scanEndpoints(rows)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var count int
	err = e.db.Get(&count, countEndpoints, projectId)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return endpoints, pagination, nil
}

func (e *endpointRepo) UpdateSecrets(ctx context.Context, endpointID string, projectID string, secrets datastore.Secrets) error {
	r, err := e.db.ExecContext(ctx, updateEndpointSecrets, endpointID, projectID, secrets)
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

func (e *endpointRepo) DeleteSecret(ctx context.Context, endpoint *datastore.Endpoint, secretID, projectID string) error {
	sc := endpoint.FindSecret(secretID)
	if sc == nil {
		return datastore.ErrSecretNotFound
	}

	sc.DeletedAt = null.NewTime(time.Now(), true)

	r, err := e.db.ExecContext(ctx, updateEndpointSecrets, endpoint.UID, projectID, endpoint.Secrets)
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

func (e *endpointRepo) scanEndpoints(rows *sqlx.Rows) ([]datastore.Endpoint, error) {
	endpoints := make([]datastore.Endpoint, 0)
	defer rows.Close()

	for rows.Next() {
		var endpoint datastore.Endpoint
		err := rows.StructScan(&endpoint)
		if err != nil {
			return nil, err
		}

		endpoints = append(endpoints, endpoint)
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
