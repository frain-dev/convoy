package postgres

import (
	"context"
	"errors"
	"math"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrOrganizationInviteNotCreated = errors.New("organization invite could not be created")
	ErrOrganizationInviteNotUpdated = errors.New("organization invite could not be updated")
	ErrOrganizationInviteNotDeleted = errors.New("organization invite could not be deleted")
)

const (
	createOrganisationInvite = `
	INSERT INTO convoy.organisation_invites (organisation_id, invitee_email, token, role, status, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6);
	`

	updateOrganisationInvite = `
	UPDATE convoy.organisation_invites
	SET
		role = $1,
		status = $2,
		expires_at = $3,
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	fetchOrganisationInviteById = `
	SELECT * FROM convoy.organisation_invites 
	WHERE id = $1 AND deleted_at IS NULL;
	`

	fetchOrganisationInviteByToken = `
	SELECT * FROM convoy.organisation_invites 
	WHERE token = $1 AND AND deleted_at IS NULL;
	`

	fetchOrganisationInvitesPaginated = `
	SELECT * FROM convoy.organisation_invites ORDER BY id LIMIT $1 OFFSET $2
	WHERE organisation_id = $3 AND status = $4 AND deleted_at IS NULL;
	`

	countOrganisationInvites = `
	SELECT COUNT(id) FROM convoy.organisation_invites WHERE deleted_at IS NULL;
	`

	deleteOrganisationInvite = `
	UPDATE convoy.organisation_invites SET 
	deleted_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL;
	`
)

type orgInviteRepo struct {
	db *sqlx.DB
}

func NewOrgInviteRepo(db *sqlx.DB) datastore.OrganisationInviteRepository {
	return &orgInviteRepo{db: db}
}

func (i *orgInviteRepo) CreateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	r, err := i.db.ExecContext(ctx, createOrganisationInvite,
		iv.OrganisationID,
		iv.InviteeEmail,
		iv.Token,
		iv.Role,
		iv.Status,
		iv.ExpiresAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganizationInviteNotCreated
	}

	return nil
}

func (i *orgInviteRepo) LoadOrganisationsInvitesPaged(ctx context.Context, orgID string, inviteStatus datastore.InviteStatus, pageable datastore.Pageable) ([]datastore.OrganisationInvite, datastore.PaginationData, error) {
	skip := (pageable.Page - 1) * pageable.PerPage
	rows, err := i.db.QueryxContext(ctx, fetchOrganisationInvitesPaginated, pageable.PerPage, skip, orgID, inviteStatus)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var invites []datastore.OrganisationInvite
	for rows.Next() {
		var iv datastore.OrganisationInvite

		err = rows.StructScan(&iv)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		invites = append(invites, iv)
	}

	var count int
	err = i.db.GetContext(ctx, &count, countOrganisationInvites)
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

	return invites, pagination, rows.Close()
}

func (i *orgInviteRepo) UpdateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	r, err := i.db.ExecContext(ctx, updateOrganisationInvite, iv.Role, iv.Status, iv.ExpiresAt)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganizationInviteNotUpdated
	}

	return nil
}

func (i *orgInviteRepo) DeleteOrganisationInvite(ctx context.Context, id string) error {
	r, err := i.db.ExecContext(ctx, deleteOrganisationInvite, id)
	if err != nil {
		return err
	}

	rowsAffected, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganizationInviteNotDeleted
	}

	return nil
}

func (i *orgInviteRepo) FetchOrganisationInviteByID(ctx context.Context, id string) (*datastore.OrganisationInvite, error) {
	var invite *datastore.OrganisationInvite
	err := i.db.QueryRowxContext(ctx, fetchOrganisationInviteById, id).StructScan(&invite)
	if err != nil {
		return nil, err
	}

	return invite, nil
}

func (i *orgInviteRepo) FetchOrganisationInviteByToken(ctx context.Context, token string) (*datastore.OrganisationInvite, error) {
	var invite *datastore.OrganisationInvite
	err := i.db.QueryRowxContext(ctx, fetchOrganisationInviteByToken, token).StructScan(&invite)
	if err != nil {
		return nil, err
	}

	return invite, nil
}
