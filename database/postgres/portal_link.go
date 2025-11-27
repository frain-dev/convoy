package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dchest/uniuri"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/xdg-go/pbkdf2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/migrations"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

var (
	ErrPortalLinkNotCreated          = errors.New("portal link could not be created")
	ErrPortalLinkNotUpdated          = errors.New("portal link could not be updated")
	ErrPortalLinkNotDeleted          = errors.New("portal link could not be deleted")
	ErrPortalLinkAuthTokenNotCreated = errors.New("portal link auth token could not be created")
)

const (
	createPortalLink = `
	INSERT INTO convoy.portal_links (id, project_id, name, token, endpoints, owner_id, can_manage_endpoint, auth_type)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
	`

	createPortalLinkAuthToken = `
	insert into convoy.portal_tokens (id, portal_link_id, token_mask_id, token_hash, token_salt, token_expires_at)
	VALUES ($1, $2, $3, $4, $5, $6);
	`

	bulkWritePortalAuthTokens = `
	INSERT INTO convoy.portal_tokens (id, portal_link_id, token_mask_id, token_hash, token_salt, token_expires_at)
	VALUES (:id, :portal_link_id, :mask_id, :hash, :salt, :expires_at)
	`

	createPortalLinkEndpoints = `
	INSERT INTO convoy.portal_links_endpoints (portal_link_id, endpoint_id) VALUES (:portal_link_id, :endpoint_id)
	`

	updatePortalLink = `
	UPDATE convoy.portal_links
	SET
		endpoints = $3,
		owner_id = $4,
		can_manage_endpoint = $5,
		name = $6,
		auth_type = $7,
		updated_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	deletePortalLinkEndpoints = `
	DELETE FROM convoy.portal_links_endpoints
	WHERE portal_link_id = $1 OR endpoint_id = $2
	`

	updateEndpointOwnerID = `
	UPDATE convoy.endpoints
	SET owner_id = $3
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL
	`

	fetchPortalLinkById = `
	SELECT
	p.id,
	p.project_id,
	p.name,
	p.token,
	p.endpoints,
	p.auth_type,
	COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
	COALESCE(p.owner_id, '') AS "owner_id",
	CASE
		WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
		ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
	END AS endpoint_count,
	p.created_at,
	p.updated_at,
	ARRAY_TO_JSON(ARRAY_AGG(DISTINCT CASE WHEN e.id IS NOT NULL THEN cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb) END)) AS endpoints_metadata
	FROM convoy.portal_links p
	LEFT JOIN convoy.portal_links_endpoints pe
		ON p.id = pe.portal_link_id
	LEFT JOIN convoy.endpoints e
		ON e.id = pe.endpoint_id
	WHERE p.id = $1 AND p.project_id = $2 AND p.deleted_at IS NULL
	GROUP BY p.id;
	`

	fetchPortalLinkByOwnerID = `
	SELECT
	p.id,
	p.project_id,
	p.name,
	p.token,
	p.endpoints,
	p.auth_type,
	COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
	COALESCE(p.owner_id, '') AS "owner_id",
	CASE
		WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
		ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
	END AS endpoint_count,
	p.created_at,
	p.updated_at,
	ARRAY_TO_JSON(ARRAY_AGG(DISTINCT CASE WHEN e.id IS NOT NULL THEN cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb) END)) AS endpoints_metadata
	FROM convoy.portal_links p
	LEFT JOIN convoy.portal_links_endpoints pe
		ON p.id = pe.portal_link_id
	LEFT JOIN convoy.endpoints e
		ON e.id = pe.endpoint_id
	WHERE p.owner_id = $1 AND p.project_id = $2 AND p.deleted_at IS NULL
	GROUP BY p.id;
	`

	fetchPortalLinkByToken = `
	SELECT
	p.id,
	p.project_id,
	p.name,
	p.token,
	p.endpoints,
	p.auth_type,
	COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
	COALESCE(p.owner_id, '') AS "owner_id",
	CASE
		WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
		ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
	END AS endpoint_count,
	p.created_at,
	p.updated_at,
	ARRAY_TO_JSON(ARRAY_AGG(DISTINCT CASE WHEN e.id IS NOT NULL THEN cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb) END)) AS endpoints_metadata
	FROM convoy.portal_links p
	LEFT JOIN convoy.portal_links_endpoints pe
		ON p.id = pe.portal_link_id
	LEFT JOIN convoy.endpoints e
		ON e.id = pe.endpoint_id
	WHERE p.token = $1 AND p.deleted_at IS NULL
	GROUP BY p.id;
	`

	fetchPortalLinkByMaskId = `
	SELECT
	    pl.id, pl.project_id, pt.token_salt, pt.token_mask_id, pt.token_expires_at, pt.token_hash, pl.name, pl.token, pl.endpoints, pl.auth_type,
		COALESCE(pl.can_manage_endpoint, FALSE) AS "can_manage_endpoint", COALESCE(pl.owner_id, '') AS "owner_id",
		CASE
			WHEN pl.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = pl.owner_id)
			ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = pl.id)
		END AS endpoint_count
	FROM convoy.portal_tokens pt
		join convoy.portal_links pl on pl.id = pt.portal_link_id
	WHERE pt.token_mask_id = $1;
	`

	countPrevPortalLinks = `
	SELECT COUNT(DISTINCT(p.id)) AS count
	FROM convoy.portal_links p
	LEFT JOIN convoy.portal_links_endpoints pe
		ON p.id = pe.portal_link_id
	LEFT JOIN convoy.endpoints e
		ON e.id = pe.endpoint_id
	WHERE p.deleted_at IS NULL
	%s
	AND p.id > :cursor GROUP BY p.id ORDER BY p.id DESC LIMIT 1`

	fetchPortalLinksPaginated = `
	SELECT
	p.id,
	p.project_id,
	p.name,
	p.token,
	p.endpoints,
	p.auth_type,
	COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
	COALESCE(p.owner_id, '') AS "owner_id",
	CASE
		WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
		ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
	END AS endpoint_count,
	p.created_at,
	p.updated_at,
	ARRAY_TO_JSON(ARRAY_AGG(DISTINCT CASE WHEN e.id IS NOT NULL THEN cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb) END)) AS endpoints_metadata
	FROM convoy.portal_links p
	LEFT JOIN convoy.portal_links_endpoints pe
		ON p.id = pe.portal_link_id
	LEFT JOIN convoy.endpoints e
		ON e.id = pe.endpoint_id
	WHERE p.deleted_at IS NULL`

	fetchPortalLinksByOwnerID = `
		SELECT
		p.id,
		p.project_id,
		p.name,
		p.token,
		p.endpoints,
		p.auth_type,
		COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
		COALESCE(p.owner_id, '') AS "owner_id",
		CASE
			WHEN p.owner_id != '' THEN (SELECT count(id) FROM convoy.endpoints WHERE owner_id = p.owner_id)
			ELSE (SELECT count(portal_link_id) FROM convoy.portal_links_endpoints WHERE portal_link_id = p.id)
		END AS endpoint_count,
		p.created_at,
		p.updated_at,
		ARRAY_TO_JSON(ARRAY_AGG(DISTINCT CASE WHEN e.id IS NOT NULL THEN cast(JSON_BUILD_OBJECT('uid', e.id, 'name', e.name, 'project_id', e.project_id, 'url', e.url, 'secrets', e.secrets) as jsonb) END)) AS endpoints_metadata
		FROM convoy.portal_links p
		LEFT JOIN convoy.portal_links_endpoints pe
			ON p.id = pe.portal_link_id
		LEFT JOIN convoy.endpoints e
			ON e.id = pe.endpoint_id
		WHERE p.owner_id = $1 AND p.deleted_at IS NULL
		GROUP BY p.id;
	`

	baseFetchPortalLinksPagedForward = `
	%s
	%s
	AND p.id <= :cursor
	GROUP BY p.id
	ORDER BY p.id DESC
	LIMIT :limit
	`

	baseFetchPortalLinksPagedBackward = `
	WITH portal_links AS (
		%s
		%s
		AND p.id >= :cursor
		GROUP BY p.id
		ORDER BY p.id ASC
		LIMIT :limit
	)

	SELECT * FROM portal_links ORDER BY id DESC
	`

	basePortalLinkFilter = `
	AND (p.project_id = :project_id OR :project_id = '')`

	deletePortalLink = `
	UPDATE convoy.portal_links SET
	deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`
)

type portalLinkRepo struct {
	db database.Database
	km keys.KeyManager
}

func NewPortalLinkRepo(db database.Database) datastore.PortalLinkRepository {
	km, err := keys.Get()
	if err != nil {
		log.Fatal(err)
	}
	return &portalLinkRepo{db: db, km: km}
}

func (p *portalLinkRepo) CreatePortalLink(ctx context.Context, portal *datastore.PortalLink) error {
	tx, err := p.db.GetDB().BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	r, err := tx.ExecContext(ctx, createPortalLink,
		portal.UID,
		portal.ProjectID,
		portal.Name,
		portal.Token,
		portal.Endpoints,
		portal.OwnerID,
		portal.CanManageEndpoint,
		portal.AuthType,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrPortalLinkNotCreated
	}

	if portal.AuthType == datastore.PortalAuthTypeRefreshToken {
		portalAuth, tokenErr := generateToken(portal.UID)
		if tokenErr != nil {
			return tokenErr
		}

		r, tokenErr = tx.ExecContext(ctx, createPortalLinkAuthToken,
			portalAuth.UID,
			portal.UID,
			portalAuth.MaskId,
			portalAuth.Hash,
			portalAuth.Salt,
			portalAuth.ExpiresAt,
		)
		if tokenErr != nil {
			return tokenErr
		}

		rowsAffected, tokenErr = r.RowsAffected()
		if tokenErr != nil {
			return tokenErr
		}

		if rowsAffected < 1 {
			return ErrPortalLinkAuthTokenNotCreated
		}

		portal.AuthKey = portalAuth.AuthKey
	}

	err = p.upsertPortalLinkEndpoint(ctx, tx, portal)
	if err != nil {
		return err
	}

	// Update endpoint owner_ids if migration signaled it's needed
	updateEndpointOwnerID, endpointIDs := migrations.GetUpdateEndpointOwnerID(ctx)
	if updateEndpointOwnerID && len(endpointIDs) > 0 {
		err = p.updateEndpointOwnerIDs(ctx, tx, endpointIDs, portal.OwnerID, portal.ProjectID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p *portalLinkRepo) UpdatePortalLink(ctx context.Context, projectID string, portal *datastore.PortalLink) error {
	tx, err := p.db.GetDB().BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	r, err := tx.ExecContext(ctx, updatePortalLink,
		portal.UID,
		projectID,
		portal.Endpoints,
		portal.OwnerID,
		portal.CanManageEndpoint,
		portal.Name,
		portal.AuthType,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrPortalLinkNotUpdated
	}

	err = p.upsertPortalLinkEndpoint(ctx, tx, portal)
	if err != nil {
		return err
	}

	// Update endpoint owner_ids if migration signaled it's needed
	updateEndpointOwnerID, endpointIDs := migrations.GetUpdateEndpointOwnerID(ctx)
	if updateEndpointOwnerID && len(endpointIDs) > 0 {
		err = p.updateEndpointOwnerIDs(ctx, tx, endpointIDs, portal.OwnerID, projectID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// updateEndpointOwnerIDs updates endpoint owner_ids for the given endpoint IDs within the transaction
func (p *portalLinkRepo) updateEndpointOwnerIDs(ctx context.Context, tx *sqlx.Tx, endpointIDs []string, portalOwnerID, projectID string) error {
	endpointRepo := NewEndpointRepo(p.db)
	for _, endpointID := range endpointIDs {
		endpoint, err := endpointRepo.FindEndpointByID(ctx, endpointID, projectID)
		if err != nil {
			return fmt.Errorf("failed to find endpoint %s: %w", endpointID, err)
		}

		// If endpoint's owner_id is blank, set it to portal link's owner_id
		if util.IsStringEmpty(endpoint.OwnerID) {
			endpoint.OwnerID = portalOwnerID
			// Update endpoint owner_id using the transaction
			err = p.updateEndpointOwnerID(ctx, tx, endpoint, projectID)
			if err != nil {
				return fmt.Errorf("failed to update endpoint %s owner_id: %w", endpointID, err)
			}
		} else if endpoint.OwnerID != portalOwnerID {
			// If endpoint's owner_id is not blank and doesn't match, throw error
			return fmt.Errorf("endpoint %s already has owner_id %s, cannot assign to portal link with owner_id %s", endpointID, endpoint.OwnerID, portalOwnerID)
		}
	}
	return nil
}

// updateEndpointOwnerID updates the endpoint's owner_id using the provided transaction
func (p *portalLinkRepo) updateEndpointOwnerID(ctx context.Context, tx *sqlx.Tx, endpoint *datastore.Endpoint, projectID string) error {
	r, err := tx.ExecContext(ctx, updateEndpointOwnerID, endpoint.UID, projectID, endpoint.OwnerID)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return fmt.Errorf("endpoint %s not found or not updated", endpoint.UID)
	}

	return nil
}

func (p *portalLinkRepo) FindPortalLinkByID(ctx context.Context, projectID string, portalLinkId string) (*datastore.PortalLink, error) {
	portalLink := datastore.PortalLink{}
	err := p.db.GetDB().QueryRowxContext(ctx, fetchPortalLinkById, portalLinkId, projectID).StructScan(&portalLink)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrPortalLinkNotFound
		}
		return nil, err
	}

	if portalLink.AuthType == datastore.PortalAuthTypeStaticToken {
		return &portalLink, nil
	}

	authToken, err := generateToken(portalLinkId)
	if err != nil {
		return nil, err
	}

	// create auth token
	r, err := p.db.GetDB().ExecContext(ctx, createPortalLinkAuthToken,
		authToken.UID,
		portalLinkId,
		authToken.MaskId,
		authToken.Hash,
		authToken.Salt,
		authToken.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowsAffected < 1 {
		return nil, ErrPortalLinkAuthTokenNotCreated
	}

	portalLink.AuthKey = authToken.AuthKey

	return &portalLink, nil
}

func (p *portalLinkRepo) FindPortalLinkByOwnerID(ctx context.Context, projectID string, ownerID string) (*datastore.PortalLink, error) {
	portalLink := &datastore.PortalLink{}
	err := p.db.GetDB().QueryRowxContext(ctx, fetchPortalLinkByOwnerID, ownerID, projectID).StructScan(portalLink)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrPortalLinkNotFound
		}
		return nil, err
	}

	return portalLink, nil
}

func (p *portalLinkRepo) FindPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error) {
	portalLink := &datastore.PortalLink{}
	err := p.db.GetDB().QueryRowxContext(ctx, fetchPortalLinkByToken, token).StructScan(portalLink)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrPortalLinkNotFound
		}
		return nil, err
	}

	return portalLink, nil
}

func (p *portalLinkRepo) LoadPortalLinksPaged(ctx context.Context, projectID string, filter *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	var err error
	var args []interface{}
	var query, filterQuery string

	if !util.IsStringEmpty(filter.EndpointID) {
		filter.EndpointIDs = append(filter.EndpointIDs, filter.EndpointID)
	}

	arg := map[string]interface{}{
		"project_id":   projectID,
		"endpoint_ids": filter.EndpointIDs,
		"limit":        pageable.Limit(),
		"cursor":       pageable.Cursor(),
	}

	if pageable.Direction == datastore.Next {
		query = baseFetchPortalLinksPagedForward
	} else {
		query = baseFetchPortalLinksPagedBackward
	}

	filterQuery = basePortalLinkFilter
	if len(filter.EndpointIDs) > 0 {
		filterQuery += ` AND pe.endpoint_id IN (:endpoint_ids)`
	}

	query = fmt.Sprintf(query, fetchPortalLinksPaginated, filterQuery)
	query, args, err = sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = p.db.GetReadDB().Rebind(query)

	rows, err := p.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	var portalLinks []datastore.PortalLink

	for rows.Next() {
		var link datastore.PortalLink

		err = rows.StructScan(&link)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		portalLinks = append(portalLinks, link)
	}

	var count datastore.PrevRowCount
	if len(portalLinks) > 0 {
		var countQuery string
		var qargs []interface{}
		first := portalLinks[0]
		qarg := arg
		qarg["cursor"] = first.UID

		cq := fmt.Sprintf(countPrevPortalLinks, filterQuery)
		countQuery, qargs, err = sqlx.Named(cq, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery, qargs, err = sqlx.In(countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = p.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err := p.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
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

	ids := make([]string, len(portalLinks))
	for i := range portalLinks {
		ids[i] = portalLinks[i].UID
	}

	if len(portalLinks) > pageable.PerPage {
		portalLinks = portalLinks[:len(portalLinks)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	if len(portalLinks) > 0 {
		var authTokens []datastore.PortalToken
		for i := range portalLinks {
			if portalLinks[i].AuthType == datastore.PortalAuthTypeStaticToken {
				continue
			}

			authToken, getTokenErr := generateToken(portalLinks[i].UID)
			if getTokenErr != nil {
				return nil, datastore.PaginationData{}, getTokenErr
			}
			authTokens = append(authTokens, *authToken)
		}

		if len(authTokens) > 0 {
			res, resultErr := p.db.GetDB().NamedExecContext(ctx, bulkWritePortalAuthTokens, authTokens)
			if resultErr != nil {
				log.WithError(resultErr).Error("failed to bulk write portal auth tokens")
				return nil, datastore.PaginationData{}, resultErr
			}

			rowsAffected, resultErr := res.RowsAffected()
			if resultErr != nil {
				return nil, datastore.PaginationData{}, resultErr
			}

			if rowsAffected != int64(len(authTokens)) {
				return nil, datastore.PaginationData{}, errors.New("failed to bulk write portal auth tokens")
			}

			for i := range portalLinks {
				for j := range authTokens {
					if portalLinks[i].UID == authTokens[j].PortalLinkID {
						portalLinks[i].AuthKey = authTokens[j].AuthKey
						portalLinks[i].TokenMaskId = authTokens[j].MaskId
						portalLinks[i].TokenHash = authTokens[j].Hash
						portalLinks[i].TokenSalt = authTokens[j].Salt
						portalLinks[i].TokenExpiresAt = authTokens[j].ExpiresAt
					}
				}
			}
		}
	}

	return portalLinks, *pagination, nil
}

func (p *portalLinkRepo) RevokePortalLink(ctx context.Context, projectID string, id string) error {
	r, err := p.db.GetDB().ExecContext(ctx, deletePortalLink, id, projectID)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrPortalLinkNotDeleted
	}

	return nil
}

func (p *portalLinkRepo) FindPortalLinksByOwnerID(ctx context.Context, ownerID string) ([]datastore.PortalLink, error) {
	var portalLinks []datastore.PortalLink
	err := p.db.GetDB().SelectContext(ctx, &portalLinks, fetchPortalLinksByOwnerID, ownerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrPortalLinkNotFound
		}
		return nil, err
	}

	return portalLinks, nil
}

func (p *portalLinkRepo) FindPortalLinkByMaskId(ctx context.Context, maskId string) (*datastore.PortalLink, error) {
	portalLink := &datastore.PortalLink{}
	err := p.db.GetDB().QueryRowxContext(ctx, fetchPortalLinkByMaskId, maskId).StructScan(portalLink)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrPortalLinkNotFound
		}
		return nil, err
	}

	return portalLink, nil
}

func (p *portalLinkRepo) RefreshPortalLinkAuthToken(ctx context.Context, projectID string, portalLinkId string) (*datastore.PortalLink, error) {
	portalLink := &datastore.PortalLink{}
	err := p.db.GetDB().QueryRowxContext(ctx, fetchPortalLinkById, portalLinkId, projectID).StructScan(portalLink)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrPortalLinkNotFound
		}
		return nil, err
	}

	maskId, key := generateAuthKey()
	salt, err := util.GenerateSecret()
	if err != nil {
		return nil, err
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	portalLink.AuthKey = key
	portalLink.TokenSalt = salt
	portalLink.TokenMaskId = maskId
	portalLink.TokenHash = encodedKey
	portalLink.TokenExpiresAt = null.NewTime(time.Now().Add(time.Hour), true)

	err = p.UpdatePortalLink(ctx, projectID, portalLink)
	if err != nil {
		return nil, err
	}

	return portalLink, nil
}

func (p *portalLinkRepo) upsertPortalLinkEndpoint(ctx context.Context, tx *sqlx.Tx, portal *datastore.PortalLink) error {
	var ids []interface{}

	if len(portal.Endpoints) > 0 {
		for _, endpointID := range portal.Endpoints {
			ids = append(ids, &PortalLinkEndpoint{PortalLinkID: portal.UID, EndpointID: endpointID})
		}
	} else if !util.IsStringEmpty(portal.OwnerID) {
		key, err := p.km.GetCurrentKeyFromCache()
		if err != nil {
			return err
		}
		rows, err := p.db.GetDB().QueryxContext(ctx, fetchEndpointsByOwnerId, key, portal.ProjectID, portal.OwnerID)
		if err != nil {
			return err
		}
		defer closeWithError(rows)

		for rows.Next() {
			var endpoint datastore.Endpoint
			err := rows.StructScan(&endpoint)
			if err != nil {
				return err
			}

			ids = append(ids, &PortalLinkEndpoint{PortalLinkID: portal.UID, EndpointID: endpoint.UID})
		}

		if len(ids) == 0 {
			return nil
		}
	} else {
		return errors.New("owner_id or endpoints must be present")
	}

	_, err := tx.ExecContext(ctx, deletePortalLinkEndpoints, portal.UID, nil)
	if err != nil {
		return err
	}

	_, err = tx.NamedExecContext(ctx, createPortalLinkEndpoints, ids)
	if err != nil {
		return err
	}

	return nil
}

type PortalLinkEndpoint struct {
	PortalLinkID string `db:"portal_link_id"`
	EndpointID   string `db:"endpoint_id"`
}

type PortalLinkPaginated struct {
	Count    int `db:"count"`
	Endpoint struct {
		UID          string `db:"id"`
		Title        string `db:"title"`
		ProjectID    string `db:"project_id"`
		SupportEmail string `db:"support_email"`
		TargetUrl    string `db:"target_url"`
	} `db:"endpoint"`
	datastore.PortalLink
}

func generateToken(portalLinkId string) (*datastore.PortalToken, error) {
	maskId, key := generateAuthKey()
	salt, err := util.GenerateSecret()
	if err != nil {
		return nil, err
	}

	dk := pbkdf2.Key([]byte(key), []byte(salt), 4096, 32, sha256.New)
	encodedKey := base64.URLEncoding.EncodeToString(dk)

	portalToken := &datastore.PortalToken{
		UID:          ulid.Make().String(),
		PortalLinkID: portalLinkId,
		MaskId:       maskId,
		Hash:         encodedKey,
		Salt:         salt,
		AuthKey:      key,
		ExpiresAt:    null.NewTime(time.Now().Add(time.Hour), true),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return portalToken, nil
}

func generateAuthKey() (string, string) {
	mask := uniuri.NewLen(16)
	key := uniuri.NewLen(64)

	var builder strings.Builder

	builder.WriteString(util.PortalAuthTokenPrefix)
	builder.WriteString(util.Separator)
	builder.WriteString(mask)
	builder.WriteString(util.Separator)
	builder.WriteString(key)

	return mask, builder.String()
}
