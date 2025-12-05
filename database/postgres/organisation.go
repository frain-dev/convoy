package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
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

	fetchOrganisationsPagedWithSearch = `
	SELECT * FROM convoy.organisations 
	WHERE deleted_at IS NULL 
	AND (LOWER(name) LIKE LOWER(:search) OR LOWER(id) LIKE LOWER(:search))
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

	countPrevOrganizationsWithSearch = `
	SELECT COUNT(DISTINCT(id)) AS count
	FROM convoy.organisations
	WHERE deleted_at IS NULL
	AND (LOWER(name) LIKE LOWER(:search) OR LOWER(id) LIKE LOWER(:search))
	AND id > :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT 1`

	countOrganizations = `
	SELECT COUNT(*) AS count
	FROM convoy.organisations
	WHERE deleted_at IS NULL`

	calculateIngressBytes = `
	SELECT COALESCE(SUM(LENGTH(e.raw)), 0) AS raw_bytes,
	       COALESCE(SUM(OCTET_LENGTH(e.data::text)), 0) AS data_bytes
	FROM convoy.events e
	JOIN convoy.projects p ON p.id = e.project_id
	WHERE p.organisation_id = $1
	  AND e.created_at >= $2 AND e.created_at <= $3
	  AND e.deleted_at IS NULL AND p.deleted_at IS NULL`

	calculateEgressBytes = `
	SELECT COALESCE(SUM(LENGTH(e.raw)), 0) + COALESCE(SUM(OCTET_LENGTH(e.data::text)), 0) AS bytes
	FROM convoy.event_deliveries d
	JOIN convoy.events e ON e.id = d.event_id
	JOIN convoy.projects p ON p.id = e.project_id
	WHERE p.organisation_id = $1
	  AND d.status = 'Success'
	  AND d.created_at >= $2 AND d.created_at <= $3
	  AND p.deleted_at IS NULL`

	countOrgEvents = `
	SELECT COUNT(*)
	FROM convoy.events e
	JOIN convoy.projects p ON p.id = e.project_id
	WHERE p.organisation_id = $1
	  AND e.created_at >= $2 AND e.created_at <= $3
	  AND e.deleted_at IS NULL AND p.deleted_at IS NULL`

	countOrgDeliveries = `
	SELECT COUNT(*)
	FROM convoy.event_deliveries d
	JOIN convoy.events e ON e.id = d.event_id
	JOIN convoy.projects p ON p.id = e.project_id
	WHERE p.organisation_id = $1
	  AND d.status = 'Success'
	  AND d.created_at >= $2 AND d.created_at <= $3
	  AND p.deleted_at IS NULL`
)

type orgRepo struct {
	db database.Database
}

func NewOrgRepo(db database.Database) datastore.OrganisationRepository {
	return &orgRepo{db: db}
}

func (o *orgRepo) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	result, err := o.db.GetDB().ExecContext(ctx, createOrganization, org.UID, org.Name, org.OwnerID, org.CustomDomain, org.AssignedDomain)
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
	return o.LoadOrganisationsPagedWithSearch(ctx, pageable, "")
}

func (o *orgRepo) LoadOrganisationsPagedWithSearch(ctx context.Context, pageable datastore.Pageable, search string) ([]datastore.Organisation, datastore.PaginationData, error) {
	var baseQuery string
	if search != "" {
		baseQuery = fetchOrganisationsPagedWithSearch
	} else {
		baseQuery = fetchOrganisationsPaged
	}

	var query string
	if pageable.Direction == datastore.Next {
		query = baseFetchOrganizationsPagedForward
	} else {
		query = baseFetchOrganizationsPagedBackward
	}

	query = fmt.Sprintf(query, baseQuery)

	arg := map[string]interface{}{
		"limit":  pageable.Limit(),
		"cursor": pageable.Cursor(),
	}

	if search != "" {
		arg["search"] = "%" + search + "%"
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

	organizations := make([]datastore.Organisation, 0)
	for rows.Next() {
		var org datastore.Organisation

		err = rows.StructScan(&org)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		organizations = append(organizations, org)
	}

	var prevRowCount datastore.PrevRowCount
	if len(organizations) > 0 {
		var countQuery string
		var qargs []interface{}

		arg["cursor"] = organizations[0].UID

		if search != "" {
			countQuery, qargs, err = sqlx.Named(countPrevOrganizationsWithSearch, arg)
		} else {
			countQuery, qargs, err = sqlx.Named(countPrevOrganizations, arg)
		}
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = o.db.GetReadDB().Rebind(countQuery)

		// count the row number before the first row
		rows, err = o.db.GetReadDB().QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		defer closeWithError(rows)

		if rows.Next() {
			err = rows.StructScan(&prevRowCount)
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

	pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
	pagination = pagination.Build(pageable, ids)

	return organizations, *pagination, nil
}

func (o *orgRepo) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	result, err := o.db.GetDB().ExecContext(ctx, updateOrganizationById, org.UID, org.Name, org.CustomDomain, org.AssignedDomain)
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
	result, err := o.db.GetDB().Exec(deleteOrganisation, uid)
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

func (o *orgRepo) CountOrganisations(ctx context.Context) (int64, error) {
	var orgCount int64
	err := o.db.GetReadDB().GetContext(ctx, &orgCount, countOrganizations)
	if err != nil {
		return 0, err
	}

	return orgCount, nil
}

func (o *orgRepo) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	org := &datastore.Organisation{}
	err := o.db.GetDB().QueryRowxContext(ctx, fmt.Sprintf("%s AND id = $1", fetchOrganisation), id).StructScan(org)
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
	err := o.db.GetReadDB().QueryRowxContext(ctx, fmt.Sprintf("%s AND assigned_domain = $1", fetchOrganisation), domain).StructScan(org)
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
	err := o.db.GetReadDB().QueryRowxContext(ctx, fmt.Sprintf("%s AND custom_domain = $1", fetchOrganisation), domain).StructScan(org)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgNotFound
		}
		return nil, err
	}

	return org, nil
}

func (o *orgRepo) CalculateUsage(ctx context.Context, orgID string, startTime, endTime time.Time) (*datastore.OrganisationUsage, error) {
	usage := &datastore.OrganisationUsage{
		OrganisationID: orgID,
		CreatedAt:      time.Now(),
	}

	// Calculate ingress bytes
	var orgRawBytes, orgDataBytes sql.NullInt64
	err := o.db.GetReadDB().QueryRowxContext(ctx, calculateIngressBytes, orgID, startTime, endTime).Scan(&orgRawBytes, &orgDataBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate ingress bytes: %w", err)
	}
	usage.Received.Bytes = orgRawBytes.Int64 + orgDataBytes.Int64

	// Calculate egress bytes
	var orgEgressBytes sql.NullInt64
	err = o.db.GetReadDB().QueryRowxContext(ctx, calculateEgressBytes, orgID, startTime, endTime).Scan(&orgEgressBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate egress bytes: %w", err)
	}
	usage.Sent.Bytes = orgEgressBytes.Int64

	// Count events
	err = o.db.GetReadDB().QueryRowxContext(ctx, countOrgEvents, orgID, startTime, endTime).Scan(&usage.Received.Volume)
	if err != nil {
		return nil, fmt.Errorf("failed to count events: %w", err)
	}

	// Count deliveries
	err = o.db.GetReadDB().QueryRowxContext(ctx, countOrgDeliveries, orgID, startTime, endTime).Scan(&usage.Sent.Volume)
	if err != nil {
		return nil, fmt.Errorf("failed to count deliveries: %w", err)
	}

	// Format period as YYYY-MM
	usage.Period = startTime.Format("2006-01")

	return usage, nil
}
