package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

const (
	createSource = `
    INSERT INTO convoy.sources (id, source_verifier_id, name,type,mask_id,provider,is_disabled,forward_headers,project_id, pub_sub)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10);
    `

	createSourceVerifier = `
    INSERT INTO convoy.source_verifiers (
        id,type,basic_username,basic_password,
        api_key_header_name,api_key_header_value,
        hmac_hash,hmac_header,hmac_secret,hmac_encoding
    )
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10);
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
	pub_sub= $9,
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
        hmac_encoding=$10,
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	baseFetchSource = `
	SELECT
		s.id,
		s.name,
		s.type,
		s.mask_id,
		s.provider,
		s.is_disabled,
		s.forward_headers,
		s.project_id,
		COALESCE(s.source_verifier_id, '') AS source_verifier_id,
		s.pub_sub,
		COALESCE(sv.type, '') as "verifier.type",
		COALESCE(sv.basic_username, '') as "verifier.basic_auth.username",
		COALESCE(sv.basic_password, '') as "verifier.basic_auth.password",
        COALESCE(sv.api_key_header_name, '') as "verifier.api_key.header_name",
        COALESCE(sv.api_key_header_value, '') as "verifier.api_key.header_value",
        COALESCE(sv.hmac_hash, '') as "verifier.hmac.hash",
        COALESCE(sv.hmac_header, '') as "verifier.hmac.header",
        COALESCE(sv.hmac_secret, '') as "verifier.hmac.secret",
        COALESCE(sv.hmac_encoding, '') as "verifier.hmac.encoding",
		s.created_at,
		s.updated_at
	FROM convoy.sources as s
	LEFT JOIN convoy.source_verifiers sv
		ON s.source_verifier_id = sv.id
	`

	fetchSource = baseFetchSource + ` WHERE %s = $1 AND s.deleted_at IS NULL;`

	fetchSourceByName = baseFetchSource + ` WHERE %s = $1 AND %s = $2 AND s.deleted_at IS NULL;`

	deleteSource = `
	UPDATE convoy.sources SET
	deleted_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteSourceVerifier = `
	UPDATE convoy.source_verifiers SET
	deleted_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteSourceSubscription = `
	UPDATE convoy.subscriptions SET
	deleted_at = now()
	WHERE source_id = $1 AND deleted_at IS NULL;
	`

	fetchSourcesPaginated = baseFetchSource + ` WHERE s.deleted_at IS NULL AND (s.type = :type OR :type = '') AND (s.provider = :provider OR :provider = '') AND (s.project_id = :project_id OR :project_id = '') ORDER BY s.id LIMIT :limit OFFSET :offset;`

	countSources = `
	SELECT COUNT(id) FROM convoy.sources WHERE deleted_at IS NULL
	AND (type = :type OR :type = '') AND (provider = :provider OR :provider = '');
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

func NewSourceRepo(db database.Database) datastore.SourceRepository {
	return &sourceRepo{db: db.GetDB()}
}

func (s *sourceRepo) CreateSource(ctx context.Context, source *datastore.Source) error {
	var sourceVerifierID *string
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
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

	if !util.IsStringEmpty(string(source.Verifier.Type)) {
		id := ulid.Make().String()
		sourceVerifierID = &id

		result2, err := tx.ExecContext(
			ctx, createSourceVerifier, sourceVerifierID, source.Verifier.Type, basic.UserName, basic.Password,
			apiKey.HeaderName, apiKey.HeaderValue, hmac.Hash, hmac.Header, hmac.Secret, hmac.Encoding,
		)
		if err != nil {
			return err
		}

		rowsAffected, err := result2.RowsAffected()
		if err != nil {
			return err
		}

		if rowsAffected < 1 {
			return ErrSourceVerifierNotCreated
		}
	}

	if !util.IsStringEmpty(string(source.Verifier.Type)) {
		source.VerifierID = *sourceVerifierID
	}

	result1, err := tx.ExecContext(
		ctx, createSource, source.UID, sourceVerifierID, source.Name, source.Type, source.MaskID,
		source.Provider, source.IsDisabled, pq.Array(source.ForwardHeaders), source.ProjectID, source.PubSub,
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

	return tx.Commit()
}

func (s *sourceRepo) UpdateSource(ctx context.Context, projectID string, source *datastore.Source) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(
		ctx, updateSourceById, source.UID, source.Name, source.Type, source.MaskID,
		source.Provider, source.IsDisabled, source.ForwardHeaders, source.ProjectID, source.PubSub,
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

	if !util.IsStringEmpty(string(source.Verifier.Type)) {
		result2, err := tx.ExecContext(
			ctx, updateSourceVerifierById, source.VerifierID, source.Verifier.Type, basic.UserName, basic.Password,
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
	}

	return tx.Commit()
}

func (s *sourceRepo) FindSourceByID(ctx context.Context, projectID string, id string) (*datastore.Source, error) {
	source := &datastore.Source{}
	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSource, "s.id"), id).StructScan(source)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrSourceNotFound
		}
		return nil, err
	}

	return source, nil
}

func (s *sourceRepo) FindSourceByName(ctx context.Context, projectID string, name string) (*datastore.Source, error) {
	source := &datastore.Source{}
	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSourceByName, "s.project_id", "s.name"), projectID, name).StructScan(source)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrSourceNotFound
		}
		return nil, err
	}

	return source, nil
}

func (s *sourceRepo) FindSourceByMaskID(ctx context.Context, maskID string) (*datastore.Source, error) {
	source := &datastore.Source{}
	err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSource, "s.mask_id"), maskID).StructScan(source)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrSourceNotFound
		}
		return nil, err
	}

	return source, nil
}

func (s *sourceRepo) DeleteSourceByID(ctx context.Context, projectID, id, sourceVeriferID string) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteSourceVerifier, sourceVeriferID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteSource, id)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteSourceSubscription, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sourceRepo) LoadSourcesPaged(ctx context.Context, projectID string, filter *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	arg := map[string]interface{}{
		"type":       filter.Type,
		"provider":   filter.Provider,
		"project_id": projectID,
		"limit":      pageable.Limit(),
		"offset":     pageable.Offset(),
	}

	query, args, err := sqlx.Named(fetchSourcesPaginated, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = s.db.Rebind(query)
	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	sources := make([]datastore.Source, 0)
	for rows.Next() {
		var source datastore.Source
		err = rows.StructScan(&source)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		sources = append(sources, source)
	}

	var count int
	query, args, err = sqlx.Named(countSources, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = s.db.Rebind(query)
	err = s.db.Get(&count, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return sources, pagination, nil
}
