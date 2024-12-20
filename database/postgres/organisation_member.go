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
		updated_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteOrgMember = `
	UPDATE convoy.organisation_members SET
	deleted_at = NOW()
	WHERE id = $1 AND organisation_id = $2 AND deleted_at IS NULL;
	`

	fetchOrgMemberById = `
	SELECT
		o.id AS id,
		o.organisation_id AS "organisation_id",
		o.role_type AS "role.type",
	    COALESCE(o.role_project,'') AS "role.project",
	    COALESCE(o.role_endpoint,'') AS "role.endpoint",
		u.id AS "user_id",
		u.id AS "user_metadata.user_id",
		u.first_name AS "user_metadata.first_name",
		u.last_name AS "user_metadata.last_name",
		u.email AS "user_metadata.email"
	FROM convoy.organisation_members o
	LEFT JOIN convoy.users u
		ON o.user_id = u.id
	WHERE o.id = $1 AND o.organisation_id = $2 AND o.deleted_at IS NULL;
	`

	fetchOrgMemberByUserId = `
	SELECT
		o.id AS id,
		o.organisation_id AS "organisation_id",
		o.role_type AS "role.type",
	    COALESCE(o.role_project,'') AS "role.project",
	    COALESCE(o.role_endpoint,'') AS "role.endpoint",
		u.id AS "user_id",
		u.id AS "user_metadata.user_id",
		u.first_name AS "user_metadata.first_name",
		u.last_name AS "user_metadata.last_name",
		u.email AS "user_metadata.email"
	FROM convoy.organisation_members o
	LEFT JOIN convoy.users u
		ON o.user_id = u.id
	WHERE o.user_id = $1 AND o.organisation_id = $2 AND o.deleted_at IS NULL;
	`

	fetchOrganisationMembersPaged = `
	SELECT
		o.id AS id,
		o.organisation_id AS "organisation_id",
		o.role_type AS "role.type",
	    COALESCE(o.role_project,'') AS "role.project",
	    COALESCE(o.role_endpoint,'') AS "role.endpoint",
		u.id AS "user_id",
		u.id AS "user_metadata.user_id",
		u.first_name AS "user_metadata.first_name",
		u.last_name AS "user_metadata.last_name",
		u.email AS "user_metadata.email"
	FROM convoy.organisation_members o
	LEFT JOIN convoy.users u ON o.user_id = u.id
	WHERE o.organisation_id = :organisation_id
	AND (o.user_id = :user_id OR :user_id = '')
	AND o.deleted_at IS NULL
	`

	baseFetchOrganisationMembersPagedForward = `
	%s
	AND o.id <= :cursor
	GROUP BY o.id, u.id
	ORDER BY o.id DESC
	LIMIT :limit
	`

	baseFetchOrganisationMembersPagedBackward = `
	WITH organisation_members AS (
		%s
		AND o.id >= :cursor
		GROUP BY o.id, u.id
		ORDER BY o.id ASC
		LIMIT :limit
	)

	SELECT * FROM organisation_members ORDER BY id DESC
	`

	countPrevOrganisationMembers = `
	SELECT COUNT(DISTINCT(o.id)) AS count
	FROM convoy.organisation_members o
	LEFT JOIN convoy.users u ON o.user_id = u.id
	WHERE o.organisation_id = :organisation_id
	AND o.deleted_at IS NULL
	AND o.id > :cursor
	GROUP BY o.id, u.id
	ORDER BY o.id DESC
	LIMIT 1`

	fetchOrgMemberOrganisations = `
	SELECT o.* FROM convoy.organisation_members m
	JOIN convoy.organisations o ON m.organisation_id = o.id
	WHERE m.user_id = :user_id
	AND o.deleted_at IS NULL
	AND m.deleted_at IS NULL
	`

	baseFetchUserOrganisationsPagedForward = `
	%s
	AND o.id <= :cursor
	GROUP BY o.id, m.id
	ORDER BY o.id DESC
	LIMIT :limit
	`

	baseFetchUserOrganisationsPagedBackward = `
	WITH user_organisations AS (
		%s
		AND o.id >= :cursor
		GROUP BY o.id, m.id
		ORDER BY o.id ASC
		LIMIT :limit
	)

	SELECT * FROM user_organisations ORDER BY id DESC
	`

	countPrevUserOrgs = `
	SELECT COUNT(DISTINCT(o.id)) AS count
	FROM convoy.organisation_members m
	JOIN convoy.organisations o ON m.organisation_id = o.id
	WHERE m.user_id = :user_id
	AND o.deleted_at IS NULL
	AND m.deleted_at IS NULL
	AND o.id > :cursor
	GROUP BY o.id, m.id
	ORDER BY o.id DESC
	LIMIT 1`

	fetchUserProjects = `
	SELECT p.id, p.name, p.type, p.retained_events, p.logo_url,
	p.organisation_id, p.project_configuration_id, p.created_at,
	p.updated_at FROM convoy.organisation_members m
	RIGHT JOIN convoy.projects p ON p.organisation_id = m.organisation_id
	WHERE m.user_id = $1 AND m.deleted_at IS NULL AND p.deleted_at IS NULL
	`
)

