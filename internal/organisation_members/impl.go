package organisation_members

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/organisation_members/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// Service implements the OrganisationMemberRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier  // SQLc-generated interface
	db     *pgxpool.Pool // Connection pool
}

// Ensure Service implements datastore.OrganisationMemberRepository at compile time
var _ datastore.OrganisationMemberRepository = (*Service)(nil)

// New creates a new Organisation Member Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// rowToOrganisationMember converts any SQLc-generated row struct to datastore.OrganisationMember
func rowToOrganisationMember(row interface{}) *datastore.OrganisationMember {
	var (
		id, organisationID, userID                                                         string
		roleType, roleProject, roleEndpoint                                                string
		userMetadataUserID, userMetadataFirstName, userMetadataLastName, userMetadataEmail string
		createdAt, updatedAt                                                               pgtype.Timestamptz
	)

	switch r := row.(type) {
	case repo.FetchOrganisationMemberByIDRow:
		id, organisationID, userID = r.ID, r.OrganisationID, r.UserID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		userMetadataUserID = r.UserMetadataUserID.String
		userMetadataFirstName = r.UserMetadataFirstName.String
		userMetadataLastName = r.UserMetadataLastName.String
		userMetadataEmail = r.UserMetadataEmail.String
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	case repo.FetchOrganisationMemberByUserIDRow:
		id, organisationID, userID = r.ID, r.OrganisationID, r.UserID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		userMetadataUserID = r.UserMetadataUserID.String
		userMetadataFirstName = r.UserMetadataFirstName.String
		userMetadataLastName = r.UserMetadataLastName.String
		userMetadataEmail = r.UserMetadataEmail.String
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	case repo.FetchInstanceAdminByUserIDRow:
		id, organisationID, userID = r.ID, r.OrganisationID, r.UserID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		userMetadataUserID = r.UserMetadataUserID.String
		userMetadataFirstName = r.UserMetadataFirstName.String
		userMetadataLastName = r.UserMetadataLastName.String
		userMetadataEmail = r.UserMetadataEmail.String
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	case repo.FetchAnyOrganisationAdminByUserIDRow:
		id, organisationID, userID = r.ID, r.OrganisationID, r.UserID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		userMetadataUserID = r.UserMetadataUserID.String
		userMetadataFirstName = r.UserMetadataFirstName.String
		userMetadataLastName = r.UserMetadataLastName.String
		userMetadataEmail = r.UserMetadataEmail.String
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	case repo.FetchOrganisationMembersPaginatedRow:
		id, organisationID, userID = r.ID, r.OrganisationID, r.UserID
		roleType, roleProject, roleEndpoint = r.RoleType, r.RoleProject, r.RoleEndpoint
		userMetadataUserID = r.UserMetadataUserID.String
		userMetadataFirstName = r.UserMetadataFirstName.String
		userMetadataLastName = r.UserMetadataLastName.String
		userMetadataEmail = r.UserMetadataEmail.String
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
	default:
		return &datastore.OrganisationMember{}
	}

	return &datastore.OrganisationMember{
		UID:            id,
		OrganisationID: organisationID,
		UserID:         userID,
		Role:           common.ParamsToRole(roleType, roleProject, roleEndpoint),
		UserMetadata: datastore.UserMetadata{
			UserID:    userMetadataUserID,
			FirstName: userMetadataFirstName,
			LastName:  userMetadataLastName,
			Email:     userMetadataEmail,
		},
		CreatedAt: createdAt.Time,
		UpdatedAt: updatedAt.Time,
	}
}

