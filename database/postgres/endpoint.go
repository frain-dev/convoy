package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/config"
	"strings"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"
)

var (
	ErrEndpointNotCreated = errors.New("endpoint could not be created")
	ErrEndpointNotUpdated = errors.New("endpoint could not be updated")
	ErrEndpointExists     = errors.New("an endpoint with that name already exists")
)

const (
	createEndpoint = `
	INSERT INTO convoy.endpoints (
		id, name, status, secrets, owner_id, url, description, http_timeout,
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
	e.id, e.name, e.status, e.owner_id,
	e.url, e.description, e.http_timeout,
	e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
	e.slack_webhook_url, e.support_email, e.app_id,
	e.project_id, e.secrets, e.created_at, e.updated_at,
	e.authentication_type AS "authentication.type",
	e.authentication_type_api_key_header_name AS "authentication.api_key.header_name",
	e.authentication_type_api_key_header_value AS "authentication.api_key.header_value"
	FROM convoy.endpoints AS e
	WHERE e.deleted_at IS NULL
	`

	fetchEndpointById = baseEndpointFetch + ` AND e.id = $1 AND e.project_id = $2;`

	fetchEndpointsById = baseEndpointFetch + ` AND e.id IN (?) AND e.project_id = ? GROUP BY e.id ORDER BY e.id;`

	fetchEndpointsByAppId = baseEndpointFetch + ` AND e.app_id = $1 AND e.project_id = $2 GROUP BY e.id ORDER BY e.id;`

	fetchEndpointsByOwnerId = baseEndpointFetch + ` AND e.project_id = $1 AND e.owner_id = $2 GROUP BY e.id ORDER BY e.id;`

	fetchEndpointByTargetURL = `
    SELECT e.id, e.name, e.status, e.owner_id, e.url,
    e.description, e.http_timeout, e.rate_limit, e.rate_limit_duration,
    e.advanced_signatures, e.slack_webhook_url, e.support_email,
    e.app_id, e.project_id, e.secrets, e.created_at, e.updated_at,
    e.authentication_type AS "authentication.type",
    e.authentication_type_api_key_header_name AS "authentication.api_key.header_name",
    e.authentication_type_api_key_header_value AS "authentication.api_key.header_value"
    FROM convoy.endpoints AS e WHERE e.deleted_at IS NULL AND e.url = $1 AND e.project_id = $2;
    `

	updateEndpoint = `
	UPDATE convoy.endpoints SET
	name = $3, status = $4, owner_id = $5,
	url = $6, description = $7, http_timeout = $8,
	rate_limit = $9, rate_limit_duration = $10, advanced_signatures = $11,
	slack_webhook_url = $12, support_email = $13,
	authentication_type = $14, authentication_type_api_key_header_name = $15,
	authentication_type_api_key_header_value = $16, secrets = $17,
	updated_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	updateEndpointStatus = `
	UPDATE convoy.endpoints SET status = $3
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL RETURNING
	id, name, status, owner_id, url,
    description, http_timeout, rate_limit, rate_limit_duration,
    advanced_signatures, slack_webhook_url, support_email,
    app_id, project_id, secrets, created_at, updated_at,
    authentication_type AS "authentication.type",
    authentication_type_api_key_header_name AS "authentication.api_key.header_name",
    authentication_type_api_key_header_value AS "authentication.api_key.header_value";
	`

	updateEndpointSecrets = `
	UPDATE convoy.endpoints SET
	    secrets = $3, updated_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL RETURNING
	id, name, status, owner_id, url,
    description, http_timeout, rate_limit, rate_limit_duration,
    advanced_signatures, slack_webhook_url, support_email,
    app_id, project_id, secrets, created_at, updated_at,
    authentication_type AS "authentication.type",
    authentication_type_api_key_header_name AS "authentication.api_key.header_name",
    authentication_type_api_key_header_value AS "authentication.api_key.header_value";
	`

	deleteEndpoint = `
	UPDATE convoy.endpoints SET deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	deleteEndpointSubscriptions = `
	UPDATE convoy.subscriptions SET deleted_at = NOW()
	WHERE endpoint_id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	countProjectEndpoints = `
	SELECT COUNT(*) AS count FROM convoy.endpoints
	WHERE project_id = $1 AND deleted_at IS NULL;
	`

	baseFetchEndpointsPaged = `
	SELECT
	e.id, e.name, e.status, e.owner_id,
	e.url, e.description, e.http_timeout,
	e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
	e.slack_webhook_url, e.support_email, e.app_id,
	e.project_id, e.secrets, e.created_at, e.updated_at,
	e.authentication_type AS "authentication.type",
	e.authentication_type_api_key_header_name AS "authentication.api_key.header_name",
	e.authentication_type_api_key_header_value AS "authentication.api_key.header_value"
	FROM convoy.endpoints AS e
	WHERE e.deleted_at IS NULL
	AND e.project_id = :project_id
	AND (e.owner_id = :owner_id OR :owner_id = '')
	AND (e.name ILIKE :name OR :name = '')`

	fetchEndpointsPagedForward = `
	%s
	%s
	AND e.id <= :cursor
	GROUP BY e.id
	ORDER BY e.id DESC
	LIMIT :limit
	`

	fetchEndpointsPagedBackward = `
	WITH endpoints AS (
	    %s
		%s
		AND e.id >= :cursor
		GROUP BY e.id
		ORDER BY e.id ASC
		LIMIT :limit
	)

	SELECT * FROM endpoints ORDER BY id DESC
	`

	countPrevEndpoints = `
	SELECT COUNT(DISTINCT(s.id)) AS count
	FROM convoy.endpoints s
	WHERE s.deleted_at IS NULL
	AND s.project_id = :project_id
	AND (s.name ILIKE :name OR :name = '')
	AND s.id > :cursor
	GROUP BY s.id
	ORDER BY s.id DESC
	LIMIT 1`
)

type endpointRepo struct {
	db    *sqlx.DB
	hook  *hooks.Hook
	cache cache.Cache
}

func NewEndpointRepo(db database.Database, ca cache.Cache) datastore.EndpointRepository {
	if ca == nil {
		ca = ncache.NewNoopCache()
	}
	return &endpointRepo{db: db.GetDB(), hook: db.GetHook(), cache: ca}
}

func (e *endpointRepo) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	ac := endpoint.GetAuthConfig()

	args := []interface{}{
		endpoint.UID, endpoint.Name, endpoint.Status, endpoint.Secrets, endpoint.OwnerID, endpoint.Url,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail, endpoint.AppID,
		projectID, ac.Type, ac.ApiKey.HeaderName, ac.ApiKey.HeaderValue,
	}

	result, err := e.db.ExecContext(ctx, createEndpoint, args...)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return ErrEndpointExists
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEndpointNotCreated
	}

	go e.hook.Fire(datastore.EndpointCreated, endpoint, nil)

	endpointCacheKey := convoy.EndpointCacheKey.Get(endpoint.UID).String()
	err = e.cache.Set(ctx, endpointCacheKey, endpoint, config.DefaultCacheTTL)
	if err != nil {
		return err
	}

	return nil
}

func (e *endpointRepo) FindEndpointByID(ctx context.Context, id, projectID string) (*datastore.Endpoint, error) {
	end, err := e.readFromCache(ctx, id, func() (*datastore.Endpoint, error) {
		endpoint := &datastore.Endpoint{}
		err := e.db.QueryRowxContext(ctx, fetchEndpointById, id, projectID).StructScan(endpoint)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrEndpointNotFound
			}
			return nil, err
		}

		return endpoint, nil
	})

	if err != nil {
		return nil, err
	}

	return end, nil
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

	r, err := e.db.ExecContext(ctx, updateEndpoint, endpoint.UID, projectID, endpoint.Name, endpoint.Status, endpoint.OwnerID, endpoint.Url,
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

	endpointCacheKey := convoy.EndpointCacheKey.Get(endpoint.UID).String()
	err = e.cache.Set(ctx, endpointCacheKey, endpoint, config.DefaultCacheTTL)
	if err != nil {
		return err
	}

	go e.hook.Fire(datastore.EndpointUpdated, endpoint, nil)
	return nil
}

func (e *endpointRepo) UpdateEndpointStatus(ctx context.Context, projectID string, endpointID string, status datastore.EndpointStatus) error {
	endpoint := datastore.Endpoint{}
	err := e.db.QueryRowxContext(ctx, updateEndpointStatus, endpointID, projectID, status).StructScan(&endpoint)
	if err != nil {
		return err
	}

	endpointCacheKey := convoy.EndpointCacheKey.Get(endpointID).String()
	err = e.cache.Set(ctx, endpointCacheKey, endpoint, config.DefaultCacheTTL)
	if err != nil {
		return err
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

	_, err = tx.ExecContext(ctx, deletePortalLinkEndpoints, nil, endpoint.UID)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	endpointCacheKey := convoy.EndpointCacheKey.Get(endpoint.UID).String()
	err = e.cache.Delete(ctx, endpointCacheKey)
	if err != nil {
		return err
	}

	go e.hook.Fire(datastore.EndpointDeleted, endpoint, nil)
	return nil
}

func (e *endpointRepo) CountProjectEndpoints(ctx context.Context, projectID string) (int64, error) {
	var count int64

	err := e.db.QueryRowxContext(ctx, countProjectEndpoints, projectID).Scan(&count)
	if err != nil {
		return count, err
	}

	return count, nil
}

func (e *endpointRepo) FindEndpointByTargetURL(ctx context.Context, projectID string, targetURL string) (*datastore.Endpoint, error) {
	endpoint, err := e.readFromCache(ctx, targetURL, func() (*datastore.Endpoint, error) {
		endpoint := &datastore.Endpoint{}
		err := e.db.QueryRowxContext(ctx, fetchEndpointByTargetURL, targetURL, projectID).StructScan(endpoint)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrEndpointNotFound
			}
			return nil, err
		}

		return endpoint, nil
	})
	if err != nil {
		return nil, err
	}

	return endpoint, nil
}

func (e *endpointRepo) LoadEndpointsPaged(ctx context.Context, projectId string, filter *datastore.Filter, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	q := filter.Query
	if !util.IsStringEmpty(q) {
		q = fmt.Sprintf("%%%s%%", q)
	}

	arg := map[string]interface{}{
		"project_id":   projectId,
		"owner_id":     filter.OwnerID,
		"limit":        pageable.Limit(),
		"cursor":       pageable.Cursor(),
		"endpoint_ids": filter.EndpointIDs,
		"name":         q,
	}

	var query, filterQuery string
	if pageable.Direction == datastore.Next {
		query = fetchEndpointsPagedForward
	} else {
		query = fetchEndpointsPagedBackward
	}

	if len(filter.EndpointIDs) > 0 {
		filterQuery = ` AND e.id IN (:endpoint_ids)`
	}

	query = fmt.Sprintf(query, baseFetchEndpointsPaged, filterQuery)
	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = e.db.Rebind(query)

	rows, err := e.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	endpoints, err := e.scanEndpoints(rows)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	ids := make([]string, len(endpoints))
	for i := range endpoints {
		ids[i] = endpoints[i].UID
	}

	if len(endpoints) > pageable.PerPage {
		endpoints = endpoints[:len(endpoints)-1]
	}

	var count datastore.PrevRowCount
	if len(endpoints) > 0 {
		var countQuery string
		var qargs []interface{}
		first := endpoints[0]
		qarg := arg
		qarg["cursor"] = first.UID

		countQuery, qargs, err = sqlx.Named(countPrevEndpoints, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = e.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := e.db.QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		defer closeWithError(rows)

		if rows.Next() {
			err = rows.StructScan(&count)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
		}
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return endpoints, *pagination, nil
}

func (e *endpointRepo) UpdateSecrets(ctx context.Context, endpointID string, projectID string, secrets datastore.Secrets) error {
	endpoint := datastore.Endpoint{}
	err := e.db.QueryRowxContext(ctx, updateEndpointSecrets, endpointID, projectID, secrets).StructScan(&endpoint)
	if err != nil {
		return err
	}

	endpointCacheKey := convoy.EndpointCacheKey.Get(endpointID).String()
	err = e.cache.Set(ctx, endpointCacheKey, endpoint, config.DefaultCacheTTL)
	if err != nil {
		return err
	}

	return nil
}

func (e *endpointRepo) DeleteSecret(ctx context.Context, endpoint *datastore.Endpoint, secretID, projectID string) error {
	sc := endpoint.FindSecret(secretID)
	if sc == nil {
		return datastore.ErrSecretNotFound
	}

	sc.DeletedAt = null.NewTime(time.Now(), true)

	updatedEndpoint := datastore.Endpoint{}
	err := e.db.QueryRowxContext(ctx, updateEndpointSecrets, endpoint.UID, projectID, endpoint.Secrets).StructScan(&updatedEndpoint)
	if err != nil {
		return err
	}

	endpointCacheKey := convoy.EndpointCacheKey.Get(updatedEndpoint.UID).String()
	err = e.cache.Set(ctx, endpointCacheKey, updatedEndpoint, config.DefaultCacheTTL)
	if err != nil {
		return err
	}

	return nil
}

func (e *endpointRepo) scanEndpoints(rows *sqlx.Rows) ([]datastore.Endpoint, error) {
	endpoints := make([]datastore.Endpoint, 0)
	defer closeWithError(rows)

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

func (e *endpointRepo) readFromCache(ctx context.Context, id string, readFromDB func() (*datastore.Endpoint, error)) (*datastore.Endpoint, error) {
	var endpoint *datastore.Endpoint
	endpointCacheKey := convoy.EndpointCacheKey.Get(id).String()
	err := e.cache.Get(ctx, endpointCacheKey, &endpoint)
	if err != nil {
		return nil, err
	}

	if endpoint != nil {
		return endpoint, err
	}

	end, err := readFromDB()
	if err != nil {
		return nil, err
	}

	err = e.cache.Set(ctx, endpointCacheKey, end, config.DefaultCacheTTL)
	if err != nil {
		return nil, err
	}

	return end, err
}

type EndpointPaginated struct {
	EndpointSecret
}

type EndpointSecret struct {
	Endpoint datastore.Endpoint `json:"endpoint"`
	Secret   datastore.Secret   `db:"secret"`
}
