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
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL ;
	`

	fetchAPIKey = `
	SELECT
	    id,
		name,
	    key_type,
	    mask_id,
	    COALESCE(role_type,'') as "role.type",
	    COALESCE(role_project,'') as "role.project",
	    COALESCE(role_endpoint,'') as "role.endpoint",
	    hash,
	    salt,
	    COALESCE(user_id, '') AS user_id,
	    created_at,
	    updated_at,
	    expires_at
	FROM convoy.api_keys
	WHERE %s = $1 AND deleted_at IS NULL
	`

	deleteAPIKeys = `
	UPDATE convoy.api_keys SET
	deleted_at = now()
	WHERE id IN (?);
	`
	baseAPIKeysCount = `
	WITH table_count AS (
		SELECT count(distinct(id)) as count
		FROM convoy.api_keys WHERE deleted_at IS NULL
		%s
	)
	`

	fetchAPIKeysPaginated = `
	SELECT
	   table_count.count,
	    id,
		name,
	    key_type,
	    mask_id,
	    COALESCE(role_type,'') as "role.type",
	    COALESCE(role_project,'') as "role.project",
	    COALESCE(role_endpoint,'') as "role.endpoint",
	    hash,
	    salt,
	    COALESCE(user_id, '') AS user_id,
	    created_at,
	    updated_at,
	    expires_at
	FROM table_count, convoy.api_keys
	WHERE deleted_at IS NULL
	%s
	ORDER BY id LIMIT :limit OFFSET :offset;
	`

	baseFilter = `AND (role_project = :project_id OR :project_id = '') AND (role_endpoint = :endpoint_id OR :endpoint_id = '') AND (user_id = :user_id OR :user_id = '') AND (key_type = :key_type OR :key_type = '')`
)

var (
	ErrAPIKeyNotCreated = errors.New("api key could not be created")
	ErrAPIKeyNotUpdated = errors.New("api key could not be updated")
	ErrAPIKeyNotRevoked = errors.New("api key could not be revoked")
)

type apiKeyRepo struct {
	db *sqlx.DB
}

func NewAPIKeyRepo(db database.Database) datastore.APIKeyRepository {
	return &apiKeyRepo{db: db.GetDB()}
}

func (a *apiKeyRepo) CreateAPIKey(ctx context.Context, key *datastore.APIKey) error {
	var userID *string
	var endpointID *string
	var projectID *string
	var roleType *auth.RoleType

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
		roleType = (&key.Role.Type)
	}

	result, err := a.db.ExecContext(
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

	result, err := a.db.ExecContext(
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
	err := a.db.QueryRowxContext(ctx, fmt.Sprintf(fetchAPIKey, "id"), id).StructScan(apiKey)
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
	err := a.db.QueryRowxContext(ctx, fmt.Sprintf(fetchAPIKey, "mask_id"), maskID).StructScan(apiKey)
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
	err := a.db.QueryRowxContext(ctx, fmt.Sprintf(fetchAPIKey, "hash"), hash).StructScan(apiKey)
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

	result, err := a.db.ExecContext(ctx, a.db.Rebind(query), args...)
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
	var query string
	var err error
	var args []interface{}

	arg := map[string]interface{}{
		"endpoint_ids": filter.EndpointIDs,
		"project_id":   filter.ProjectID,
		"endpoint_id":  filter.EndpointID,
		"user_id":      filter.UserID,
		"key_type":     filter.KeyType,
		"limit":        pageable.Limit(),
		"offset":       pageable.Offset(),
	}

	if len(filter.EndpointIDs) > 0 {
		filterQuery := `AND role_endpoint IN (:endpoint_ids) ` + baseFilter
		q := fmt.Sprintf(baseAPIKeysCount, filterQuery) + fmt.Sprintf(fetchAPIKeysPaginated, filterQuery)
		query, args, err = sqlx.Named(q, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		query, args, err = sqlx.In(query, args...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		query = a.db.Rebind(query)
	} else {
		q := fmt.Sprintf(baseAPIKeysCount, baseFilter) + fmt.Sprintf(fetchAPIKeysPaginated, baseFilter)
		query, args, err = sqlx.Named(q, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		query = a.db.Rebind(query)
	}

	rows, err := a.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer rows.Close()

	var apiKeys []datastore.APIKey

	var count int
	for rows.Next() {
		ak := ApiKeyPaginated{}
		err = rows.StructScan(&ak)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		apiKeys = append(apiKeys, ak.APIKey)
		count = ak.Count
	}

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return apiKeys, pagination, nil
}

func (a *apiKeyRepo) FindAPIKeyByProjectID(ctx context.Context, projectID string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}
	err := a.db.QueryRowxContext(ctx, fmt.Sprintf(fetchAPIKey, "role_project"), projectID).StructScan(apiKey)
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
