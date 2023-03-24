package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/frain-dev/convoy/util"

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
	    COALESCE(role_project,'') as "role.project",
	    COALESCE(role_endpoint,'') as "role.endpoint",
	    created_at, updated_at, expires_at
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
	    COALESCE(role_project,'') as "role.project",
	    COALESCE(role_endpoint,'') as "role.endpoint",
	    created_at, updated_at, expires_at
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
	    COALESCE(role_project,'') as "role.project",
	    COALESCE(role_endpoint,'') as "role.endpoint",
	    created_at, updated_at, expires_at
	FROM convoy.organisation_invites
	WHERE organisation_id = :org_id
	AND status = :status
	AND deleted_at IS NULL
	`

	baseFetchInvitesPagedForward = `
	%s
	AND id <= :cursor 
	GROUP BY id
	ORDER BY id DESC 
	LIMIT :limit
	`

	baseFetchInvitesPagedBackward = `
	WITH organisation_invites AS (
		%s
		AND id >= :cursor 
		GROUP BY id
		ORDER BY id ASC 
		LIMIT :limit
	)

	SELECT * FROM organisation_invites ORDER BY id DESC
	`

	countPrevOrganisationInvites = `
	SELECT count(distinct(id)) as count
	FROM convoy.organisation_invites
	WHERE organisation_id = :org_id
	AND deleted_at IS NULL
	AND id > :cursor 
	GROUP BY id
	ORDER BY id DESC
	LIMIT 1
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
	var endpointID *string
	var projectID *string
	if !util.IsStringEmpty(iv.Role.Endpoint) {
		endpointID = &iv.Role.Endpoint
	}

	if !util.IsStringEmpty(iv.Role.Project) {
		projectID = &iv.Role.Project
	}

	r, err := i.db.ExecContext(ctx, createOrganisationInvite,
		iv.UID,
		iv.OrganisationID,
		iv.InviteeEmail,
		iv.Token,
		iv.Role.Type,
		projectID,
		endpointID,
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
	arg := map[string]interface{}{
		"org_id": orgID,
		"status": inviteStatus,
		"limit":  pageable.Limit(),
		"cursor": pageable.Cursor(),
	}

	var query string
	if pageable.Direction == datastore.Next {
		query = baseFetchInvitesPagedForward
	} else {
		query = baseFetchInvitesPagedBackward
	}

	query = fmt.Sprintf(query, fetchOrganisationInvitesPaginated)

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = i.db.Rebind(query)

	rows, err := i.db.QueryxContext(ctx, query, args...)
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

	var count datastore.PrevRowCount
	if len(invites) > 0 {
		var countQuery string
		var qargs []interface{}
		first := invites[0]
		qarg := arg
		qarg["cursor"] = first.UID

		countQuery, qargs, err = sqlx.Named(countPrevOrganisationInvites, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = i.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := i.db.QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		if rows.Next() {
			err = rows.StructScan(&count)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
		}
		rows.Close()
	}

	ids := make([]string, len(invites))
	for i := range invites {
		ids[i] = invites[i].UID
	}

	if len(invites) > pageable.PerPage {
		invites = invites[:len(invites)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return invites, *pagination, nil
}

func (i *orgInviteRepo) UpdateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	var endpointID *string
	var projectID *string
	if !util.IsStringEmpty(iv.Role.Endpoint) {
		endpointID = &iv.Role.Endpoint
	}

	if !util.IsStringEmpty(iv.Role.Project) {
		projectID = &iv.Role.Project
	}

	r, err := i.db.ExecContext(ctx,
		updateOrganisationInvite,
		iv.UID,
		iv.Role.Type,
		projectID,
		endpointID,
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
