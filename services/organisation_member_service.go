package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
)

type OrganisationMemberService struct {
	orgMemberRepo datastore.OrganisationMemberRepository
}

func NewOrganisationMemberService(orgMemberRepo datastore.OrganisationMemberRepository) *OrganisationMemberService {
	return &OrganisationMemberService{orgMemberRepo: orgMemberRepo}
}

func (om *OrganisationMemberService) CreateOrganisationMember(ctx context.Context, org *datastore.Organisation, user *datastore.User, role *auth.Role) (*datastore.OrganisationMember, error) {
	err := role.Validate("organisation member")
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to validate organisation member role update")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	member := &datastore.OrganisationMember{
		UID:            ulid.Make().String(),
		OrganisationID: org.UID,
		UserID:         user.UID,
		Role:           *role,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = om.orgMemberRepo.CreateOrganisationMember(ctx, member)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create organisation member")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member"))
	}

	return member, nil
}

func (om *OrganisationMemberService) UpdateOrganisationMember(ctx context.Context, organisationMember *datastore.OrganisationMember, role *auth.Role) (*datastore.OrganisationMember, error) {
	err := role.Validate("organisation member")
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to validate organisation member role update")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	fmt.Println(role)

	organisationMember.UpdatedAt = time.Now()
	organisationMember.Role = *role
	err = om.orgMemberRepo.UpdateOrganisationMember(ctx, organisationMember)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to to update organisation member")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update organisation member"))
	}

	return organisationMember, nil
}

func (om *OrganisationMemberService) FindOrganisationMemberByID(ctx context.Context, org *datastore.Organisation, id string) (*datastore.OrganisationMember, error) {
	member, err := om.orgMemberRepo.FetchOrganisationMemberByID(ctx, id, org.UID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find organisation member by id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find organisation member by id"))
	}
	return member, err
}

func (om *OrganisationMemberService) LoadOrganisationMembersPaged(ctx context.Context, org *datastore.Organisation, pageable datastore.Pageable) ([]*datastore.OrganisationMember, datastore.PaginationData, error) {
	organisationMembers, paginationData, err := om.orgMemberRepo.LoadOrganisationMembersPaged(ctx, org.UID, pageable)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch organisation members")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, errors.New("failed to load organisation members"))
	}

	return organisationMembers, paginationData, nil
}

func (om *OrganisationMemberService) DeleteOrganisationMember(ctx context.Context, memberID string, org *datastore.Organisation) error {
	member, err := om.orgMemberRepo.FetchOrganisationMemberByID(ctx, memberID, org.UID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to find organisation member by id")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to find organisation member by id"))
	}

	if member.UserID == org.OwnerID {
		return util.NewServiceError(http.StatusForbidden, errors.New("cannot deactivate organisation owner"))
	}

	err = om.orgMemberRepo.DeleteOrganisationMember(ctx, memberID, org.UID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to delete organisation member")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to delete organisation member"))
	}
	return err
}
