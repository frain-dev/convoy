package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/auth"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

const (
	createAPIKey = `
    INSERT INTO convoy.api_keys (id,name,key_type,mask_id,role_type,role_project,role_endpoint,hash,salt,user_id,expires_at)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11);
    `

	updateAPIKeyById = `
	UPDATE convoy.api_keys SET
	    name = $2,
		role_type= $3,
		role_project=$4,
		role_endpoint=$5,
		updated_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL ;
	`

	fetchAPIKey = `
	SELECT
	    id,
		name,
	    key_type,
	    mask_id,
	    COALESCE(role_type,'') AS "role.type",
	    COALESCE(role_project,'') AS "role.project",
	    COALESCE(role_endpoint,'') AS "role.endpoint",
	    hash,
	    salt,
	    COALESCE(user_id, '') AS user_id,
	    created_at,
	    updated_at,
	    expires_at
	FROM convoy.api_keys
	WHERE deleted_at IS NULL
	`

	deleteAPIKeys = `
	UPDATE convoy.api_keys SET
	deleted_at = NOW()
	WHERE id IN (?);
	`

	fetchAPIKeysPaged = `
	SELECT
	    id,
		name,
	    key_type,
	    mask_id,
	    COALESCE(role_type,'') AS "role.type",
	    COALESCE(role_project,'') AS "role.project",
	    COALESCE(role_endpoint,'') AS "role.endpoint",
	    hash,
	    salt,
	    COALESCE(user_id, '') AS user_id,
	    created_at,
	    updated_at,
	    expires_at
	FROM convoy.api_keys
	WHERE deleted_at IS NULL`

	baseApiKeysFilter = `
	AND (role_project = :project_id OR :project_id = '')
	AND (role_endpoint = :endpoint_id OR :endpoint_id = '')
	AND (user_id = :user_id OR :user_id = '')
	AND (key_type = :key_type OR :key_type = '')`

	baseFetchAPIKeysPagedForward = `
	%s
	%s
	AND id <= :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT :limit
	`

	baseFetchAPIKeysPagedBackward = `
	WITH api_keys AS (
		%s
		%s
		AND id >= :cursor
		GROUP BY id
		ORDER BY id ASC
		LIMIT :limit
	)

	SELECT * FROM api_keys ORDER BY id DESC
	`

	countPrevAPIKeys = `
	SELECT COUNT(DISTINCT(id)) AS count
	FROM convoy.api_keys s
	WHERE s.deleted_at IS NULL
	%s
	AND id > :cursor
	GROUP BY id
	ORDER BY id
	DESC LIMIT 1`
)

var (
	ErrAPIKeyNotCreated = errors.New("api key could not be created")
	ErrAPIKeyNotUpdated = errors.New("api key could not be updated")
	ErrAPIKeyNotRevoked = errors.New("api key could not be revoked")
)

type apiKeyRepo struct {
	db database.Database
}

func NewAPIKeyRepo(db database.Database) datastore.APIKeyRepository {
	return &apiKeyRepo{db: db}
}

func (a *apiKeyRepo) CreateAPIKey(ctx context.Context, key *datastore.APIKey) error {
	var (
		userID     *string
		endpointID *string
		projectID  *string
		roleType   *auth.RoleType
	)

	if !util.IsStringEmpty(key.UserID) {
		userID = &key.UserID
	}

	if !util.IsStringEmpty(key.Role.Endpoint) {
		endpointID = &key.Role.Endpoint
	}

	if !util.IsStringEmpty(key.Role.Project) {
		projectID = &key.Role.Project
	}

	if !util.IsStringEmpty(string(key.Role.Type)) {
		roleType = &key.Role.Type
	}

	result, err := a.db.GetDB().ExecContext(
		ctx, createAPIKey, key.UID, key.Name, key.Type, key.MaskID,
		roleType, projectID, endpointID, key.Hash,
		key.Salt, userID, key.ExpiresAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrAPIKeyNotCreated
	}

	return nil
}

