package badger

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
)

type orgMemberRepo struct{}

func (o orgMemberRepo) FetchOrganisationMemberByUserID(ctx context.Context, userID, orgID string) (*datastore.OrganisationMember, error) {
	return nil, nil
}

func NewOrgMemberRepo(db *badgerhold.Store) datastore.OrganisationMemberRepository {
	return orgMemberRepo{}
}

func (o orgMemberRepo) LoadOrganisationMembersPaged(ctx context.Context, organisationID string, pageable datastore.Pageable) ([]datastore.OrganisationMember, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}

func (o orgMemberRepo) CreateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	return nil
}

func (o orgMemberRepo) UpdateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	return nil
}

func (o orgMemberRepo) DeleteOrganisationMember(ctx context.Context, memberID, orgID string) error {
	return nil
}

func (o orgMemberRepo) FetchOrganisationMemberByID(ctx context.Context, orgID, memberID string) (*datastore.OrganisationMember, error) {
	return nil, nil
}
