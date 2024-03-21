package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	ncache "github.com/frain-dev/convoy/cache/noop"
	"time"

	"github.com/lib/pq"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

const (
	createSource = `
    INSERT INTO convoy.sources (id,source_verifier_id,name,type,mask_id,provider,is_disabled,forward_headers,project_id,
                                pub_sub,custom_response_body,custom_response_content_type,idempotency_keys, body_function, header_function)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15);
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
	custom_response_body = $10,
	custom_response_content_type = $11,
	idempotency_keys = $12,
	body_function = $13,
	header_function = $14,
	updated_at = NOW()
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
		updated_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	baseFetchSource = `
	SELECT
		s.id,
		s.name,
		s.type,
		s.pub_sub,
		s.mask_id,
		s.provider,
		s.is_disabled,
		s.forward_headers,
		s.idempotency_keys,
		s.project_id,
		s.body_function,
		s.header_function,
		COALESCE(s.source_verifier_id, '') AS source_verifier_id,
		COALESCE(s.custom_response_body, '') AS "custom_response.body",
		COALESCE(s.custom_response_content_type, '') AS "custom_response.content_type",
		COALESCE(sv.type, '') AS "verifier.type",
		COALESCE(sv.basic_username, '') AS "verifier.basic_auth.username",
		COALESCE(sv.basic_password, '') AS "verifier.basic_auth.password",
        COALESCE(sv.api_key_header_name, '') AS "verifier.api_key.header_name",
        COALESCE(sv.api_key_header_value, '') AS "verifier.api_key.header_value",
        COALESCE(sv.hmac_hash, '') AS "verifier.hmac.hash",
        COALESCE(sv.hmac_header, '') AS "verifier.hmac.header",
        COALESCE(sv.hmac_secret, '') AS "verifier.hmac.secret",
        COALESCE(sv.hmac_encoding, '') AS "verifier.hmac.encoding",
		s.created_at,
		s.updated_at
	FROM convoy.sources AS s
	LEFT JOIN convoy.source_verifiers sv ON s.source_verifier_id = sv.id
	WHERE s.deleted_at IS NULL
	`

	fetchPubSubSources = `
	SELECT
	    id,
		name,
		type,
		pub_sub,
		mask_id,
		provider,
		is_disabled,
		forward_headers,
		idempotency_keys,
		body_function,
		header_function,
		project_id,
		created_at,
		updated_at
	FROM convoy.sources
	WHERE type = '%s' AND project_id IN (:project_ids) AND deleted_at IS NULL
	AND (id <= :cursor OR :cursor = '')
    ORDER BY id DESC
    LIMIT :limit
	`

	deleteSource = `
	UPDATE convoy.sources SET
	deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	deleteSourceVerifier = `
	UPDATE convoy.source_verifiers SET
	deleted_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteSourceSubscription = `
	UPDATE convoy.subscriptions SET
	deleted_at = NOW()
	WHERE source_id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	fetchSourcesPagedFilter = `
	AND (s.type = :type OR :type = '')
    AND (s.provider = :provider OR :provider = '')
	AND s.name ILIKE :query
	AND s.project_id = :project_id
	`

	fetchSourcesPagedForward = `
	%s
	%s
	AND s.id <= :cursor
	GROUP BY s.id, sv.id
	ORDER BY s.id DESC
	LIMIT :limit
	`

	fetchSourcesPagedBackward = `
	WITH sources AS (
		%s
		%s
		AND s.id >= :cursor
		GROUP BY s.id, sv.id
		ORDER BY s.id ASC
		LIMIT :limit
	)

	SELECT * FROM sources ORDER BY id DESC
	`

	countPrevSources = `
	SELECT COUNT(DISTINCT(s.id)) AS count
	FROM convoy.sources s
	WHERE s.deleted_at IS NULL
	%s
	AND s.id > :cursor GROUP BY s.id ORDER BY s.id DESC LIMIT 1`
)

var (
	fetchSource       = baseFetchSource + ` AND %s = $1;`
	fetchSourceByName = baseFetchSource + ` AND %s = $1 AND %s = $2;`
)

var (
	ErrSourceNotCreated         = errors.New("source could not be created")
	ErrSourceVerifierNotCreated = errors.New("source verifier could not be created")
	ErrSourceVerifierNotUpdated = errors.New("source verifier could not be updated")
	ErrSourceNotUpdated         = errors.New("source could not be updated")
)

type sourceRepo struct {
	db    *sqlx.DB
	cache cache.Cache
}

func NewSourceRepo(db database.Database, ca cache.Cache) datastore.SourceRepository {
	if ca == nil {
		ca = ncache.NewNoopCache()
	}
	return &sourceRepo{db: db.GetDB(), cache: ca}
}

func (s *sourceRepo) CreateSource(ctx context.Context, source *datastore.Source) error {
	var sourceVerifierID *string
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

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
		source.Provider, source.IsDisabled, pq.Array(source.ForwardHeaders), source.ProjectID,
		source.PubSub, source.CustomResponse.Body, source.CustomResponse.ContentType,
		source.IdempotencyKeys, source.BodyFunction, source.HeaderFunction,
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

	err = tx.Commit()
	if err != nil {
		return err
	}

	srcCacheKey := convoy.SourceCacheKey.Get(source.UID).String()
	err = s.cache.Set(ctx, srcCacheKey, source, time.Hour*24)
	if err != nil {
		return err
	}

	return nil
}

