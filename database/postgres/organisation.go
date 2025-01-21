package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/instance"
	"github.com/frain-dev/convoy/pkg/log"
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
	fetchOrganisationByProjectId = `
	SELECT * FROM convoy.organisations
	WHERE deleted_at IS NULL AND id=(SELECT organisation_id FROM convoy.projects WHERE id=$1)
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

	countOrganizations = `
	SELECT COUNT(*) AS count
	FROM convoy.organisations
	WHERE deleted_at IS NULL`
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

		countQuery, qargs, err = sqlx.Named(countPrevOrganizations, arg)
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

	err = EnrichOrganisationWithOverrides(ctx, o.db.GetReadDB(), org, instance.GetEncryptionPassphrase())
	if err != nil {
		return nil, err
	}

	return org, nil
}

func EnrichOrganisationWithOverrides(ctx context.Context, db *sqlx.DB, org *datastore.Organisation, encryptionKey string) error {
	query := `
		SELECT key, pgp_sym_decrypt(value_cipher::bytea, CONCAT($1::text, '-', id)) AS value
		FROM convoy.instance_overrides
		WHERE scope_type = 'organisation' AND scope_id = $2;
	`

	rows, err := db.QueryContext(ctx, query, encryptionKey, org.UID)
	if err != nil {
		return fmt.Errorf("failed to fetch overrides: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Error("failed to close rows: ", err)
		}
	}(rows)

	if org.Config == nil {
		org.Config = &datastore.InstanceConfig{}
	}
	if org.Config.ProjectConfig == nil {
		org.Config.ProjectConfig = &datastore.ProjectInstanceConfig{}
	}

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return fmt.Errorf("failed to scan override: %w", err)
		}

		switch key {
		case instance.KeyStaticIP:
			var v instance.Boolean
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				return fmt.Errorf("failed to unmarshal static_ip: %w", err)
			}
			org.Config.StaticIP = &v.Value

		case instance.KeyEnterpriseSSO:
			var v instance.Boolean
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				return fmt.Errorf("failed to unmarshal enterprise_sso: %w", err)
			}
			org.Config.EnterpriseSSO = &v.Value

		case instance.KeyRetentionPolicy:
			var v config.RetentionPolicyConfiguration
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				return fmt.Errorf("failed to unmarshal retention_policy: %w", err)
			}
			org.Config.ProjectConfig.RetentionPolicy = &v

		case instance.KeyInstanceIngestRate:
			var v instance.IngestRate
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				return fmt.Errorf("failed to unmarshal ingest_rate_limit: %w", err)
			}
			org.Config.ProjectConfig.IngestRateLimit = &v.Value
		}
	}

	// Check for errors during iteration
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating over overrides: %w", err)
	}

	return nil
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

func (o *orgRepo) FetchOrganisationByProjectID(ctx context.Context, id string) (*datastore.Organisation, error) {
	org := &datastore.Organisation{}
	err := o.db.GetDB().QueryRowxContext(ctx, fetchOrganisationByProjectId, id).StructScan(org)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrOrgNotFound
		}
		return nil, err
	}

	return org, nil
}
