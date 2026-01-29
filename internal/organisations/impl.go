package organisations

import (
    "context"
    "errors"
    "fmt"
    "net/http"
    "time"

    "github.com/jackc/pgx/v5"
    "github.com/jackc/pgx/v5/pgtype"
    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/frain-dev/convoy/database"
    "github.com/frain-dev/convoy/datastore"
    "github.com/frain-dev/convoy/internal/common"
    "github.com/frain-dev/convoy/internal/organisations/repo"
    "github.com/frain-dev/convoy/pkg/log"
    "github.com/frain-dev/convoy/util"
)

// Service implements the OrganisationRepository using SQLc-generated queries
type Service struct {
    logger log.StdLogger
    repo   repo.Querier      // SQLc-generated interface
    db     *pgxpool.Pool     // Connection pool
    legacy database.Database // For gradual migration if needed
}

// Ensure Service implements datastore.OrganisationRepository at compile time
var _ datastore.OrganisationRepository = (*Service)(nil)

// New creates a new Organisation Service
func New(logger log.StdLogger, db database.Database) *Service {
    return &Service{
        logger: logger,
        repo:   repo.New(db.GetConn()),
        db:     db.GetConn(),
        legacy: db,
    }
}

// rowToOrganisation converts any SQLc-generated row struct to datastore.Organisation
func rowToOrganisation(row interface{}) datastore.Organisation {
    var (
        id, ownerID, name               string
        customDomain, assignedDomain    pgtype.Text
        createdAt, updatedAt, deletedAt pgtype.Timestamptz
        disabledAt                      pgtype.Timestamptz
    )

    switch r := row.(type) {
    case repo.FetchOrganisationByIDRow:
        id, ownerID, name = r.ID, r.OwnerID, r.Name
        customDomain, assignedDomain = r.CustomDomain, r.AssignedDomain
        createdAt, updatedAt, deletedAt, disabledAt = r.CreatedAt, r.UpdatedAt, r.DeletedAt, r.DisabledAt
    case repo.FetchOrganisationByCustomDomainRow:
        id, ownerID, name = r.ID, r.OwnerID, r.Name
        customDomain, assignedDomain = r.CustomDomain, r.AssignedDomain
        createdAt, updatedAt, deletedAt, disabledAt = r.CreatedAt, r.UpdatedAt, r.DeletedAt, r.DisabledAt
    case repo.FetchOrganisationByAssignedDomainRow:
        id, ownerID, name = r.ID, r.OwnerID, r.Name
        customDomain, assignedDomain = r.CustomDomain, r.AssignedDomain
        createdAt, updatedAt, deletedAt, disabledAt = r.CreatedAt, r.UpdatedAt, r.DeletedAt, r.DisabledAt
    case repo.FetchOrganisationsPaginatedRow:
        id, ownerID, name = r.ID, r.OwnerID, r.Name
        customDomain, assignedDomain = r.CustomDomain, r.AssignedDomain
        createdAt, updatedAt, deletedAt, disabledAt = r.CreatedAt, r.UpdatedAt, r.DeletedAt, r.DisabledAt
    default:
        return datastore.Organisation{}
    }

    return datastore.Organisation{
        UID:            id,
        OwnerID:        ownerID,
        Name:           name,
        CustomDomain:   common.PgTextToNullString(customDomain),
        AssignedDomain: common.PgTextToNullString(assignedDomain),
        CreatedAt:      createdAt.Time,
        UpdatedAt:      updatedAt.Time,
        DeletedAt:      common.PgTimestamptzToNullTime(deletedAt),
        DisabledAt:     common.PgTimestamptzToNullTime(disabledAt),
    }
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateOrganisation creates a new organisation
func (s *Service) CreateOrganisation(ctx context.Context, org *datastore.Organisation) error {
    if org == nil {
        return util.NewServiceError(http.StatusBadRequest, errors.New("organisation cannot be nil"))
    }

    err := s.repo.CreateOrganisation(ctx, repo.CreateOrganisationParams{
        ID:             org.UID,
        Name:           org.Name,
        OwnerID:        org.OwnerID,
        CustomDomain:   common.NullStringToPgText(org.CustomDomain),
        AssignedDomain: common.NullStringToPgText(org.AssignedDomain),
    })

    if err != nil {
        s.logger.WithError(err).Error("failed to create organisation")
        return util.NewServiceError(http.StatusInternalServerError, err)
    }

    return nil
}

// UpdateOrganisation updates an existing organisation
func (s *Service) UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error {
    if org == nil {
        return util.NewServiceError(http.StatusBadRequest, errors.New("organisation cannot be nil"))
    }

    result, err := s.repo.UpdateOrganisation(ctx, repo.UpdateOrganisationParams{
        ID:             org.UID,
        Name:           org.Name,
        CustomDomain:   common.NullStringToPgText(org.CustomDomain),
        AssignedDomain: common.NullStringToPgText(org.AssignedDomain),
        DisabledAt:     common.NullTimeToPgTimestamptz(org.DisabledAt),
    })

    if err != nil {
        s.logger.WithError(err).Error("failed to update organisation")
        return util.NewServiceError(http.StatusInternalServerError, err)
    }

    if result.RowsAffected() == 0 {
        return util.NewServiceError(http.StatusNotFound, datastore.ErrOrgNotFound)
    }

    return nil
}

// DeleteOrganisation soft deletes an organisation by ID
func (s *Service) DeleteOrganisation(ctx context.Context, id string) error {
    result, err := s.repo.DeleteOrganisation(ctx, id)
    if err != nil {
        s.logger.WithError(err).Error("failed to delete organisation")
        return util.NewServiceError(http.StatusInternalServerError, err)
    }

    if result.RowsAffected() == 0 {
        return util.NewServiceError(http.StatusNotFound, datastore.ErrOrgNotFound)
    }

    return nil
}

// FetchOrganisationByID retrieves an organisation by its ID
func (s *Service) FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
    row, err := s.repo.FetchOrganisationByID(ctx, id)
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, datastore.ErrOrgNotFound
        }
        s.logger.WithError(err).Error("failed to fetch organisation by id")
        return nil, util.NewServiceError(http.StatusInternalServerError, err)
    }

    org := rowToOrganisation(row)
    return &org, nil
}

