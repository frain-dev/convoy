package badger

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
)

type orgInviteRepo struct{}

func (o orgInviteRepo) LoadOrganisationsInvitesPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.OrganisationInvite, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}

func (o orgInviteRepo) CreateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	return nil
}

func (o orgInviteRepo) UpdateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	return nil
}

func (o orgInviteRepo) DeleteOrganisationInvite(ctx context.Context, uid string) error {
	return nil
}

func (o orgInviteRepo) FetchOrganisationInviteByID(ctx context.Context, uid string) (*datastore.OrganisationInvite, error) {
	return nil, nil
}

func (o orgInviteRepo) FetchOrganisationInviteByTokenAndEmail(ctx context.Context, token, email string) (*datastore.OrganisationInvite, error) {
	return nil, nil
}

func NewOrgInviteRepo(db *badgerhold.Store) datastore.OrganisationInviteRepository {
	return orgInviteRepo{}
}
