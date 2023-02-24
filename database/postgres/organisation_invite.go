package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/frain-dev/convoy/database"
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
	INSERT INTO convoy.organisation_invites (id, organisation_id, invitee_email, token, role_type, role_project, role_endpoint, status, expires_at) 
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);
	`

	updateOrganisationInvite = `
	UPDATE convoy.organisation_invites
	SET
		role_type = $2,
		role_project = $3,
		role_endpoint = $4,
		status = $5,
		expires_at = $6,
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	fetchOrganisationInviteById = `
	SELECT
	    id, 
		organisation_id,
		invitee_email,
		token,
		status,
		role_type as "role.type",
		role_project as "role.project",
		role_endpoint as "role.endpoint"
	FROM convoy.organisation_invites
	WHERE id = $1 AND deleted_at IS NULL;
	`

	fetchOrganisationInviteByToken = `
	SELECT
	    id,
		organisation_id,
		invitee_email,
		token,
		status,
		role_type as "role.type",
		role_project as "role.project",
		role_endpoint as "role.endpoint"
	FROM convoy.organisation_invites
	WHERE token = $1 AND deleted_at IS NULL;
	`

	fetchOrganisationInvitesPaginated = `
	SELECT
	    id, 
		organisation_id,
		invitee_email,
		status,
		role_type as "role.type",
		role_project as "role.project",
		role_endpoint as "role.endpoint"
	FROM convoy.organisation_invites
	WHERE organisation_id = $3 AND status = $4 AND deleted_at IS NULL
	ORDER BY id LIMIT $1 OFFSET $2
	`

	countOrganisationInvites = `
	SELECT COUNT(id) FROM convoy.organisation_invites
	WHERE organisation_id = $1 AND deleted_at IS NULL;
	`

	deleteOrganisationInvite = `
	UPDATE convoy.organisation_invites SET 
	deleted_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`
)

type orgInviteRepo struct {
	db *sqlx.DB
}

func NewOrgInviteRepo(db database.Database) datastore.OrganisationInviteRepository {
	return &orgInviteRepo{db: db.GetDB()}
}

func (i *orgInviteRepo) CreateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	r, err := i.db.ExecContext(ctx, createOrganisationInvite,
		iv.UID,
		iv.OrganisationID,
		iv.InviteeEmail,
		iv.Token,
		iv.Role.Type,
		iv.Role.Project,
		iv.Role.Endpoint,
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
	rows, err := i.db.QueryxContext(ctx, fetchOrganisationInvitesPaginated, pageable.Limit(), pageable.Offset(), orgID, inviteStatus)
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
	err = i.db.GetContext(ctx, &count, countOrganisationInvites, orgID)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return invites, pagination, rows.Close()
}

func (i *orgInviteRepo) UpdateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	r, err := i.db.ExecContext(ctx,
		updateOrganisationInvite,
		iv.UID,
		iv.Role.Type,
		iv.Role.Project,
		iv.Role.Endpoint,
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
	invite := &datastore.OrganisationInvite{}
	err := i.db.QueryRowxContext(ctx, fetchOrganisationInviteById, id).StructScan(invite)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgInviteNotFound
		}
		return nil, err
	}

	return invite, nil
}

func (i *orgInviteRepo) FetchOrganisationInviteByToken(ctx context.Context, token string) (*datastore.OrganisationInvite, error) {
	invite := &datastore.OrganisationInvite{}
	err := i.db.QueryRowxContext(ctx, fetchOrganisationInviteByToken, token).StructScan(invite)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgInviteNotFound
		}
		return nil, err
	}

	return invite, nil
}
