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
	ErrOrganizationNotCreated = errors.New("organization could not be created")
	ErrOrganizationNotUpdated = errors.New("organization could not be updated")
	ErrOrganizationNotDeleted = errors.New("organization could not be deleted")
)

const (
	createOrganization = `
	INSERT INTO convoy.organisations (id, name, owner_id, custom_domain, assigned_domain)
	VALUES ($1, $2, $3, $4, $5);
	`

	fetchOrganisation = `
	SELECT * FROM convoy.organisations
	WHERE %s = $1 AND deleted_at IS NULL;
	`

	fetchOrganisationsPaged = `
	SELECT * FROM convoy.organisations WHERE deleted_at IS NULL
	`

	updateOrganizationById = `
	UPDATE convoy.organisations SET
	name = $2,
 	custom_domain = $3,
	assigned_domain = $4,
	updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteOrganisation = `
	UPDATE convoy.organisations SET
	deleted_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	baseFetchOrganizationsPagedForward = `
	%s 
	AND id <= :cursor 
	GROUP BY id
	ORDER BY id DESC 
	LIMIT :limit
	`

	baseFetchOrganizationsPagedBackward = `
	WITH organizations AS (  
		%s 
		AND id >= :cursor 
		GROUP BY id
		ORDER BY id ASC
		LIMIT :limit
	)

	SELECT * FROM organizations ORDER BY id DESC
	`

	countPrevOrganizations = `
	SELECT count(distinct(id)) as count
	FROM convoy.organisations
	WHERE deleted_at IS NULL
	AND id > :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT 1`
)

type orgRepo struct {
	db *sqlx.DB
}

func NewOrgRepo(db database.Database) datastore.OrganisationRepository {
	return &orgRepo{db: db.GetDB()}
}

func (o *orgRepo) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	result, err := o.db.ExecContext(ctx, createOrganization, org.UID, org.Name, org.OwnerID, org.CustomDomain, org.AssignedDomain)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganizationNotCreated
	}

	return nil
}

func (o *orgRepo) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	var query string
	if pageable.Direction == datastore.Next {
		query = baseFetchOrganizationsPagedForward
	} else {
		query = baseFetchOrganizationsPagedBackward
	}

	query = fmt.Sprintf(query, fetchOrganisationsPaged)

	arg := map[string]interface{}{
		"limit":  pageable.Limit(),
		"cursor": pageable.Cursor(),
	}

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = o.db.Rebind(query)

	rows, err := o.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer rows.Close()

	organizations := make([]datastore.Organisation, 0)
	for rows.Next() {
		var org datastore.Organisation

		err = rows.StructScan(&org)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		organizations = append(organizations, org)
	}

	var count datastore.PrevRowCount
	if len(organizations) > 0 {
		var countQuery string
		var qargs []interface{}

		arg["cursor"] = organizations[0].UID

		countQuery, qargs, err = sqlx.Named(countPrevOrganizations, arg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = o.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := o.db.QueryxContext(ctx, countQuery, qargs...)
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

	ids := make([]string, len(organizations))
	for i := range organizations {
		ids[i] = organizations[i].UID
	}

	if len(organizations) > pageable.PerPage {
		organizations = organizations[:len(organizations)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return organizations, *pagination, nil
}

func (o *orgRepo) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	result, err := o.db.ExecContext(ctx, updateOrganizationById, org.UID, org.Name, org.CustomDomain, org.AssignedDomain)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganizationNotUpdated
	}

	return nil
}

func (o *orgRepo) DeleteOrganisation(ctx context.Context, uid string) error {
	result, err := o.db.Exec(deleteOrganisation, uid)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganizationNotDeleted
	}

	return nil
}

func (o *orgRepo) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	org := &datastore.Organisation{}
	err := o.db.QueryRowxContext(ctx, fmt.Sprintf(fetchOrganisation, "id"), id).StructScan(org)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgNotFound
		}
		return nil, err
	}

	return org, nil
}

func (o *orgRepo) FetchOrganisationByAssignedDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	org := &datastore.Organisation{}
	err := o.db.QueryRowxContext(ctx, fmt.Sprintf(fetchOrganisation, "assigned_domain"), domain).StructScan(org)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgNotFound
		}
		return nil, err
	}

	return org, nil
}

func (o *orgRepo) FetchOrganisationByCustomDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	org := &datastore.Organisation{}
	err := o.db.QueryRowxContext(ctx, fmt.Sprintf(fetchOrganisation, "custom_domain"), domain).StructScan(org)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgNotFound
		}
		return nil, err
	}

	return org, nil
}