func (s *sourceRepo) UpdateSource(ctx context.Context, projectID string, source *datastore.Source) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	result, err := tx.ExecContext(
		ctx, updateSourceById, source.UID, source.Name, source.Type, source.MaskID,
		source.Provider, source.IsDisabled, source.ForwardHeaders, projectID,
		source.PubSub, source.CustomResponse.Body, source.CustomResponse.ContentType,
		source.IdempotencyKeys, source.BodyFunction, source.HeaderFunction,
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

	err = tx.Commit()
	if err != nil {
		return err
	}

	srcCacheKey := convoy.SourceCacheKey.Get(source.UID).String()
	err = s.cache.Set(ctx, srcCacheKey, source, time.Hour*24)
	if err != nil {
		return err
	}

	return nil
}

func (s *sourceRepo) FindSourceByID(ctx context.Context, projectId string, id string) (*datastore.Source, error) {
	fromCache, err := s.readFromCache(ctx, id, func() (*datastore.Source, error) {
		source := &datastore.Source{}
		err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSource, "s.id"), id).StructScan(source)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrSourceNotFound
			}
			return nil, err
		}

		return source, nil
	})

	if err != nil {
		return nil, err
	}

	return fromCache, nil
}

func (s *sourceRepo) FindSourceByName(ctx context.Context, projectID string, name string) (*datastore.Source, error) {
	fromCache, err := s.readFromCache(ctx, name, func() (*datastore.Source, error) {
		source := &datastore.Source{}
		err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSourceByName, "s.project_id", "s.name"), projectID, name).StructScan(source)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrSourceNotFound
			}
			return nil, err
		}

		return source, nil
	})

	if err != nil {
		return nil, err
	}

	return fromCache, nil
}

func (s *sourceRepo) FindSourceByMaskID(ctx context.Context, maskID string) (*datastore.Source, error) {
	fromCache, err := s.readFromCache(ctx, maskID, func() (*datastore.Source, error) {
		source := &datastore.Source{}
		err := s.db.QueryRowxContext(ctx, fmt.Sprintf(fetchSource, "s.mask_id"), maskID).StructScan(source)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrSourceNotFound
			}
			return nil, err
		}

		return source, nil
	})

	if err != nil {
		return nil, err
	}

	return fromCache, nil
}

func (s *sourceRepo) DeleteSourceByID(ctx context.Context, projectId string, id, sourceVerifierId string) error {
	tx, err := s.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	_, err = tx.ExecContext(ctx, deleteSourceVerifier, sourceVerifierId)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteSource, id, projectId)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteSourceSubscription, id, projectId)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	srcCacheKey := convoy.SourceCacheKey.Get(id).String()
	err = s.cache.Delete(ctx, srcCacheKey)
	if err != nil {
		return err
	}

	return nil
}

func (s *sourceRepo) LoadSourcesPaged(ctx context.Context, projectID string, filter *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	arg := map[string]interface{}{
		"type":       filter.Type,
		"provider":   filter.Provider,
		"project_id": projectID,
		"limit":      pageable.Limit(),
		"cursor":     pageable.Cursor(),
		"query":      "%" + filter.Query + "%",
	}

	var query string
	if pageable.Direction == datastore.Next {
		query = fetchSourcesPagedForward
	} else {
		query = fetchSourcesPagedBackward
	}

	query = fmt.Sprintf(query, baseFetchSource, fetchSourcesPagedFilter)

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = s.db.Rebind(query)

	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	sources := make([]datastore.Source, 0)
	for rows.Next() {
		var source datastore.Source
		err = rows.StructScan(&source)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		sources = append(sources, source)
	}

	var count datastore.PrevRowCount
	if len(sources) > 0 {
		var countQuery string
		var qargs []interface{}
		first := sources[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := fmt.Sprintf(countPrevSources, fetchSourcesPagedFilter)
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = s.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := s.db.QueryxContext(ctx, countQuery, qargs...)
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

	ids := make([]string, len(sources))
	for i := range sources {
		ids[i] = sources[i].UID
	}

	if len(sources) > pageable.PerPage {
		sources = sources[:len(sources)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return sources, *pagination, nil
}

func (s *sourceRepo) readFromCache(ctx context.Context, key string, readFromDB func() (*datastore.Source, error)) (*datastore.Source, error) {
	var source *datastore.Source
	srcCacheKey := convoy.SourceCacheKey.Get(key).String()
	err := s.cache.Get(ctx, srcCacheKey, &source)
	if err != nil {
		return nil, err
	}

	if source != nil {
		return source, err
	}

	fromDB, err := readFromDB()
	if err != nil {
		return nil, err
	}

	err = s.cache.Set(ctx, srcCacheKey, fromDB, time.Hour*24)
	if err != nil {
		return nil, err
	}

	return fromDB, err
}

func (s *sourceRepo) LoadPubSubSourcesByProjectIDs(ctx context.Context, projectIDs []string, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	arg := map[string]interface{}{
		"project_ids": projectIDs,
		"limit":       pageable.Limit(),
		"cursor":      pageable.Cursor(),
	}

	query := fmt.Sprintf(fetchPubSubSources, datastore.PubSubSource)

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = s.db.Rebind(query)

	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	sources := make([]datastore.Source, 0)
	for rows.Next() {
		var source datastore.Source
		err = rows.StructScan(&source)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		sources = append(sources, source)
	}

	// Bypass pagination.Build here since we're only dealing with forward paging here
	var hasNext bool
	var cursor string
	if len(sources) > pageable.PerPage {
		cursor = sources[len(sources)-1].UID
		sources = sources[:len(sources)-1]
		hasNext = true
	}

	pagination := &datastore.PaginationData{
		PerPage:        int64(pageable.PerPage),
		HasNextPage:    hasNext,
		NextPageCursor: cursor,
	}

	return sources, *pagination, nil
}
