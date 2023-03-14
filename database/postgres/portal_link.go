package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrPortalLinkNotCreated = errors.New("portal link could not be created")
	ErrPortalLinkNotUpdated = errors.New("portal link could not be updated")
	ErrPortalLinkNotDeleted = errors.New("portal link could not be deleted")
)

const (
	createPortalLink = `
	INSERT INTO convoy.portal_links (id, project_id, name, token, endpoints, created_at, updated_at)
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
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deletePortalLinkEndpoints = `
	DELETE from convoy.portal_links_endpoints WHERE portal_link_id = $1
	`

	fetchPortalLinkById = `
	SELECT * FROM convoy.portal_links
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	fetchPortalLinkByToken = `
	SELECT * FROM convoy.portal_links
	WHERE token = $1 AND deleted_at IS NULL;
	`

	basePortalLinksCount = `
	WITH table_count AS (
		SELECT count(distinct(p.id)) as count
		FROM convoy.portal_links p
		LEFT JOIN convoy.portal_links_endpoints pe ON p.id = pe.portal_link_id
		LEFT JOIN convoy.endpoints e ON e.id = pe.endpoint_id
		WHERE p.deleted_at IS NULL
		%s
	)
	`

	fetchPortalLinksPaginated = `
	SELECT table_count.count as count, p.id, p.project_id, p.name, p.token, p.endpoints, p.created_at, p.updated_at,
	e.id AS "endpoint.id", e.title AS "endpoint.title",
	e.project_id AS "endpoint.project_id", e.support_email AS "endpoint.support_email",
	e.target_url AS "endpoint.target_url"
	FROM table_count, convoy.portal_links p
	LEFT JOIN convoy.portal_links_endpoints pe ON p.id = pe.portal_link_id
	LEFT JOIN convoy.endpoints e ON e.id = pe.endpoint_id
	WHERE p.deleted_at IS NULL
	%s
	ORDER BY p.id LIMIT :limit OFFSET :offset
	`

	basePortalLinkFilter = `AND (p.project_id = :project_id OR :project_id = '') AND (pe.endpoint_id = :endpoint_id OR :endpoint_id = '')`

	deletePortalLink = `
	UPDATE convoy.portal_links SET
	deleted_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`
)

type portalLinkRepo struct {
	db *sqlx.DB
}

func NewPortalLinkRepo(db database.Database) datastore.PortalLinkRepository {
	return &portalLinkRepo{db: db.GetDB()}
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
		portal.CreatedAt,
		portal.UpdatedAt,
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

	var ids []interface{}
	if len(portal.Endpoints) > 0 {
		for _, endpointID := range portal.Endpoints {
			ids = append(ids, &PortalLinkEndpoint{PortalLinkID: portal.UID, EndpointID: endpointID})
		}

		_, err = tx.NamedExecContext(ctx, createPortalLinkEndpoints, ids)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p *portalLinkRepo) UpdatePortalLink(ctx context.Context, projectID string, portal *datastore.PortalLink) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	r, err := tx.ExecContext(ctx, updatePortalLink, portal.UID, portal.Name, portal.Endpoints)
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

	var ids []interface{}
	if len(portal.Endpoints) > 0 {
		for _, endpointID := range portal.Endpoints {
			ids = append(ids, &PortalLinkEndpoint{PortalLinkID: portal.UID, EndpointID: endpointID})
		}

		_, err = tx.ExecContext(ctx, deletePortalLinkEndpoints, portal.UID)
		if err != nil {
			return err
		}

		_, err = tx.NamedExecContext(ctx, createPortalLinkEndpoints, ids)
		if err != nil {
			return err
		}
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

func (p *portalLinkRepo) LoadPortalLinksPaged(ctx context.Context, projectID string, f *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	var err error
	var args []interface{}
	var query string

	arg := map[string]interface{}{
		"project_id":   projectID,
		"endpoint_id":  f.EndpointID,
		"endpoint_ids": f.EndpointIDs,
		"limit":        pageable.Limit(),
		"offset":       pageable.Offset(),
	}

	if len(f.EndpointIDs) > 0 {
		filterQuery := `AND pe.endpoint_id IN (:endpoint_ids) ` + basePortalLinkFilter
		query = fmt.Sprintf(basePortalLinksCount, filterQuery) + fmt.Sprintf(fetchPortalLinksPaginated, filterQuery)
		query, args, err = sqlx.Named(query, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		query, args, err = sqlx.In(query, args...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		query = p.db.Rebind(query)
	} else {
		query = fmt.Sprintf(basePortalLinksCount, basePortalLinkFilter) + fmt.Sprintf(fetchPortalLinksPaginated, basePortalLinkFilter)
		query, args, err = sqlx.Named(query, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		query = p.db.Rebind(query)
	}

	rows, err := p.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	defer rows.Close()

	var portalLinks []datastore.PortalLink
	portalLinksMap := make(map[string]*datastore.PortalLink, 0)
	var count int

	for rows.Next() {
		var link PortalLinkPaginated
		err = rows.StructScan(&link)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		portal := link.PortalLink
		endpoint := link.Endpoint

		record, exists := portalLinksMap[portal.UID]
		if exists {
			record.EndpointsMetadata = append(record.EndpointsMetadata, datastore.Endpoint{
				UID:          endpoint.UID,
				Title:        endpoint.Title,
				ProjectID:    endpoint.ProjectID,
				SupportEmail: endpoint.SupportEmail,
				TargetURL:    endpoint.TargetUrl,
			})
		} else {
			portal := link.PortalLink
			portal.EndpointsMetadata = append(portal.EndpointsMetadata, datastore.Endpoint{
				UID:          endpoint.UID,
				Title:        endpoint.Title,
				ProjectID:    endpoint.ProjectID,
				SupportEmail: endpoint.SupportEmail,
				TargetURL:    endpoint.TargetUrl,
			})

			portalLinksMap[portal.UID] = &portal
		}
		count = link.Count
	}

	for _, portalLink := range portalLinksMap {
		portalLinks = append(portalLinks, *portalLink)
	}

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return portalLinks, pagination, rows.Close()
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
