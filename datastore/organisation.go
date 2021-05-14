package datastore

import (
	"github.com/hookcamp/hookcamp"
)

func (db *database) LoadOrganisations() ([]hookcamp.Organisation, error) {

	var orgs = make([]hookcamp.Organisation, 0)

	return orgs, db.inner.Find(&orgs).Error
}