// FetchOrganisationByCustomDomain retrieves an organisation by its custom domain
func (s *Service) FetchOrganisationByCustomDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
    row, err := s.repo.FetchOrganisationByCustomDomain(ctx, common.StringToPgText(domain))
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, datastore.ErrOrgNotFound
        }
        s.logger.WithError(err).Error("failed to fetch organisation by custom domain")
        return nil, util.NewServiceError(http.StatusInternalServerError, err)
    }

    org := rowToOrganisation(row)
    return &org, nil
}

// FetchOrganisationByAssignedDomain retrieves an organisation by its assigned domain
func (s *Service) FetchOrganisationByAssignedDomain(ctx context.Context, domain string) (*datastore.Organisation, error) {
    row, err := s.repo.FetchOrganisationByAssignedDomain(ctx, common.StringToPgText(domain))
    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, datastore.ErrOrgNotFound
        }
        s.logger.WithError(err).Error("failed to fetch organisation by assigned domain")
        return nil, util.NewServiceError(http.StatusInternalServerError, err)
    }

    org := rowToOrganisation(row)
    return &org, nil
}

// LoadOrganisationsPaged retrieves organisations with pagination
func (s *Service) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
    return s.LoadOrganisationsPagedWithSearch(ctx, pageable, "")
}

// LoadOrganisationsPagedWithSearch retrieves organisations with pagination and search
func (s *Service) LoadOrganisationsPagedWithSearch(ctx context.Context, pageable datastore.Pageable, search string) ([]datastore.Organisation, datastore.PaginationData, error) {
    // Determine direction for query
    direction := "next"
    if pageable.Direction == datastore.Prev {
        direction = "prev"
    }

    // Prepare search parameter
    hasSearch := !util.IsStringEmpty(search)
    searchParam := ""
    if hasSearch {
        searchParam = "%" + search + "%"
    }

    // Query organisations with pagination
    rows, err := s.repo.FetchOrganisationsPaginated(ctx, repo.FetchOrganisationsPaginatedParams{
        Direction: direction,
        Cursor:    pageable.Cursor(),
        HasSearch: hasSearch,
        Search:    searchParam,
        LimitVal:  int64(pageable.Limit()),
    })
    if err != nil {
        s.logger.WithError(err).Error("failed to load organisations paged")
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
        count, err := s.repo.CountPrevOrganisations(ctx, repo.CountPrevOrganisationsParams{
            Cursor:    first.UID,
            HasSearch: hasSearch,
            Search:    searchParam,
        })
        if err != nil {
            s.logger.WithError(err).Error("failed to count prev organisations")
            return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, err)
        }
        prevRowCount.Count = int(count.Int64)
    }

    // Build pagination data
    pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
    pagination = pagination.Build(pageable, ids)

    return organisations, *pagination, nil
}

