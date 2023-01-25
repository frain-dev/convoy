package postgres

import (
	"context"
	"errors"
	"math"

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
	INSERT INTO convoy.organisations (name, owner_id)
	VALUES ($1, $2);
	`

	fetchOrganisation = `
	SELECT * FROM convoy.organisations
	WHERE $1 = $2;
	`

	fetchOrganisationsPaginated = `
	SELECT * FROM convoy.organisations
	ORDER BY id
	LIMIT $1
	OFFSET $2;
	`

	updateOrganizationById = `
	UPDATE convoy.organisations SET
	name = $2,
	owner_id = $3,
	custom_domain = $4,
	assigned_domain = $5
	updated_at = now()
	WHERE id = $1;
	`

	deleteOrganisation = `
	UPDATE convoy.organisations SET
	deleted_at = now()
	WHERE id = $1;
	`

	countOrganizations = `
	SELECT COUNT(id) FROM convoy.organisations;
	`
)

type orgRepo struct {
	db *sqlx.DB
}

func NewOrgRepo(db *sqlx.DB) datastore.OrganisationRepository {
	return &orgRepo{db: db}
}

func (o *orgRepo) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	result, err := o.db.Exec(createOrganization, org.Name, org.OwnerID)
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
	skip := (pageable.Page - 1) * pageable.PerPage
	rows, err := o.db.Queryx(fetchOrganisationsPaginated, pageable.PerPage, skip)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var organizations []datastore.Organisation
	for rows.Next() {
		var org datastore.Organisation

		err = rows.StructScan(&org)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		organizations = append(organizations, org)
	}

	var count int
	err = o.db.Get(&count, countOrganizations)
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

	return organizations, pagination, nil
}

func (o *orgRepo) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	result, err := o.db.Exec(updateOrganizationById, org.UID, org.Name, org.OwnerID, org.CustomDomain, org.AssignedDomain)
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
	var org *datastore.Organisation
	err := o.db.QueryRowx(fetchOrganisation, "id", id).StructScan(&org)
	if err != nil {
		return nil, err
	}

	return org, nil
}

func (o *orgRepo) FetchOrganisationByAssignedDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	var org *datastore.Organisation
	err := o.db.QueryRowx(fetchOrganisation, "assigned_domain", domain).StructScan(&org)
	if err != nil {
		return nil, err
	}

	return org, nil
}

func (o *orgRepo) FetchOrganisationByCustomDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
	var org *datastore.Organisation
	err := o.db.QueryRowx(fetchOrganisation, "custom_domain", domain).StructScan(&org)
	if err != nil {
		return nil, err
	}

	return org, nil
}

// getPrevPage returns calculated value for the prev page
func getPrevPage(page int) int {
	if page == 0 {
		return 1
	}

	prev := 0
	if page-1 <= 0 {
		prev = page
	} else {
		prev = page - 1
	}

	return prev
}
