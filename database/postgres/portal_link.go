package postgres

import (
	"context"
	"errors"
	"math"

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
	INSERT INTO convoy.portal_links (project_id, name, token, endpoints)
	VALUES ($1, $2, $3, $4);
	`

	updatePortalLink = `
	UPDATE convoy.portal_links
	SET
		name = $2
		endpoints = $3
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	fetchPortalLinkById = `
	SELECT * FROM convoy.portal_links 
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`

	fetchPortalLinkByToken = `
	SELECT * FROM convoy.portal_links 
	WHERE token = $1 AND AND deleted_at IS NULL;
	`

	fetchPortalLinksPaginated = `
	SELECT * FROM convoy.portal_links
	ORDER BY id
	LIMIT $1
	OFFSET $2
	WHERE project_id = $3 AND deleted_at IS NULL;
	`

	countPortalLinks = `
	SELECT COUNT(id) FROM convoy.portal_links WHERE deleted_at IS NULL;
	`

	deletePortalLink = `
	UPDATE convoy.portal_links SET 
	deleted_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`
)

type portalLinkRepo struct {
	db *sqlx.DB
}

func NewPortalLinkRepo(db *sqlx.DB) datastore.PortalLinkRepository {
	return &portalLinkRepo{db: db}
}

func (p *portalLinkRepo) CreatePortalLink(ctx context.Context, portal *datastore.PortalLink) error {
	r, err := p.db.ExecContext(ctx, createPortalLink,
		portal.ProjectID,
		portal.Name,
		portal.Token,
		portal.Endpoints,
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

	return nil
}

func (p *portalLinkRepo) UpdatePortalLink(ctx context.Context, projectID string, portal *datastore.PortalLink) error {
	r, err := p.db.ExecContext(ctx, updatePortalLink, portal.Name, portal.Endpoints)
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

	return nil
}

func (p *portalLinkRepo) FindPortalLinkByID(ctx context.Context, projectID string, id string) (*datastore.PortalLink, error) {
	var portalLink *datastore.PortalLink
	err := p.db.QueryRowx(fetchPortalLinkById, id, projectID).StructScan(&portalLink)
	if err != nil {
		return nil, err
	}

	return portalLink, nil
}

func (p *portalLinkRepo) FindPortalLinkByToken(ctx context.Context, token string) (*datastore.PortalLink, error) {
	var portalLink *datastore.PortalLink
	err := p.db.QueryRowx(fetchPortalLinkByToken, token).StructScan(&portalLink)
	if err != nil {
		return nil, err
	}

	return portalLink, nil
}

func (p *portalLinkRepo) LoadPortalLinksPaged(ctx context.Context, projectID string, f *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.PortalLink, datastore.PaginationData, error) {
	skip := (pageable.Page - 1) * pageable.PerPage
	rows, err := p.db.QueryxContext(ctx, fetchPortalLinksPaginated, pageable.PerPage, skip)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var portalLinks []datastore.PortalLink
	for rows.Next() {
		var link datastore.PortalLink

		err = rows.StructScan(&link)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		portalLinks = append(portalLinks, link)
	}

	var count int
	err = p.db.Get(&count, countPortalLinks)
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

	return portalLinks, pagination, nil
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
