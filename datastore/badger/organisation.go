package badger

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
	"math"
)

type orgRepo struct {
	db *badgerhold.Store
}

func NewOrgRepo(db *badgerhold.Store) *orgRepo {
	return &orgRepo{db: db}
}

func (o *orgRepo) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	var organisations = make([]datastore.Organisation, 0)

	page := pageable.Page
	perPage := pageable.PerPage
	data := datastore.PaginationData{}

	if pageable.Page < 1 {
		page = 1
	}

	if pageable.PerPage < 1 {
		perPage = 10
	}

	prevPage := page - 1
	lowerBound := perPage * prevPage

	qry := (&badgerhold.Query{}).Skip(lowerBound).Limit(perPage).SortBy("CreatedAt")
	if pageable.Sort == -1 {
		qry.Reverse()
	}

	err := o.db.Find(&organisations, qry)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	total, err := o.db.Count(&datastore.Organisation{}, nil)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	data.TotalPage = int64(math.Ceil(float64(total) / float64(perPage)))
	data.Total = int64(total)
	data.PerPage = int64(perPage)
	data.Next = int64(page + 1)
	data.Page = int64(page)
	data.Prev = int64(prevPage)

	return organisations, data, err
}

func (o *orgRepo) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	return o.db.Upsert(org.UID, org)
}

func (o *orgRepo) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
	return o.db.Update(org.UID, org)
}

func (o *orgRepo) DeleteOrganisation(ctx context.Context, uid string) error {
	return o.db.Delete(uid, &datastore.Organisation{})
}

func (o *orgRepo) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	var organisation *datastore.Organisation

	err := o.db.Get(id, &organisation)
	if err != nil && errors.Is(err, badgerhold.ErrNotFound) {
		return organisation, datastore.ErrOrgNotFound
	}

	return organisation, err
}
