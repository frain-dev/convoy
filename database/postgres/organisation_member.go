package postgres

import (
	"context"
	"errors"
	"math"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

var (
	ErrOrganizationMemberNotCreated = errors.New("organization member could not be created")
	ErrOrganizationMemberNotUpdated = errors.New("organization member could not be updated")
	ErrOrganizationMemberNotDeleted = errors.New("organization member could not be deleted")
)

const (
	createOrgMember = `
	INSERT INTO convoy.organisation_members (organisation_id, user_id, role_type, 
	role_project, role_endpoint) VALUES ($1, $2, $3);
	`

	updateOrgMember = `
	UPDATE convoy.organisation_members
	SET
		role_type = $1,
		role_project = $2,
		role_endpoint = $3,
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteOrgMember = `
	UPDATE convoy.organisation_members SET 
	deleted_at = now()
	WHERE id = $1 AND organisation_id = $2 AND deleted_at IS NULL;
	`

	fetchOrgMemberById = `
	SELECT
		o.id as id,
		o.role_type as "role.type",
		o.role_project as "role.project",
		o.role_endpoint as "role.endpoint",
		u.first_name as "user_metadata.first_name",
		u.last_name as "user_metadata.last_name",
		u.email as "user_metadata.email"
	FROM convoy.organisation_members o
	LEFT JOIN convoy.users u 
		ON o.user_id = u.id
	WHERE o.id = $1 AND o.organisation_id = $2 AND o.deleted_at IS NULL;
	`

	fetchOrgMemberByUserId = `
	SELECT
		o.id as id,
		o.role_type as "role.type",
		o.role_project as "role.project",
		o.role_endpoint as "role.endpoint",
		u.first_name as "user_metadata.first_name",
		u.last_name as "user_metadata.last_name",
		u.email as "user_metadata.email"
	FROM convoy.organisation_members o
	LEFT JOIN convoy.users u 
		ON o.user_id = u.id
	WHERE o.user_id = $1 AND o.organisation_id = $2 AND o.deleted_at IS NULL;
	`

	fetchOrganisationMembersPaginated = `
	SELECT
		o.id as id,
		o.role_type as "role.type",
		o.role_project as "role.project",
		o.role_endpoint as "role.endpoint",
		u.first_name as "user_metadata.first_name",
		u.last_name as "user_metadata.last_name",
		u.email as "user_metadata.email"
	FROM convoy.organisation_members o
	LEFT JOIN convoy.users u 
		ON o.user_id = u.id
	WHERE o.organisation_id = $3 AND o.deleted_at IS NULL
	ORDER BY id LIMIT $1 OFFSET $2
	`

	countOrganisationMembers = `
	SELECT COUNT(id) FROM convoy.organisation_members
	WHERE organisation_id = $1 AND deleted_at IS NULL;
	`

	fetchOrgMemberOrganisations = `
	SELECT o.* FROM convoy.organisation_members m
	JOIN convoy.organisations o ON m.organisation_id = o.id
	WHERE m.user_id = $3 AND o.deleted_at IS NULL AND m.deleted_at IS NULL
	ORDER BY id LIMIT $1 OFFSET $2
	`

	countOrgMemberOrganisations = `
	SELECT COUNT(o.id) FROM convoy.organisation_members m
	JOIN convoy.organisations o ON m.organisation_id = o.id
	WHERE m.user_id = $1 AND o.deleted_at IS NULL AND m.deleted_at IS NULL
	`
)

type orgMemberRepo struct {
	db *sqlx.DB
}

func NewOrgMemberRepo(db *sqlx.DB) datastore.OrganisationMemberRepository {
	return &orgMemberRepo{db: db}
}

func (o *orgMemberRepo) CreateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	r, err := o.db.ExecContext(ctx, createOrgMember,
		member.OrganisationID,
		member.UserID,
		member.Role,
	)
	if err != nil {
		return err
	}

	nRows, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if nRows < 1 {
		return ErrOrganizationMemberNotCreated
	}

	return nil
}

func (o *orgMemberRepo) LoadOrganisationMembersPaged(ctx context.Context, organisationID string, pageable datastore.Pageable) ([]*datastore.OrganisationMember, datastore.PaginationData, error) {
	skip := (pageable.Page - 1) * pageable.PerPage
	rows, err := o.db.QueryxContext(ctx, fetchOrganisationMembersPaginated, pageable.PerPage, skip, organisationID)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var members []*datastore.OrganisationMember
	for rows.Next() {
		var member datastore.OrganisationMember

		err = rows.StructScan(&member)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		members = append(members, &member)
	}

	var count int
	err = o.db.GetContext(ctx, &count, countOrganisationMembers, organisationID)
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

	return members, pagination, rows.Close()
}

func (o *orgMemberRepo) LoadUserOrganisationsPaged(ctx context.Context, userID string, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	skip := (pageable.Page - 1) * pageable.PerPage
	rows, err := o.db.QueryxContext(ctx, fetchOrgMemberOrganisations, pageable.PerPage, skip, userID)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var organisations []datastore.Organisation
	for rows.Next() {
		var org datastore.Organisation

		err = rows.StructScan(&org)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		organisations = append(organisations, org)
	}

	var count int
	err = o.db.GetContext(ctx, &count, countOrgMemberOrganisations, userID)
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

	return organisations, pagination, rows.Close()
}

func (o *orgMemberRepo) UpdateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	r, err := o.db.ExecContext(ctx,
		updateOrgMember,
		member.Role.Type,
		member.Role.Project,
		member.Role.Endpoint,
	)
	if err != nil {
		return err
	}

	nRows, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if nRows < 1 {
		return ErrOrganizationMemberNotUpdated
	}

	return nil
}

func (o *orgMemberRepo) DeleteOrganisationMember(ctx context.Context, uid, orgID string) error {
	r, err := o.db.ExecContext(ctx, deleteOrgMember, uid, orgID)
	if err != nil {
		return err
	}

	nRows, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if nRows < 1 {
		return ErrOrganizationMemberNotDeleted
	}

	return nil
}

func (o *orgMemberRepo) FetchOrganisationMemberByID(ctx context.Context, uid, orgID string) (*datastore.OrganisationMember, error) {
	var member *datastore.OrganisationMember
	err := o.db.QueryRowxContext(ctx, fetchOrgMemberById, uid, orgID).StructScan(&member)
	if err != nil {
		return nil, err
	}

	return member, nil
}

func (o *orgMemberRepo) FetchOrganisationMemberByUserID(ctx context.Context, userID, orgID string) (*datastore.OrganisationMember, error) {
	var member *datastore.OrganisationMember
	err := o.db.QueryRowxContext(ctx, fetchOrgMemberByUserId, userID, orgID).StructScan(&member)
	if err != nil {
		return nil, err
	}

	return member, nil
}
