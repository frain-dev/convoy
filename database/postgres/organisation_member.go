package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/database"
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
	INSERT INTO convoy.organisation_members (id, organisation_id, user_id, role_type, role_project, role_endpoint)
	VALUES ($1, $2, $3, $4, $5, $6);
	`

	updateOrgMember = `
	UPDATE convoy.organisation_members
	SET
		role_type = $2,
		role_project = $3,
		role_endpoint = $4,
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
		o.organisation_id as "organisation_id",
		o.role_type as "role.type",
	    COALESCE(o.role_project,'') as "role.project",
	    COALESCE(o.role_endpoint,'') as "role.endpoint",
		u.id as "user_id",
		u.id as "user_metadata.user_id",
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
		o.organisation_id as "organisation_id",
		o.role_type as "role.type",
	    COALESCE(o.role_project,'') as "role.project",
	    COALESCE(o.role_endpoint,'') as "role.endpoint",
		u.id as "user_id",
		u.id as "user_metadata.user_id",
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
		o.organisation_id as "organisation_id",
		o.role_type as "role.type",
	    COALESCE(o.role_project,'') as "role.project",
	    COALESCE(o.role_endpoint,'') as "role.endpoint",
		u.id as "user_id",
		u.id as "user_metadata.user_id",
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

	fetchUserProjects = `
	SELECT p.id, p.name, p.type, p.retained_events, p.logo_url,
	p.organisation_id, p.project_configuration_id, p.created_at,
	p.updated_at FROM convoy.organisation_members m
	LEFT JOIN convoy.projects p ON p.organisation_id = m.organisation_id
	WHERE m.user_id = $1 AND m.deleted_at IS NULL AND p.deleted_at IS NULL
	`
)

type orgMemberRepo struct {
	db *sqlx.DB
}

func NewOrgMemberRepo(db database.Database) datastore.OrganisationMemberRepository {
	return &orgMemberRepo{db: db.GetDB()}
}

func (o *orgMemberRepo) CreateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	var endpointID *string
	var projectID *string
	if !util.IsStringEmpty(member.Role.Endpoint) {
		endpointID = &member.Role.Endpoint
	}

	if !util.IsStringEmpty(member.Role.Project) {
		projectID = &member.Role.Project
	}

	r, err := o.db.ExecContext(ctx, createOrgMember,
		member.UID,
		member.OrganisationID,
		member.UserID,
		member.Role.Type,
		projectID,
		endpointID,
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
	rows, err := o.db.QueryxContext(ctx, fetchOrganisationMembersPaginated, pageable.Limit(), pageable.Offset(), organisationID)
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

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return members, pagination, nil
}

func (o *orgMemberRepo) LoadUserOrganisationsPaged(ctx context.Context, userID string, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	rows, err := o.db.QueryxContext(ctx, fetchOrgMemberOrganisations, pageable.Limit(), pageable.Offset(), userID)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	organisations := make([]datastore.Organisation, 0)
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

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return organisations, pagination, nil
}

func (o *orgMemberRepo) FindUserProjects(ctx context.Context, userID string) ([]datastore.Project, error) {
	rows, err := o.db.QueryxContext(ctx, fetchUserProjects, userID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var projects []datastore.Project
	for rows.Next() {
		var proj datastore.Project

		err = rows.StructScan(&proj)
		if err != nil {
			return nil, err
		}

		projects = append(projects, proj)
	}

	return projects, nil
}

func (o *orgMemberRepo) UpdateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	var endpointID *string
	var projectID *string
	if !util.IsStringEmpty(member.Role.Endpoint) {
		endpointID = &member.Role.Endpoint
	}

	if !util.IsStringEmpty(member.Role.Project) {
		projectID = &member.Role.Project
	}

	r, err := o.db.ExecContext(ctx,
		updateOrgMember,
		member.UID,
		member.Role.Type,
		projectID,
		endpointID,
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
	member := &datastore.OrganisationMember{}
	err := o.db.QueryRowxContext(ctx, fetchOrgMemberById, uid, orgID).StructScan(member)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgMemberNotFound
		}
		return nil, err
	}

	return member, nil
}

func (o *orgMemberRepo) FetchOrganisationMemberByUserID(ctx context.Context, userID, orgID string) (*datastore.OrganisationMember, error) {
	member := &datastore.OrganisationMember{}
	err := o.db.QueryRowxContext(ctx, fetchOrgMemberByUserId, userID, orgID).StructScan(member)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgMemberNotFound
		}
		return nil, err
	}

	return member, nil
}
