package postgres

import (
	"context"
	"errors"
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
	WHERE id = $1;
	`

	fetchAPIKey = `
	SELECT * FROM convoy.api_keys
	WHERE $1 = $2 AND deleted_at IS NULL;
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
	ErrAPIKeyNotCreated = errors.New("organization could not be created")
	ErrAPIKeyNotUpdated = errors.New("organization could not be updated")
	ErrAPIKeyNotRevoked = errors.New("organization could not be revoked")
)

type apiKeyRepo struct {
	db *sqlx.DB
}

func NewAPIKeyRepo(db *sqlx.DB) datastore.APIKeyRepository {
	return &apiKeyRepo{db: db}
}

func (a *apiKeyRepo) CreateAPIKey(ctx context.Context, key *datastore.APIKey) error {
	result, err := a.db.Exec(
		createAPIKey, key.Name, key.Type, key.MaskID,
		key.RoleType, key.RoleProject, key.RoleEndpoint, key.Hash,
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
		ctx, updateAPIKeyById, key.UID, key.RoleType, key.RoleProject, key.RoleEndpoint,
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
	var apiKey *datastore.APIKey
	err := a.db.QueryRowxContext(ctx, fetchAPIKey, "id", id).StructScan(apiKey)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (a *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	var apiKey *datastore.APIKey
	err := a.db.QueryRowxContext(ctx, fetchAPIKey, "mask_id", maskID).StructScan(apiKey)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (a *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	var apiKey *datastore.APIKey
	err := a.db.QueryRowxContext(ctx, fetchAPIKey, "hash", hash).StructScan(apiKey)
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
