package organisation_invites

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
	"github.com/frain-dev/convoy/internal/organisation_invites/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// Service implements the OrganisationInviteRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier      // SQLc-generated interface
	db     *pgxpool.Pool     // Connection pool
	legacy database.Database // For gradual migration if needed
}

// Ensure Service implements datastore.OrganisationInviteRepository at compile time
var _ datastore.OrganisationInviteRepository = (*Service)(nil)

// New creates a new Organisation Invite Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
		legacy: db,
	}
}

// rowToOrganisationInvite converts any SQLc-generated row struct to datastore.OrganisationInvite
func rowToOrganisationInvite(row interface{}) datastore.OrganisationInvite {
	var (
		id, organisationID, inviteeEmail, token, status string
		roleType, roleProject, roleEndpoint             string
		createdAt, updatedAt, expiresAt                 pgtype.Timestamptz
	)

	switch r := row.(type) {
	case repo.FetchOrganisationInviteByIDRow:
		id, organisationID, inviteeEmail, token = r.ID, r.OrganisationID, r.InviteeEmail, r.Token
		status = r.Status
		roleType = r.RoleType
		roleProject = common.PgTextToString(r.RoleProject)
		roleEndpoint = common.PgTextToString(r.RoleEndpoint)
		createdAt, updatedAt, expiresAt = r.CreatedAt, r.UpdatedAt, r.ExpiresAt
	case repo.FetchOrganisationInviteByTokenRow:
		id, organisationID, inviteeEmail, token = r.ID, r.OrganisationID, r.InviteeEmail, r.Token
		status = r.Status
		roleType = r.RoleType
		roleProject = common.PgTextToString(r.RoleProject)
		roleEndpoint = common.PgTextToString(r.RoleEndpoint)
		createdAt, updatedAt, expiresAt = r.CreatedAt, r.UpdatedAt, r.ExpiresAt
	case repo.FetchOrganisationInvitesPaginatedRow:
		id, organisationID, inviteeEmail = r.ID, r.OrganisationID, r.InviteeEmail
		status = r.Status
		roleType = r.RoleType
		roleProject = common.PgTextToString(r.RoleProject)
		roleEndpoint = common.PgTextToString(r.RoleEndpoint)
		createdAt, updatedAt, expiresAt = r.CreatedAt, r.UpdatedAt, r.ExpiresAt
		// Note: Token is not included in paginated results for security
		token = ""
	default:
		return datastore.OrganisationInvite{}
	}

	return datastore.OrganisationInvite{
		UID:            id,
		OrganisationID: organisationID,
		InviteeEmail:   inviteeEmail,
		Token:          token,
		Status:         datastore.InviteStatus(status),
		Role:           common.ParamsToRole(roleType, roleProject, roleEndpoint),
		ExpiresAt:      expiresAt.Time,
		CreatedAt:      createdAt.Time,
		UpdatedAt:      updatedAt.Time,
	}
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateOrganisationInvite creates a new organisation invite
func (s *Service) CreateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	if iv == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("organisation invite cannot be nil"))
	}

	// Convert role to database params
	roleTypePg, roleProjectPg, roleEndpointPg := common.RoleToParams(iv.Role)

	err := s.repo.CreateOrganisationInvite(ctx, repo.CreateOrganisationInviteParams{
		ID:             pgtype.Text{String: iv.UID, Valid: true},
		OrganisationID: pgtype.Text{String: iv.OrganisationID, Valid: true},
		InviteeEmail:   pgtype.Text{String: iv.InviteeEmail, Valid: true},
		Token:          pgtype.Text{String: iv.Token, Valid: true},
		RoleType:       roleTypePg,
		RoleProject:    roleProjectPg,
		RoleEndpoint:   roleEndpointPg,
		Status:         pgtype.Text{String: string(iv.Status), Valid: true},
		ExpiresAt:      pgtype.Timestamptz{Time: iv.ExpiresAt, Valid: true},
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to create organisation invite")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// UpdateOrganisationInvite updates an existing organisation invite
func (s *Service) UpdateOrganisationInvite(ctx context.Context, iv *datastore.OrganisationInvite) error {
	if iv == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("organisation invite cannot be nil"))
	}

	// Convert role to database params
	roleTypePg, roleProjectPg, roleEndpointPg := common.RoleToParams(iv.Role)

	err := s.repo.UpdateOrganisationInvite(ctx, repo.UpdateOrganisationInviteParams{
		ID:           pgtype.Text{String: iv.UID, Valid: true},
		RoleType:     roleTypePg,
		RoleProject:  roleProjectPg,
		RoleEndpoint: roleEndpointPg,
		Status:       pgtype.Text{String: string(iv.Status), Valid: true},
		ExpiresAt:    pgtype.Timestamptz{Time: iv.ExpiresAt, Valid: true},
		DeletedAt:    common.NullTimeToPgTimestamptz(iv.DeletedAt),
	})

	if err != nil {
		s.logger.WithError(err).Error("failed to update organisation invite")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// DeleteOrganisationInvite soft deletes an organisation invite by ID
func (s *Service) DeleteOrganisationInvite(ctx context.Context, uid string) error {
	err := s.repo.DeleteOrganisationInvite(ctx, pgtype.Text{String: uid, Valid: true})
	if err != nil {
		s.logger.WithError(err).Error("failed to delete organisation invite")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// FetchOrganisationInviteByID retrieves an organisation invite by its ID
func (s *Service) FetchOrganisationInviteByID(ctx context.Context, uid string) (*datastore.OrganisationInvite, error) {
	row, err := s.repo.FetchOrganisationInviteByID(ctx, pgtype.Text{String: uid, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrOrgInviteNotFound
		}
		s.logger.WithError(err).Error("failed to fetch organisation invite by id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	invite := rowToOrganisationInvite(row)
	return &invite, nil
}

// FetchOrganisationInviteByToken retrieves an organisation invite by its token
func (s *Service) FetchOrganisationInviteByToken(ctx context.Context, token string) (*datastore.OrganisationInvite, error) {
	row, err := s.repo.FetchOrganisationInviteByToken(ctx, pgtype.Text{String: token, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrOrgInviteNotFound
		}
		s.logger.WithError(err).Error("failed to fetch organisation invite by token")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	invite := rowToOrganisationInvite(row)
	return &invite, nil
}

// LoadOrganisationsInvitesPaged retrieves organisation invites with pagination
func (s *Service) LoadOrganisationsInvitesPaged(ctx context.Context, orgID string, inviteStatus datastore.InviteStatus, pageable datastore.Pageable) ([]datastore.OrganisationInvite, datastore.PaginationData, error) {
	// Determine direction for query
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	// Query organisation invites with pagination
	rows, err := s.repo.FetchOrganisationInvitesPaginated(ctx, repo.FetchOrganisationInvitesPaginatedParams{
		Direction:      pgtype.Text{String: direction, Valid: true},
		OrganisationID: pgtype.Text{String: orgID, Valid: true},
		Status:         pgtype.Text{String: string(inviteStatus), Valid: true},
		Cursor:         pgtype.Text{String: pageable.Cursor(), Valid: true},
		LimitVal:       pgtype.Int8{Int64: int64(pageable.Limit()), Valid: true},
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to load organisation invites paged")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Convert rows to organisation invites
	invites := make([]datastore.OrganisationInvite, 0, len(rows))
	for _, row := range rows {
		invite := rowToOrganisationInvite(row)
		invites = append(invites, invite)
	}

	// Build IDs for pagination
	ids := make([]string, len(invites))
	for i := range invites {
		ids[i] = invites[i].UID
	}

	// If we got more results than requested, trim the extra one (used for hasNext detection)
	if len(invites) > pageable.PerPage {
		invites = invites[:len(invites)-1]
	}

	// Count previous rows for pagination
	var prevRowCount datastore.PrevRowCount
	if len(invites) > 0 {
		first := invites[0]
		count, err := s.repo.CountPrevOrganisationInvites(ctx, repo.CountPrevOrganisationInvitesParams{
			OrgID:  pgtype.Text{String: orgID, Valid: true},
			Cursor: pgtype.Text{String: first.UID, Valid: true},
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to count prev organisation invites")
			return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
		}
		prevRowCount.Count = int(count.Int64)
	}

	// Build pagination data
	pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
	pagination = pagination.Build(pageable, ids)

	return invites, *pagination, nil
}
