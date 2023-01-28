package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

const (
	createAPIKey = `
    INSERT INTO convoy.api_keys (name,key_type,mask_id,role_type,role_project,role_endpoint,hash,salt,user_id,expires_at)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10);
    `

	updateAPIKeyById = `
	UPDATE convoy.api_keys SET
	role_type= $2,
	role_project=$3,
	role_endpoint=$4,
	updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL ;
	`

	fetchAPIKey = `
	SELECT
	    id,
		name,
	    key_type,
	    mask_id,
	    role_type as "role.type",
	    role_project as "role.project",
	    role_endpoint as "role.endpoint",
	    hash,
	    salt,
	    user_id,
	    created_at,
	    updated_at,
	    expires_at
	FROM convoy.api_keys
	WHERE %s = $1 AND deleted_at IS NULL;
	`

	deleteAPIKeys = `
	UPDATE convoy.api_keys SET
	deleted_at = now()
	WHERE id IN $1;
	`

	fetchAPIKeysPaginated = `
	SELECT * FROM convoy.api_keys
	WHERE deleted_at IS NULL
	ORDER BY id LIMIT $1 OFFSET $2;
	`
	countAPIKeys = `
	SELECT COUNT(id) FROM convoy.api_keys WHERE deleted_at IS NULL;
	`
)

var (
	ErrAPIKeyNotCreated = errors.New("api key could not be created")
	ErrAPIKeyNotUpdated = errors.New("api key could not be updated")
	ErrAPIKeyNotRevoked = errors.New("api key could not be revoked")
)

type apiKeyRepo struct {
	db *sqlx.DB
}

func NewAPIKeyRepo(db *sqlx.DB) datastore.APIKeyRepository {
	return &apiKeyRepo{db: db}
}

func (a *apiKeyRepo) CreateAPIKey(ctx context.Context, key *datastore.APIKey) error {
	result, err := a.db.ExecContext(
		ctx, createAPIKey, key.Name, key.Type, key.MaskID,
		key.Role.Type, key.Role.Project, key.Role.Endpoint, key.Hash,
		key.Salt, key.UserID, key.ExpiresAt,
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
	result, err := a.db.ExecContext(
		ctx, updateAPIKeyById, key.UID, key.Role.Type, key.Role.Project, key.Role.Endpoint,
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
		return nil, err
	}

	return apiKey, nil
}

func (a *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}
	err := a.db.QueryRowxContext(ctx, fmt.Sprintf(fetchAPIKey, "mask_id"), maskID).StructScan(apiKey)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (a *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}
	err := a.db.QueryRowxContext(ctx, fmt.Sprintf(fetchAPIKey, "hash"), hash).StructScan(apiKey)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (a *apiKeyRepo) RevokeAPIKeys(ctx context.Context, ids []string) error {
	result, err := a.db.ExecContext(ctx, deleteAPIKeys, ids)
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
	skip := (pageable.Page - 1) * pageable.PerPage
	rows, err := a.db.QueryxContext(ctx, fetchAPIKeysPaginated, pageable.PerPage, skip)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var apiKeys []datastore.APIKey
	err = rows.StructScan(&apiKeys)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var count int
	err = a.db.Get(&count, countAPIKeys)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := datastore.PaginationData{
		Total:     int64(count),
		Page:      int64(pageable.Page),
		PerPage:   int64(pageable.PerPage),
		Prev:      int64(getPrevPage(pageable.Page)),
		Next:      int64(pageable.Page + 1),
		TotalPage: int64(math.Ceil(float64(count) / float64(pageable.PerPage))),
	}

	return apiKeys, pagination, nil
}
