package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/lib/pq"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

const (
	createSource = `
    INSERT INTO convoy.sources (id,name,type,mask_id,provider,is_disabled,forward_headers,project_id)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8);
    `

	createSourceVerifier = `
    INSERT INTO convoy.source_verifiers (
        id,type,basic_username,basic_password,
        api_key_header_name,api_key_header_value,
        hmac_hash,hmac_header,hmac_secret,hmac_encoding,source_id
    )
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11);
    `

	updateSourceById = `
	UPDATE convoy.sources SET
	name= $2,
	type=$3,
	mask_id=$4,
	provider = $5,
	is_disabled=$6,
	forward_headers=$7,
	project_id =$8,
	updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL ;
	`

	updateSourceVerifierById = `
	UPDATE convoy.source_verifiers SET
        type=$2,
        basic_username=$3,
        basic_password=$4,
        api_key_header_name=$5,
        api_key_header_value=$6,
        hmac_hash=$7,
        hmac_header=$8,
        hmac_secret=$9,
        hmac_encoding=$10
	WHERE source_id = $1;
	`

	fetchSource = `
	SELECT
		s.id,
		s.name,
		s.type,
		s.mask_id,
		s.provider,
		s.is_disabled,
		s.forward_headers,
		s.project_id,
		sv.type as "verifier.type",
		sv.basic_username as "verifier.basic_auth.username",
		sv.basic_password as "verifier.basic_auth.password",
        sv.api_key_header_name as "verifier.api_key.header_name",
        sv.api_key_header_value as "verifier.api_key.header_value",
        sv.hmac_hash as "verifier.hmac.hash",
        sv.hmac_header as "verifier.hmac.header",
        sv.hmac_secret as "verifier.hmac.secret",
        sv.hmac_encoding as "verifier.hmac.encoding",
		s.created_at,
		s.updated_at
	FROM convoy.sources as s
	LEFT JOIN convoy.source_verifiers sv
		ON s.id = sv.source_id
	WHERE %s = $1 AND s.deleted_at IS NULL;
	`

	deleteSource = `
	UPDATE convoy.sources SET
	deleted_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteSourceVerifier = `
	UPDATE convoy.source_verifiers SET
	deleted_at = now()
	WHERE source_id = $1 AND deleted_at IS NULL;
	`

	fetchSourcesPaginated = `
	SELECT * FROM convoy.sources
	WHERE deleted_at IS NULL
	ORDER BY id LIMIT $1 OFFSET $2;
	`
	countSources = `
	SELECT COUNT(id) FROM convoy.sources WHERE deleted_at IS NULL;
	`
)

var (
	ErrSourceNotCreated         = errors.New("source could not be created")
	ErrSourceVerifierNotCreated = errors.New("source verifier could not be created")
	ErrSourceVerifierNotUpdated = errors.New("source verifier could not be updated")
	ErrSourceNotUpdated         = errors.New("source could not be updated")
)

type sourceRepo struct {
	db *sqlx.DB
}

func NewSourceRepo(db *sqlx.DB) datastore.SourceRepository {
	return &sourceRepo{db: db}
}

func (s *sourceRepo) CreateSource(ctx context.Context, source *datastore.Source) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	source.UID = ulid.Make().String()
	result1, err := tx.ExecContext(
		ctx, createSource, source.UID, source.Name, source.Type, source.MaskID,
		source.Provider, source.IsDisabled, pq.Array(source.ForwardHeaders), source.ProjectID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result1.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrSourceNotCreated
	}

	var (
		hmac   datastore.HMac
		basic  datastore.BasicAuth
		apiKey datastore.ApiKey
	)

	switch source.Verifier.Type {
	case datastore.APIKeyVerifier:
		apiKey = *source.Verifier.ApiKey
	case datastore.BasicAuthVerifier:
		basic = *source.Verifier.BasicAuth
	case datastore.HMacVerifier:
		hmac = *source.Verifier.HMac
	}

	result2, err := tx.ExecContext(
		ctx, createSourceVerifier, ulid.Make().String(), source.Verifier.Type, basic.UserName, basic.Password,
		apiKey.HeaderName, apiKey.HeaderValue, hmac.Hash, hmac.Header, hmac.Secret, hmac.Encoding, source.UID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err = result2.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrSourceVerifierNotCreated
	}

	return tx.Commit()
}

func (s *sourceRepo) UpdateSource(ctx context.Context, projectID string, source *datastore.Source) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(
		ctx, updateSourceById, source.UID, source.Name, source.Type, source.MaskID,
		source.Provider, source.IsDisabled, source.ForwardHeaders, source.ProjectID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected < 1 {
		return ErrSourceNotUpdated
	}

	var (
		hmac   datastore.HMac
		basic  datastore.BasicAuth
		apiKey datastore.ApiKey
	)

	switch source.Verifier.Type {
	case datastore.APIKeyVerifier:
		apiKey = *source.Verifier.ApiKey
	case datastore.BasicAuthVerifier:
		basic = *source.Verifier.BasicAuth
	case datastore.HMacVerifier:
		hmac = *source.Verifier.HMac
	}

	result2, err := tx.ExecContext(
		ctx, updateSourceVerifierById, source.UID, source.Verifier.Type, basic.UserName, basic.Password,
		apiKey.HeaderName, apiKey.HeaderValue, hmac.Hash, hmac.Header, hmac.Secret, hmac.Encoding,
	)
	if err != nil {
		return err
	}

	rowsAffected, err = result2.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrSourceVerifierNotUpdated
	}

	return tx.Commit()
}

func (s *sourceRepo) FindSourceByID(ctx context.Context, projectID string, id string) (*datastore.Source, error) {
	source := &datastore.Source{}
	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSource, "s.id"), id).StructScan(source)
	if err != nil {
		return nil, err
	}

	return source, nil
}

func (s *sourceRepo) FindSourceByMaskID(ctx context.Context, maskID string) (*datastore.Source, error) {
	source := &datastore.Source{}
	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSource, "s.mask_id"), maskID).StructScan(source)
	if err != nil {
		return nil, err
	}

	return source, nil
}

func (s *sourceRepo) DeleteSourceByID(ctx context.Context, projectID string, id string) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteSourceVerifier, id)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteSource, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sourceRepo) LoadSourcesPaged(ctx context.Context, projectID string, filter *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	skip := (pageable.Page - 1) * pageable.PerPage
	rows, err := s.db.QueryxContext(ctx, fetchSourcesPaginated, pageable.PerPage, skip)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var apiKeys []datastore.Source
	err = rows.StructScan(&apiKeys)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var count int
	err = s.db.Get(&count, countSources)
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
