package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/log"
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
                authentication_type_api_key_header_value,
                is_encrypted, secrets_cipher, authentication_type_api_key_header_value_cipher
            )
            VALUES
              (
                $1, $2, $3, CASE WHEN $19 THEN '[]'::jsonb ELSE $4::jsonb END,
                $5, $6, $7, $8, $9, $10, $11, $12, $13,
                $14, $15, $16, $17, CASE WHEN $19 THEN '' ELSE $18 END,
               $19,
               CASE WHEN $19 THEN pgp_sym_encrypt($4::TEXT, $20)  END, -- Ciphered values if encrypted
               CASE WHEN $19 THEN pgp_sym_encrypt($18, $20) END
              );
            `

	baseEndpointFetch = `
	SELECT
	e.id, e.name, e.status, e.owner_id,
	e.url, e.description, e.http_timeout,
	e.rate_limit, e.rate_limit_duration, e.advanced_signatures,
	e.slack_webhook_url, e.support_email, e.app_id,
	e.project_id,
	CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, $1)::jsonb
        ELSE e.secrets
    END AS secrets, e.created_at, e.updated_at,
	e.authentication_type AS "authentication.type",
	e.authentication_type_api_key_header_name AS "authentication.api_key.header_name",
	CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, $1)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS "authentication.api_key.header_value"
	FROM convoy.endpoints AS e
	WHERE e.deleted_at IS NULL
	`

	fetchEndpointById = baseEndpointFetch + ` AND e.id = $2 AND e.project_id = $3;`

	fetchEndpointsById = baseEndpointFetch + ` AND e.id IN (?) AND e.project_id = ? GROUP BY e.id ORDER BY e.id;`

	fetchEndpointsByAppId = baseEndpointFetch + ` AND e.app_id = $2 AND e.project_id = $3 GROUP BY e.id ORDER BY e.id;`

	fetchEndpointsByOwnerId = baseEndpointFetch + ` AND e.project_id = $2 AND e.owner_id = $3 GROUP BY e.id ORDER BY e.id;`

	fetchEndpointByTargetURL = `
    SELECT e.id, e.name, e.status, e.owner_id, e.url,
    e.description, e.http_timeout, e.rate_limit, e.rate_limit_duration,
    e.advanced_signatures, e.slack_webhook_url, e.support_email,
    e.app_id, e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, $3)::jsonb
        ELSE e.secrets
    END AS secrets, e.created_at, e.updated_at,
    e.authentication_type AS "authentication.type",
    e.authentication_type_api_key_header_name AS "authentication.api_key.header_name",
	CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, $3)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS "authentication.api_key.header_value"
    FROM convoy.endpoints AS e WHERE e.deleted_at IS NULL AND e.url = $1 AND e.project_id = $2;
    `

	updateEndpoint = `
	UPDATE convoy.endpoints SET
	name = $3, status = $4, owner_id = $5,
	url = $6, description = $7, http_timeout = $8,
	rate_limit = $9, rate_limit_duration = $10, advanced_signatures = $11,
	slack_webhook_url = $12, support_email = $13,
	authentication_type = $14, authentication_type_api_key_header_name = $15,
	authentication_type_api_key_header_value_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt($16, $18)
    END,
    authentication_type_api_key_header_value = CASE
        WHEN is_encrypted THEN ''
        ELSE $16
    END,
    secrets_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt($17::jsonb::TEXT, $18)
    END,
    secrets = CASE
        WHEN is_encrypted THEN '[]'
        ELSE $17
    END,
	updated_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	updateEndpointStatus = `
	UPDATE convoy.endpoints SET status = $3
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL RETURNING
	id, name, status, owner_id, url,
    description, http_timeout, rate_limit, rate_limit_duration,
    advanced_signatures, slack_webhook_url, support_email,
    app_id, project_id,
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(secrets_cipher::bytea, $4)::jsonb
        ELSE secrets
    END AS secrets, created_at, updated_at,
    authentication_type AS "authentication.type",
    authentication_type_api_key_header_name AS "authentication.api_key.header_name",
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(authentication_type_api_key_header_value_cipher::bytea, $4)::TEXT
        ELSE authentication_type_api_key_header_value
    END AS "authentication.api_key.header_value";
	`

	updateEndpointSecrets = `
	UPDATE convoy.endpoints SET
	    secrets_cipher = CASE
        WHEN is_encrypted THEN pgp_sym_encrypt($3::jsonb::TEXT, $4)
        END,
        secrets = CASE
            WHEN is_encrypted THEN '[]'
            ELSE $3
        END,
	    updated_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL RETURNING
	id, name, status, owner_id, url,
    description, http_timeout, rate_limit, rate_limit_duration,
    advanced_signatures, slack_webhook_url, support_email,
    app_id, project_id,
	CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(secrets_cipher::bytea, $4)::jsonb
        ELSE secrets
    END AS secrets,
	created_at, updated_at,
    authentication_type AS "authentication.type",
    authentication_type_api_key_header_name AS "authentication.api_key.header_name",
    CASE
        WHEN is_encrypted THEN pgp_sym_decrypt(authentication_type_api_key_header_value_cipher::bytea, $4)::TEXT
        ELSE authentication_type_api_key_header_value
    END AS "authentication.api_key.header_value";
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
	e.project_id,
    CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.secrets_cipher::bytea, :encryption_key)::jsonb
        ELSE e.secrets
    END AS secrets, e.created_at, e.updated_at,
	e.authentication_type AS "authentication.type",
	e.authentication_type_api_key_header_name AS "authentication.api_key.header_name",
	CASE
        WHEN e.is_encrypted THEN pgp_sym_decrypt(e.authentication_type_api_key_header_value_cipher::bytea, :encryption_key)::TEXT
        ELSE e.authentication_type_api_key_header_value
    END AS "authentication.api_key.header_value"
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
	db   database.Database
	hook *hooks.Hook
	km   keys.KeyManager
}

func NewEndpointRepo(db database.Database) datastore.EndpointRepository {
	km, err := keys.Get()
	if err != nil {
		log.Fatal(err)
	}
	return &endpointRepo{db: db, hook: db.GetHook(), km: km}
}

// checkEncryptionStatus checks if any row is already encrypted.
func checkEncryptionStatus(db database.Database) (bool, error) {
	checkQuery := "SELECT is_encrypted FROM convoy.endpoints WHERE is_encrypted=TRUE LIMIT 1;"
	var isEncrypted bool
	err := db.GetReadDB().Get(&isEncrypted, checkQuery)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return false, fmt.Errorf("failed to check encryption status of endpoints: %w", err)
	}

	return isEncrypted, nil
}

func (e *endpointRepo) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	ac := endpoint.GetAuthConfig()
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}

	isEncrypted, err := checkEncryptionStatus(e.db)
	if err != nil {
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return err2
		}
		return err
	}

	args := []interface{}{
		endpoint.UID, endpoint.Name, endpoint.Status, endpoint.Secrets, endpoint.OwnerID, endpoint.Url,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail, endpoint.AppID,
		projectID, ac.Type, ac.ApiKey.HeaderName, ac.ApiKey.HeaderValue, isEncrypted, key,
	}

	result, err := e.db.GetDB().ExecContext(ctx, createEndpoint, args...)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return ErrEndpointExists
		}
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return err2
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

	// todo(raymond): we should not run this in the foreground.
	// go e.hook.Fire(ctx, datastore.EndpointCreated, endpoint, nil)

	return nil
}

func (e *endpointRepo) FindEndpointByID(ctx context.Context, id, projectID string) (*datastore.Endpoint, error) {
	endpoint := &datastore.Endpoint{}
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	err = e.db.GetReadDB().QueryRowxContext(ctx, fetchEndpointById, key, id, projectID).StructScan(endpoint)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrEndpointNotFound
		}
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	return endpoint, nil
}

func (e *endpointRepo) FindEndpointsByID(ctx context.Context, ids []string, projectID string) ([]datastore.Endpoint, error) {
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	query, args, err := sqlx.In(strings.Replace(fetchEndpointsById, "$1", "?", 1), key, ids, projectID)
	if err != nil {
		return nil, err
	}

	query = e.db.GetReadDB().Rebind(query)
	rows, err := e.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	return e.scanEndpoints(rows)
}

func (e *endpointRepo) FindEndpointsByAppID(ctx context.Context, appID, projectID string) ([]datastore.Endpoint, error) {
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	rows, err := e.db.GetReadDB().QueryxContext(ctx, fetchEndpointsByAppId, key, appID, projectID)
	if err != nil {
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	return e.scanEndpoints(rows)
}

func (e *endpointRepo) FindEndpointsByOwnerID(ctx context.Context, projectID string, ownerID string) ([]datastore.Endpoint, error) {
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	rows, err := e.db.GetReadDB().QueryxContext(ctx, fetchEndpointsByOwnerId, key, projectID, ownerID)
	if err != nil {
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	return e.scanEndpoints(rows)
}

func (e *endpointRepo) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	ac := endpoint.GetAuthConfig()

	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}

	r, err := e.db.GetReadDB().ExecContext(ctx, updateEndpoint, endpoint.UID, projectID, endpoint.Name, endpoint.Status, endpoint.OwnerID, endpoint.Url,
		endpoint.Description, endpoint.HttpTimeout, endpoint.RateLimit, endpoint.RateLimitDuration,
		endpoint.AdvancedSignatures, endpoint.SlackWebhookURL, endpoint.SupportEmail,
		ac.Type, ac.ApiKey.HeaderName, ac.ApiKey.HeaderValue, endpoint.Secrets, key,
	)
	if err != nil {
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return err2
		}
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrEndpointNotUpdated
	}

	// todo(raymond): we should not run this in the foreground.
	// go e.hook.Fire(ctx, datastore.EndpointUpdated, endpoint, nil)
	return nil
}

func (e *endpointRepo) UpdateEndpointStatus(ctx context.Context, projectID string, endpointID string, status datastore.EndpointStatus) error {
	endpoint := datastore.Endpoint{}
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}
	err = e.db.GetReadDB().QueryRowxContext(ctx, updateEndpointStatus, endpointID, projectID, status, key).StructScan(&endpoint)
	if err != nil {
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return err2
		}
		return err
	}

	return nil
}

func (e *endpointRepo) DeleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	tx, err := e.db.GetDB().BeginTxx(ctx, &sql.TxOptions{})
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

	// todo(raymond): we should not run this in the foreground.
	// go e.hook.Fire(ctx, datastore.EndpointDeleted, endpoint, nil)
	return nil
}

func (e *endpointRepo) CountProjectEndpoints(ctx context.Context, projectID string) (int64, error) {
	var count int64

	err := e.db.GetReadDB().QueryRowxContext(ctx, countProjectEndpoints, projectID).Scan(&count)
	if err != nil {
		return count, err
	}

	return count, nil
}

func (e *endpointRepo) FindEndpointByTargetURL(ctx context.Context, projectID string, targetURL string) (*datastore.Endpoint, error) {
	endpoint := &datastore.Endpoint{}
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}
	err = e.db.GetReadDB().QueryRowxContext(ctx, fetchEndpointByTargetURL, targetURL, projectID, key).StructScan(endpoint)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrEndpointNotFound
		}
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	return endpoint, nil
}

func (e *endpointRepo) LoadEndpointsPaged(ctx context.Context, projectId string, filter *datastore.Filter, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	q := filter.Query
	if !util.IsStringEmpty(q) {
		q = fmt.Sprintf("%%%s%%", q)
	}

	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	arg := map[string]interface{}{
		"encryption_key": key,
		"project_id":     projectId,
		"owner_id":       filter.OwnerID,
		"limit":          pageable.Limit(),
		"cursor":         pageable.Cursor(),
		"endpoint_ids":   filter.EndpointIDs,
		"name":           q,
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

	query = e.db.GetReadDB().Rebind(query)

	query = strings.ReplaceAll(query, ":", "::")

	rows, err := e.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		isEncErr, err2 := e.isEncryptionError(err)
		if isEncErr && err2 != nil {
			return nil, datastore.PaginationData{}, err2
		}
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

		countQuery = e.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err = e.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
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

func (e *endpointRepo) isEncryptionError(err error) (bool, error) {
	if strings.Contains(err.Error(), "Illegal argument") {
		isEncrypted, err2 := checkEncryptionStatus(e.db)
		if err2 == nil && isEncrypted {
			return true, keys.ErrCredentialEncryptionFeatureUnavailableUpgradeOrRevert
		}
	}
	return false, nil
}

func (e *endpointRepo) UpdateSecrets(ctx context.Context, endpointID string, projectID string, secrets datastore.Secrets) error {
	endpoint := datastore.Endpoint{}
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}
	err = e.db.GetReadDB().QueryRowxContext(ctx, updateEndpointSecrets, endpointID, projectID, secrets, key).StructScan(&endpoint)
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
	key, err := e.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}
	err = e.db.GetReadDB().QueryRowxContext(ctx, updateEndpointSecrets, endpoint.UID, projectID, endpoint.Secrets, key).StructScan(&updatedEndpoint)
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

type EndpointPaginated struct {
	EndpointSecret
}

type EndpointSecret struct {
	Endpoint datastore.Endpoint `json:"endpoint"`
	Secret   datastore.Secret   `db:"secret"`
}