func (a *apiKeyRepo) UpdateAPIKey(ctx context.Context, key *datastore.APIKey) error {
	var endpointID *string
	var projectID *string
	var roleType *auth.RoleType

	if !util.IsStringEmpty(key.Role.Endpoint) {
		endpointID = &key.Role.Endpoint
	}

	if !util.IsStringEmpty(key.Role.Project) {
		projectID = &key.Role.Project
	}

	if !util.IsStringEmpty(string(key.Role.Type)) {
		roleType = &key.Role.Type
	}

	result, err := a.db.GetDB().ExecContext(
		ctx, updateAPIKeyById, key.UID, key.Name, roleType, projectID, endpointID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrAPIKeyNotUpdated
	}

	return nil
}

func (a *apiKeyRepo) FindAPIKeyByID(ctx context.Context, id string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}
	err := a.db.GetReadDB().QueryRowxContext(ctx, fmt.Sprintf("%s AND id = $1;", fetchAPIKey), id).StructScan(apiKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrAPIKeyNotFound
		}
		return nil, err
	}

	return apiKey, nil
}

func (a *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}
	err := a.db.GetReadDB().QueryRowxContext(ctx, fmt.Sprintf("%s AND mask_id = $1;", fetchAPIKey), maskID).StructScan(apiKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrAPIKeyNotFound
		}
		return nil, err
	}

	return apiKey, nil
}

func (a *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}
	err := a.db.GetReadDB().QueryRowxContext(ctx, fmt.Sprintf("%s AND hash = $1;", fetchAPIKey), hash).StructScan(apiKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrAPIKeyNotFound
		}
		return nil, err
	}

	return apiKey, nil
}

func (a *apiKeyRepo) RevokeAPIKeys(ctx context.Context, ids []string) error {
	query, args, err := sqlx.In(deleteAPIKeys, ids)
	if err != nil {
		return err
	}

	result, err := a.db.GetReadDB().ExecContext(ctx, a.db.GetReadDB().Rebind(query), args...)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrAPIKeyNotRevoked
	}

	return nil
}

func (a *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, filter *datastore.ApiKeyFilter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	var query, filterQuery string
	var err error
	var args []interface{}

	arg := map[string]interface{}{
		"endpoint_ids": filter.EndpointIDs,
		"project_id":   filter.ProjectID,
		"endpoint_id":  filter.EndpointID,
		"user_id":      filter.UserID,
		"key_type":     filter.KeyType,
		"limit":        pageable.Limit(),
		"cursor":       pageable.Cursor(),
	}

	if pageable.Direction == datastore.Next {
		query = baseFetchAPIKeysPagedForward
	} else {
		query = baseFetchAPIKeysPagedBackward
	}

	filterQuery = baseApiKeysFilter
	if len(filter.EndpointIDs) > 0 {
		filterQuery += ` AND role_endpoint IN (:endpoint_ids)`
	}

	query = fmt.Sprintf(query, fetchAPIKeysPaged, filterQuery)

	query, args, err = sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = a.db.GetReadDB().Rebind(query)

	rows, err := a.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	var apiKeys []datastore.APIKey

	for rows.Next() {
		ak := ApiKeyPaginated{}
		err = rows.StructScan(&ak)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		apiKeys = append(apiKeys, ak.APIKey)
	}

	var count datastore.PrevRowCount
	if len(apiKeys) > 0 {
		var countQuery string
		var qargs []interface{}
		first := apiKeys[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := fmt.Sprintf(countPrevAPIKeys, filterQuery)
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = a.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err := a.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
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

	ids := make([]string, len(apiKeys))
	for i := range apiKeys {
		ids[i] = apiKeys[i].UID
	}

	if len(apiKeys) > pageable.PerPage {
		apiKeys = apiKeys[:len(apiKeys)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(*pageable, ids)

	return apiKeys, *pagination, nil
}

func (a *apiKeyRepo) FindAPIKeyByProjectID(ctx context.Context, projectID string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}
	err := a.db.GetReadDB().QueryRowxContext(ctx, fmt.Sprintf("%s AND role_project = $1;", fetchAPIKey), projectID).StructScan(apiKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrAPIKeyNotFound
		}
		return nil, err
	}

	return apiKey, nil
}

type ApiKeyPaginated struct {
	Count int `db:"count"`
	datastore.APIKey
}
