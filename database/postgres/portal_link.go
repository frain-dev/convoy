package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/cache"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/jmoiron/sqlx"
)

var (
	ErrPortalLinkNotCreated = errors.New("portal link could not be created")
	ErrPortalLinkNotUpdated = errors.New("portal link could not be updated")
	ErrPortalLinkNotDeleted = errors.New("portal link could not be deleted")
)

const (
	createPortalLink = `
	INSERT INTO convoy.portal_links (id, project_id, name, token, endpoints, owner_id, can_manage_endpoint)
	VALUES ($1, $2, $3, $4, $5, $6, $7);
	`

	createPortalLinkEndpoints = `
	INSERT INTO convoy.portal_links_endpoints (portal_link_id, endpoint_id) VALUES (:portal_link_id, :endpoint_id)
	`

	updatePortalLink = `
	UPDATE convoy.portal_links
	SET
		name = $2,
		endpoints = $3,
		owner_id = $4,
		can_manage_endpoint = $5,
		updated_at = NOW()
	WHERE id = $1 AND project_id = $6 AND deleted_at IS NULL;
	`

	deletePortalLinkEndpoints = `
	DELETE FROM convoy.portal_links_endpoints
	WHERE portal_link_id = $1 OR endpoint_id = $2
	`

	fetchPortalLinkById = `
	SELECT
	p.id,
	p.project_id,
	p.name,
	p.token,
	p.endpoints,
	COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
	COALESCE(p.owner_id, '') AS "owner_id",
	p.created_at,
	p.updated_at,
    ARRAY_TO_JSON(ARRAY_AGG(DISTINCT cast(JSON_BUILD_OBJECT('uid', e.id, 'title', e.title, 'project_id', e.project_id, 'target_url', e.target_url) as jsonb))) AS endpoints_metadata
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
	COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
	COALESCE(p.owner_id, '') AS "owner_id",
	p.created_at,
	p.updated_at,
	ARRAY_TO_JSON(ARRAY_AGG(DISTINCT cast(JSON_BUILD_OBJECT('uid', e.id, 'title', e.title, 'project_id', e.project_id, 'target_url', e.target_url) as jsonb))) AS endpoints_metadata
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
	COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
	COALESCE(p.owner_id, '') AS "owner_id",
	p.created_at,
	p.updated_at,
	ARRAY_TO_JSON(ARRAY_AGG(DISTINCT cast(JSON_BUILD_OBJECT('uid', e.id, 'title', e.title, 'project_id', e.project_id, 'target_url', e.target_url, 'secrets', e.secrets) as jsonb)))  AS endpoints_metadata
	FROM convoy.portal_links p
	LEFT JOIN convoy.portal_links_endpoints pe
		ON p.id = pe.portal_link_id
	LEFT JOIN convoy.endpoints e
		ON e.id = pe.endpoint_id
	WHERE p.token = $1 AND p.deleted_at IS NULL
	GROUP BY p.id;
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
        COALESCE(p.can_manage_endpoint, FALSE) AS "can_manage_endpoint",
        COALESCE(p.owner_id, '') AS "owner_id",
        p.created_at,
        p.updated_at,
    ARRAY_TO_JSON(ARRAY_AGG(DISTINCT cast(JSON_BUILD_OBJECT('uid', e.id, 'title', e.title, 'project_id', e.project_id, 'target_url', e.target_url) as jsonb))) AS endpoints_metadata
    FROM convoy.portal_links p
        LEFT JOIN convoy.portal_links_endpoints pe ON p.id = pe.portal_link_id
        LEFT JOIN convoy.endpoints_portal_links ep ON p.owner_id = ep.owner_id
        LEFT JOIN convoy.endpoints e ON e.id = pe.endpoint_id OR e.id = ep.endpoint_id
    WHERE p.deleted_at IS NULL`

	baseFetchPortalLinksPagedForward = `
	%s
	%s
	AND p.id <= :cursor
	GROUP BY p.id, p.project_id, p.name, p.token, p.endpoints, p.can_manage_endpoint, p.owner_id, p.created_at, p.updated_at
	ORDER BY p.id DESC
	LIMIT :limit
	`

	baseFetchPortalLinksPagedBackward = `
	WITH portal_links AS (
		%s
		%s
		AND p.id >= :cursor
		GROUP BY p.id, p.project_id, p.name, p.token, p.endpoints, p.can_manage_endpoint, p.owner_id, p.created_at, p.updated_at
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
	db    *sqlx.DB
	cache cache.Cache
}

func NewPortalLinkRepo(db database.Database, cache cache.Cache) datastore.PortalLinkRepository {
	return &portalLinkRepo{db: db.GetDB(), cache: cache}
}

func (p *portalLinkRepo) CreatePortalLink(ctx context.Context, portal *datastore.PortalLink) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	r, err := tx.ExecContext(ctx, createPortalLink,
		portal.UID,
		portal.ProjectID,
		portal.Name,
		portal.Token,
		portal.Endpoints,
		portal.OwnerID,
		portal.CanManageEndpoint,
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

	err = p.upsertPortalLinkEndpoint(ctx, tx, portal)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (p *portalLinkRepo) UpdatePortalLink(ctx context.Context, projectID string, portal *datastore.PortalLink) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	r, err := tx.ExecContext(ctx, updatePortalLink,
		portal.UID,
		portal.Name,
		portal.Endpoints,
		portal.OwnerID,
		portal.CanManageEndpoint,
		projectID,
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

	return tx.Commit()
}

func (p *portalLinkRepo) FindPortalLinkByID(ctx context.Context, projectID string, id string) (*datastore.PortalLink, error) {
	portalLink := &datastore.PortalLink{}
	err := p.db.QueryRowxContext(ctx, fetchPortalLinkById, id, projectID).StructScan(portalLink)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrPortalLinkNotFound
		}
		return nil, err
	}

	return portalLink, nil
}

func (p *portalLinkRepo) FindPortalLinkByOwnerID(ctx context.Context, projectID string, ownerID string) (*datastore.PortalLink, error) {
	portalLink := &datastore.PortalLink{}
	err := p.db.QueryRowxContext(ctx, fetchPortalLinkByOwnerID, ownerID, projectID).StructScan(portalLink)
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
	err := p.db.QueryRowxContext(ctx, fetchPortalLinkByToken, token).StructScan(portalLink)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrPortalLinkNotFound
		}
		return nil, err
	}

	return portalLink, nil
}

func (p *portalLinkRepo) LoadPortalLinksPaged(ctx context.Context, projectID string, filter *datastore.PortalLinkFilter, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	var err error
	var args []interface{}
	var query, filterQuery string

	arg := map[string]interface{}{
		"project_id":   projectID,
		"endpoint_ids": filter.EndpointIDs,
		"owner_id":     filter.OwnerID,
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

	query = p.db.Rebind(query)

	rows, err := p.db.QueryxContext(ctx, query, args...)
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

		countQuery = p.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := p.db.QueryxContext(ctx, countQuery, qargs...)
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

	return portalLinks, *pagination, nil
}

func (p *portalLinkRepo) RevokePortalLink(ctx context.Context, projectID string, id string) error {
	r, err := p.db.ExecContext(ctx, deletePortalLink, id, projectID)
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

func (p *portalLinkRepo) upsertPortalLinkEndpoint(ctx context.Context, tx *sqlx.Tx, portal *datastore.PortalLink) error {
	var ids []interface{}

	if len(portal.Endpoints) > 0 {
		for _, endpointID := range portal.Endpoints {
			ids = append(ids, &PortalLinkEndpoint{PortalLinkID: portal.UID, EndpointID: endpointID})
		}
	} else if !util.IsStringEmpty(portal.OwnerID) {
		rows, err := p.db.QueryxContext(ctx, fetchEndpointsByOwnerId, portal.ProjectID, portal.OwnerID)
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
