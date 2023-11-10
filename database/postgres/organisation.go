package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/config"

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
	WHERE deleted_at IS NULL
	`

	fetchOrganisationsPaged = `
	SELECT * FROM convoy.organisations WHERE deleted_at IS NULL
	`

	updateOrganizationById = `
	UPDATE convoy.organisations SET
	name = $2,
 	custom_domain = $3,
	assigned_domain = $4,
	updated_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteOrganisation = `
	UPDATE convoy.organisations SET
	deleted_at = NOW()
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
	SELECT COUNT(DISTINCT(id)) AS count
	FROM convoy.organisations
	WHERE deleted_at IS NULL
	AND id > :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT 1`
)

type orgRepo struct {
	db    *sqlx.DB
	cache cache.Cache
}

func NewOrgRepo(db database.Database, ca cache.Cache) datastore.OrganisationRepository {
	if ca == nil {
		ca = ncache.NewNoopCache()
	}
	return &orgRepo{db: db.GetDB(), cache: ca}
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

	orCacheKey := convoy.OrganisationCacheKey.Get(org.UID).String()
	err = o.cache.Set(ctx, orCacheKey, org, config.DefaultCacheTTL)
	if err != nil {
		return err
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
	defer closeWithError(rows)

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
		defer closeWithError(rows)

		if rows.Next() {
			err = rows.StructScan(&count)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
		}
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

	orCacheKey := convoy.OrganisationCacheKey.Get(org.UID).String()
	err = o.cache.Set(ctx, orCacheKey, org, config.DefaultCacheTTL)
	if err != nil {
		return err
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

	orgCacheKey := convoy.OrganisationCacheKey.Get(uid).String()
	err = o.cache.Delete(ctx, orgCacheKey)
	if err != nil {
		return err
	}

	return nil
}

func (o *orgRepo) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	fromCache, err := o.readFromCache(ctx, id, func() (*datastore.Organisation, error) {
		org := &datastore.Organisation{}
		err := o.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND id = $1", fetchOrganisation), id).StructScan(org)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrOrgNotFound
			}
			return nil, err
		}

		return org, nil
	})

	if err != nil {
		return nil, err
	}

	return fromCache, nil
}

func (o *orgRepo) FetchOrganisationByAssignedDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	fromCache, err := o.readFromCache(ctx, domain, func() (*datastore.Organisation, error) {
		org := &datastore.Organisation{}
		err := o.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND assigned_domain = $1", fetchOrganisation), domain).StructScan(org)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrOrgNotFound
			}
			return nil, err
		}

		return org, nil
	})

	if err != nil {
		return nil, err
	}

	return fromCache, nil
}

func (o *orgRepo) FetchOrganisationByCustomDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	fromCache, err := o.readFromCache(ctx, domain, func() (*datastore.Organisation, error) {
		org := &datastore.Organisation{}
		err := o.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND custom_domain = $1", fetchOrganisation), domain).StructScan(org)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, datastore.ErrOrgNotFound
			}
			return nil, err
		}

		return org, nil
	})

	if err != nil {
		return nil, err
	}

	return fromCache, nil
}

func (o *orgRepo) readFromCache(ctx context.Context, key string, readFromDB func() (*datastore.Organisation, error)) (*datastore.Organisation, error) {
	var organisation *datastore.Organisation
	userCacheKey := convoy.OrganisationCacheKey.Get(key).String()
	err := o.cache.Get(ctx, userCacheKey, &organisation)
	if err != nil {
		return nil, err
	}

	if organisation != nil {
		return organisation, err
	}

	fromDB, err := readFromDB()
	if err != nil {
		return nil, err
	}

	err = o.cache.Set(ctx, userCacheKey, fromDB, config.DefaultCacheTTL)
	if err != nil {
		return nil, err
	}

	return fromDB, err
}