// CountOrganisations returns the total count of organisations
func (s *Service) CountOrganisations(ctx context.Context) (int64, error) {
    count, err := s.repo.CountOrganisations(ctx)
    if err != nil {
        s.logger.WithError(err).Error("failed to count organisations")
        return 0, util.NewServiceError(http.StatusInternalServerError, err)
    }

    return count, nil
}

// CalculateUsage calculates usage metrics for an organisation
func (s *Service) CalculateUsage(ctx context.Context, orgID string, startTime, endTime time.Time) (*datastore.OrganisationUsage, error) {
    usage := &datastore.OrganisationUsage{
        OrganisationID: orgID,
        CreatedAt:      time.Now(),
    }

    // Calculate ingress bytes (raw + data bytes)
    ingressRow, err := s.repo.CalculateIngressBytes(ctx, repo.CalculateIngressBytesParams{
        OrganisationID: orgID,
        CreatedAt:      pgtype.Timestamptz{Time: startTime, Valid: true},
        CreatedAt_2:    pgtype.Timestamptz{Time: endTime, Valid: true},
    })
    if err != nil {
        s.logger.WithError(err).Error("failed to calculate ingress bytes")
        return nil, util.NewServiceError(http.StatusInternalServerError, fmt.Errorf("failed to calculate ingress bytes: %w", err))
    }
    usage.Received.Bytes = ingressRow.RawBytes.Int64 + ingressRow.DataBytes.Int64

    // Calculate egress bytes
    egressBytes, err := s.repo.CalculateEgressBytes(ctx, repo.CalculateEgressBytesParams{
        OrganisationID: orgID,
        CreatedAt:      pgtype.Timestamptz{Time: startTime, Valid: true},
        CreatedAt_2:    pgtype.Timestamptz{Time: endTime, Valid: true},
    })
    if err != nil {
        s.logger.WithError(err).Error("failed to calculate egress bytes")
        return nil, util.NewServiceError(http.StatusInternalServerError, fmt.Errorf("failed to calculate egress bytes: %w", err))
    }
    usage.Sent.Bytes = egressBytes

    // Count events
    eventCount, err := s.repo.CountOrgEvents(ctx, repo.CountOrgEventsParams{
        OrganisationID: orgID,
        CreatedAt:      pgtype.Timestamptz{Time: startTime, Valid: true},
        CreatedAt_2:    pgtype.Timestamptz{Time: endTime, Valid: true},
    })
    if err != nil {
        s.logger.WithError(err).Error("failed to count events")
        return nil, util.NewServiceError(http.StatusInternalServerError, fmt.Errorf("failed to count events: %w", err))
    }
    usage.Received.Volume = eventCount

    // Count deliveries
    deliveryCount, err := s.repo.CountOrgDeliveries(ctx, repo.CountOrgDeliveriesParams{
        OrganisationID: orgID,
        CreatedAt:      pgtype.Timestamptz{Time: startTime, Valid: true},
        CreatedAt_2:    pgtype.Timestamptz{Time: endTime, Valid: true},
    })
    if err != nil {
        s.logger.WithError(err).Error("failed to count deliveries")
        return nil, util.NewServiceError(http.StatusInternalServerError, fmt.Errorf("failed to count deliveries: %w", err))
    }
    usage.Sent.Volume = deliveryCount

    // Format period as YYYY-MM
    usage.Period = startTime.Format("2006-01")

    return usage, nil
}