type orgMemberRepo struct {
	db database.Database
}

func NewOrgMemberRepo(db database.Database) datastore.OrganisationMemberRepository {
	return &orgMemberRepo{db: db}
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

	r, err := o.db.GetDB().ExecContext(ctx, createOrgMember,
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

func (o *orgMemberRepo) LoadOrganisationMembersPaged(ctx context.Context, organisationID, userID string, pageable datastore.Pageable) ([]*datastore.OrganisationMember, datastore.PaginationData, error) {
	var query string
	if pageable.Direction == datastore.Next {
		query = baseFetchOrganisationMembersPagedForward
	} else {
		query = baseFetchOrganisationMembersPagedBackward
	}

	query = fmt.Sprintf(query, fetchOrganisationMembersPaged)

	arg := map[string]interface{}{
		"limit":           pageable.Limit(),
		"cursor":          pageable.Cursor(),
		"organisation_id": organisationID,
		"user_id":         userID,
	}

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = o.db.GetReadDB().Rebind(query)

	rows, err := o.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	var members []*datastore.OrganisationMember
	for rows.Next() {
		var member datastore.OrganisationMember

		err = rows.StructScan(&member)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		members = append(members, &member)
	}

	var count datastore.PrevRowCount
	if len(members) > 0 {
		var countQuery string
		var qargs []interface{}

		arg["cursor"] = members[0].UID

		countQuery, qargs, err = sqlx.Named(countPrevOrganisationMembers, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = o.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err := o.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
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

	ids := make([]string, len(members))
	for i := range members {
		ids[i] = members[i].UID
	}

	if len(members) > pageable.PerPage {
		members = members[:len(members)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return members, *pagination, nil
}

func (o *orgMemberRepo) LoadUserOrganisationsPaged(ctx context.Context, userID string, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	var query string
	if pageable.Direction == datastore.Next {
		query = baseFetchUserOrganisationsPagedForward
	} else {
		query = baseFetchUserOrganisationsPagedBackward
	}

	query = fmt.Sprintf(query, fetchOrgMemberOrganisations)

	arg := map[string]interface{}{
		"limit":   pageable.Limit(),
		"cursor":  pageable.Cursor(),
		"user_id": userID,
	}

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = o.db.GetReadDB().Rebind(query)

	rows, err := o.db.GetReadDB().QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	organisations := make([]datastore.Organisation, 0)
	for rows.Next() {
		var org datastore.Organisation

		err = rows.StructScan(&org)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		organisations = append(organisations, org)
	}

	var count datastore.PrevRowCount
	if len(organisations) > 0 {
		var countQuery string
		var qargs []interface{}

		arg["cursor"] = organisations[0].UID

		countQuery, qargs, err = sqlx.Named(countPrevUserOrgs, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = o.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err := o.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
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

	ids := make([]string, len(organisations))
	for i := range organisations {
		ids[i] = organisations[i].UID
	}

	if len(organisations) > pageable.PerPage {
		organisations = organisations[:len(organisations)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return organisations, *pagination, nil
}

func (o *orgMemberRepo) FindUserProjects(ctx context.Context, userID string) ([]datastore.Project, error) {
	rows, err := o.db.GetReadDB().QueryxContext(ctx, fetchUserProjects, userID)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

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

	r, err := o.db.GetDB().ExecContext(ctx,
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
	r, err := o.db.GetDB().ExecContext(ctx, deleteOrgMember, uid, orgID)
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
	err := o.db.GetDB().QueryRowxContext(ctx, fetchOrgMemberById, uid, orgID).StructScan(member)
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
	err := o.db.GetDB().QueryRowxContext(ctx, fetchOrgMemberByUserId, userID, orgID).StructScan(member)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgMemberNotFound
		}
		return nil, err
	}

	return member, nil
}
