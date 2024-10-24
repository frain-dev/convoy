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
	ErrOrganisationNotCreated = errors.New("organisation could not be created")
	ErrOrganisationNotUpdated = errors.New("organisation could not be updated")
	ErrOrganisationNotDeleted = errors.New("organisation could not be deleted")
)

const (
	createOrganisation = `
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

	updateOrganisationById = `
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

	baseFetchOrganisationsPagedForward = `
	%s
	AND id <= :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT :limit
	`

	baseFetchOrganisationsPagedBackward = `
	WITH organisations AS (
		%s
		AND id >= :cursor
		GROUP BY id
		ORDER BY id ASC
		LIMIT :limit
	)

	SELECT * FROM organisations ORDER BY id DESC
	`

	countPrevOrganisations = `
	SELECT COUNT(DISTINCT(id)) AS count
	FROM convoy.organisations
	WHERE deleted_at IS NULL
	AND id > :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT 1`

	countOrganisations = `
	SELECT COUNT(*) AS count
	FROM convoy.organisations
	WHERE deleted_at IS NULL`
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
	result, err := o.db.ExecContext(ctx, createOrganisation, org.UID, org.Name, org.OwnerID, org.CustomDomain, org.AssignedDomain)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganisationNotCreated
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
		query = baseFetchOrganisationsPagedForward
	} else {
		query = baseFetchOrganisationsPagedBackward
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

		countQuery, qargs, err = sqlx.Named(countPrevOrganisations, arg)
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

func (o *orgRepo) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	result, err := o.db.ExecContext(ctx, updateOrganisationById, org.UID, org.Name, org.CustomDomain, org.AssignedDomain)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganisationNotUpdated
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
		return ErrOrganisationNotDeleted
	}

	orgCacheKey := convoy.OrganisationCacheKey.Get(uid).String()
	err = o.cache.Delete(ctx, orgCacheKey)
	if err != nil {
		return err
	}

	return nil
}

func (o *orgRepo) CountOrganisations(ctx context.Context) (int64, error) {
	var count int64
	err := o.db.GetContext(ctx, &count, countOrganisations)
	if err != nil {
		return 0, err
	}

	return count, nil
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