// rowToOrganisation converts FetchUserOrganisationsPaginatedRow to datastore.Organisation
func rowToOrganisation(row repo.FetchUserOrganisationsPaginatedRow) datastore.Organisation {
	return datastore.Organisation{
		UID:            row.ID,
		Name:           row.Name,
		OwnerID:        row.OwnerID,
		CustomDomain:   common.PgTextToNullString(row.CustomDomain),
		AssignedDomain: common.PgTextToNullString(row.AssignedDomain),
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
		DeletedAt:      common.PgTimestamptzToNullTime(row.DeletedAt),
	}
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateOrganisationMember creates a new organisation member
func (s *Service) CreateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	if member == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("organisation member cannot be nil"))
	}

	roleTypePg, roleProject, roleEndpoint := common.RoleToParams(member.Role)

	err := s.repo.CreateOrganisationMember(ctx, repo.CreateOrganisationMemberParams{
		ID:             member.UID,
		OrganisationID: member.OrganisationID,
		UserID:         member.UserID,
		RoleType:       roleTypePg.String,
		RoleProject:    roleProject,
		RoleEndpoint:   roleEndpoint,
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to create organisation member")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// UpdateOrganisationMember updates an existing organisation member's role
func (s *Service) UpdateOrganisationMember(ctx context.Context, member *datastore.OrganisationMember) error {
	if member == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("organisation member cannot be nil"))
	}

	roleTypePg, roleProject, roleEndpoint := common.RoleToParams(member.Role)

	err := s.repo.UpdateOrganisationMember(ctx, repo.UpdateOrganisationMemberParams{
		ID:           member.UID,
		RoleType:     roleTypePg.String,
		RoleProject:  roleProject,
		RoleEndpoint: roleEndpoint,
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to update organisation member")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// DeleteOrganisationMember soft deletes an organisation member
func (s *Service) DeleteOrganisationMember(ctx context.Context, memberID, orgID string) error {
	err := s.repo.DeleteOrganisationMember(ctx, repo.DeleteOrganisationMemberParams{
		ID:             memberID,
		OrganisationID: orgID,
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to delete organisation member")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// FetchOrganisationMemberByID retrieves an organisation member by ID
func (s *Service) FetchOrganisationMemberByID(ctx context.Context, memberID, organisationID string) (*datastore.OrganisationMember, error) {
	row, err := s.repo.FetchOrganisationMemberByID(ctx, repo.FetchOrganisationMemberByIDParams{
		ID:             memberID,
		OrganisationID: organisationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrOrgMemberNotFound
		}
		s.logger.WithError(err).Error("failed to fetch organisation member by id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	member := rowToOrganisationMember(row)
	return member, nil
}

// FetchOrganisationMemberByUserID retrieves an organisation member by user ID
func (s *Service) FetchOrganisationMemberByUserID(ctx context.Context, userID, organisationID string) (*datastore.OrganisationMember, error) {
	row, err := s.repo.FetchOrganisationMemberByUserID(ctx, repo.FetchOrganisationMemberByUserIDParams{
		UserID:         userID,
		OrganisationID: organisationID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrOrgMemberNotFound
		}
		s.logger.WithError(err).Error("failed to fetch organisation member by user id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	member := rowToOrganisationMember(row)
	return member, nil
}

// FetchInstanceAdminByUserID retrieves an instance admin by user ID
func (s *Service) FetchInstanceAdminByUserID(ctx context.Context, userID string) (*datastore.OrganisationMember, error) {
	row, err := s.repo.FetchInstanceAdminByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrOrgMemberNotFound
		}
		s.logger.WithError(err).Error("failed to fetch instance admin by user id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	member := rowToOrganisationMember(row)
	return member, nil
}

// FetchAnyOrganisationAdminByUserID retrieves any organisation admin by user ID
func (s *Service) FetchAnyOrganisationAdminByUserID(ctx context.Context, userID string) (*datastore.OrganisationMember, error) {
	row, err := s.repo.FetchAnyOrganisationAdminByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrOrgMemberNotFound
		}
		s.logger.WithError(err).Error("failed to fetch organisation admin by user id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	member := rowToOrganisationMember(row)
	return member, nil
}

// CountInstanceAdminUsers counts the number of instance admin users
func (s *Service) CountInstanceAdminUsers(ctx context.Context) (int64, error) {
	count, err := s.repo.CountInstanceAdminUsers(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to count instance admin users")
		return 0, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return count, nil
}

// CountOrganisationAdminUsers counts the number of organisation admin users
func (s *Service) CountOrganisationAdminUsers(ctx context.Context) (int64, error) {
	count, err := s.repo.CountOrganisationAdminUsers(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to count organisation admin users")
		return 0, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return count, nil
}

// HasInstanceAdminAccess checks if a user has instance admin access
func (s *Service) HasInstanceAdminAccess(ctx context.Context, userID string) (bool, error) {
	result, err := s.repo.HasInstanceAdminAccess(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("failed to check instance admin access")
		return false, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return result.Bool, nil
}

// IsFirstInstanceAdmin checks if a user is the first instance admin
func (s *Service) IsFirstInstanceAdmin(ctx context.Context, userID string) (bool, error) {
	isFirst, err := s.repo.IsFirstInstanceAdmin(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("failed to check if user is first instance admin")
		return false, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return isFirst, nil
}

// LoadOrganisationMembersPaged retrieves organisation members with pagination
func (s *Service) LoadOrganisationMembersPaged(ctx context.Context, organisationID, userID string, pageable datastore.Pageable) ([]*datastore.OrganisationMember, datastore.PaginationData, error) {
	// Determine direction for query
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	// Query organisation members with pagination
	rows, err := s.repo.FetchOrganisationMembersPaginated(ctx, repo.FetchOrganisationMembersPaginatedParams{
		Direction:      direction,
		OrganisationID: organisationID,
		UserID:         userID,
		Cursor:         pageable.Cursor(),
		LimitVal:       int64(pageable.Limit()),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to load organisation members paged")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Convert rows to organisation members
	members := make([]*datastore.OrganisationMember, 0, len(rows))
	for _, row := range rows {
		member := rowToOrganisationMember(row)
		members = append(members, member)
	}

	// Build IDs for pagination
	ids := make([]string, len(members))
	for i := range members {
		ids[i] = members[i].UID
	}

	// If we got more results than requested, trim the extra one (used for hasNext detection)
	if len(members) > pageable.PerPage {
		members = members[:len(members)-1]
	}

	// Count previous rows for pagination
	var prevRowCount datastore.PrevRowCount
	if len(members) > 0 {
		first := members[0]
		count, err2 := s.repo.CountPrevOrganisationMembers(ctx, repo.CountPrevOrganisationMembersParams{
			OrganisationID: organisationID,
			Cursor:         first.UID,
		})
		if err2 != nil {
			s.logger.WithError(err2).Error("failed to count prev organisation members")
			return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err2)
		}
		prevRowCount.Count = int(count)
	}

	// Build pagination data
	pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
	pagination = pagination.Build(pageable, ids)

	return members, *pagination, nil
}

// LoadUserOrganisationsPaged retrieves organisations for a user with pagination
func (s *Service) LoadUserOrganisationsPaged(ctx context.Context, userID string, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	// Determine direction for query
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	// Query user organisations with pagination
	rows, err := s.repo.FetchUserOrganisationsPaginated(ctx, repo.FetchUserOrganisationsPaginatedParams{
		Direction: direction,
		UserID:    userID,
		Cursor:    pageable.Cursor(),
		LimitVal:  int64(pageable.Limit()),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to load user organisations paged")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Convert rows to organisations
	organisations := make([]datastore.Organisation, 0, len(rows))
	for _, row := range rows {
		org := rowToOrganisation(row)
		organisations = append(organisations, org)
	}

	// Build IDs for pagination
	ids := make([]string, len(organisations))
	for i := range organisations {
		ids[i] = organisations[i].UID
	}

	// If we got more results than requested, trim the extra one (used for hasNext detection)
	if len(organisations) > pageable.PerPage {
		organisations = organisations[:len(organisations)-1]
	}

	// Count previous rows for pagination
	var prevRowCount datastore.PrevRowCount
	if len(organisations) > 0 {
		first := organisations[0]
		count, err2 := s.repo.CountPrevUserOrganisations(ctx, repo.CountPrevUserOrganisationsParams{
			UserID: userID,
			Cursor: first.UID,
		})
		if err2 != nil {
			s.logger.WithError(err2).Error("failed to count prev user organisations")
			return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err2)
		}
		prevRowCount.Count = int(count)
	}

	// Build pagination data
	pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
	pagination = pagination.Build(pageable, ids)

	return organisations, *pagination, nil
}

// FindUserProjects retrieves all projects for a user
func (s *Service) FindUserProjects(ctx context.Context, userID string) ([]datastore.Project, error) {
	rows, err := s.repo.FindUserProjects(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Error("failed to find user projects")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	projects := make([]datastore.Project, 0, len(rows))
	for _, row := range rows {
		project := datastore.Project{
			UID:             row.ID,
			Name:            row.Name,
			Type:            datastore.ProjectType(row.Type),
			RetainedEvents:  int(row.RetainedEvents.Int32),
			LogoURL:         row.LogoUrl.String,
			OrganisationID:  row.OrganisationID,
			ProjectConfigID: row.ProjectConfigurationID,
			CreatedAt:       row.CreatedAt.Time,
			UpdatedAt:       row.UpdatedAt.Time,
		}
		projects = append(projects, project)
	}

	return projects, nil
}
