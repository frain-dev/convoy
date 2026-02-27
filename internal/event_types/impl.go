package event_types

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/event_types/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrEventTypeNotFound   = errors.New("event type not found")
	ErrEventTypeNotCreated = errors.New("event type could not be created")
	ErrEventTypeNotUpdated = errors.New("event type could not be updated")
)

type Service struct {
	logger log.StdLogger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.EventTypesRepository at compile time
var _ datastore.EventTypesRepository = (*Service)(nil)

func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// Helper function to convert string to pgtype.Text (nullable)
// Empty strings are converted to NULL
func textToPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{String: "", Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// Helper function to convert row to ProjectEventType
// Handles all 4 row types (FetchEventTypeByID, FetchEventTypeByName, FetchAllEventTypes, DeprecateEventType)
func rowToEventType(row any) datastore.ProjectEventType {
	var (
		id, name, projectID string
		description         pgtype.Text
		category            pgtype.Text
		jsonSchema          []byte
		createdAt           pgtype.Timestamptz
		updatedAt           pgtype.Timestamptz
		deprecatedAt        pgtype.Timestamptz
	)

	switch r := row.(type) {
	case repo.FetchEventTypeByIDRow:
		id, name, projectID = r.ID, r.Name, r.ProjectID
		description, category = r.Description, r.Category
		jsonSchema = r.JsonSchema
		createdAt, updatedAt, deprecatedAt = r.CreatedAt, r.UpdatedAt, r.DeprecatedAt
	case repo.FetchEventTypeByNameRow:
		id, name, projectID = r.ID, r.Name, r.ProjectID
		description, category = r.Description, r.Category
		jsonSchema = r.JsonSchema
		createdAt, updatedAt, deprecatedAt = r.CreatedAt, r.UpdatedAt, r.DeprecatedAt
	case repo.FetchAllEventTypesRow:
		id, name, projectID = r.ID, r.Name, r.ProjectID
		description, category = r.Description, r.Category
		jsonSchema = r.JsonSchema
		createdAt, updatedAt, deprecatedAt = r.CreatedAt, r.UpdatedAt, r.DeprecatedAt
	case repo.DeprecateEventTypeRow:
		id, name, projectID = r.ID, r.Name, r.ProjectID
		description, category = r.Description, r.Category
		jsonSchema = r.JsonSchema
		createdAt, updatedAt, deprecatedAt = r.CreatedAt, r.UpdatedAt, r.DeprecatedAt
	default:
		return datastore.ProjectEventType{}
	}

	// Convert pgtype values to datastore types
	return datastore.ProjectEventType{
		UID:          id,
		Name:         name,
		ProjectId:    projectID,
		Description:  description.String,
		Category:     category.String,
		JSONSchema:   jsonSchema,
		CreatedAt:    createdAt.Time,
		UpdatedAt:    updatedAt.Time,
		DeprecatedAt: null.NewTime(deprecatedAt.Time, deprecatedAt.Valid),
	}
}

func (s *Service) CreateEventType(ctx context.Context, eventType *datastore.ProjectEventType) error {
	if eventType == nil {
		return util.NewServiceError(400, ErrEventTypeNotCreated)
	}

	err := s.repo.CreateEventType(ctx, repo.CreateEventTypeParams{
		ID:          eventType.UID,
		Name:        eventType.Name,
		Description: textToPgText(eventType.Description),
		Category:    textToPgText(eventType.Category),
		ProjectID:   eventType.ProjectId,
		JsonSchema:  eventType.JSONSchema,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to create event type")
		return util.NewServiceError(500, err)
	}

	return nil
}

func (s *Service) CreateDefaultEventType(ctx context.Context, projectId string) error {
	err := s.repo.CreateDefaultEventType(ctx, repo.CreateDefaultEventTypeParams{
		ID:          ulid.Make().String(),
		Name:        "*",
		Description: pgtype.Text{String: "", Valid: false}, // NULL
		Category:    pgtype.Text{String: "", Valid: false}, // NULL
		ProjectID:   projectId,
		JsonSchema:  []byte("{}"),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to create default event type")
		return util.NewServiceError(500, err)
	}

	return nil
}

func (s *Service) UpdateEventType(ctx context.Context, eventType *datastore.ProjectEventType) error {
	if eventType == nil {
		return util.NewServiceError(400, ErrEventTypeNotUpdated)
	}

	result, err := s.repo.UpdateEventType(ctx, repo.UpdateEventTypeParams{
		Description: textToPgText(eventType.Description),
		Category:    textToPgText(eventType.Category),
		JsonSchema:  eventType.JSONSchema,
		ID:          eventType.UID,
		ProjectID:   eventType.ProjectId,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to update event type")
		return util.NewServiceError(500, err)
	}

	if result.RowsAffected() == 0 {
		return util.NewServiceError(404, ErrEventTypeNotUpdated)
	}

	return nil
}

func (s *Service) DeprecateEventType(ctx context.Context, id, projectId string) (*datastore.ProjectEventType, error) {
	row, err := s.repo.DeprecateEventType(ctx, repo.DeprecateEventTypeParams{
		ID:        id,
		ProjectID: projectId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, util.NewServiceError(404, ErrEventTypeNotFound)
		}
		s.logger.WithError(err).Error("failed to deprecate event type")
		return nil, util.NewServiceError(500, err)
	}

	eventType := rowToEventType(row)
	return &eventType, nil
}

func (s *Service) FetchEventTypeById(ctx context.Context, id, projectId string) (*datastore.ProjectEventType, error) {
	row, err := s.repo.FetchEventTypeByID(ctx, repo.FetchEventTypeByIDParams{
		ID:        id,
		ProjectID: projectId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, util.NewServiceError(404, ErrEventTypeNotFound)
		}
		s.logger.WithError(err).Error("failed to fetch event type by id")
		return nil, util.NewServiceError(500, err)
	}

	eventType := rowToEventType(row)
	return &eventType, nil
}

func (s *Service) FetchEventTypeByName(ctx context.Context, name, projectId string) (*datastore.ProjectEventType, error) {
	row, err := s.repo.FetchEventTypeByName(ctx, repo.FetchEventTypeByNameParams{
		Name:      name,
		ProjectID: projectId,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, util.NewServiceError(404, ErrEventTypeNotFound)
		}
		s.logger.WithError(err).Error("failed to fetch event type by name")
		return nil, util.NewServiceError(500, err)
	}

	eventType := rowToEventType(row)
	return &eventType, nil
}

func (s *Service) CheckEventTypeExists(ctx context.Context, name, projectId string) (bool, error) {
	exists, err := s.repo.CheckEventTypeExists(ctx, repo.CheckEventTypeExistsParams{
		Name:      name,
		ProjectID: projectId,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to check event type exists")
		return false, util.NewServiceError(500, err)
	}

	return exists, nil
}

func (s *Service) FetchAllEventTypes(ctx context.Context, projectId string) ([]datastore.ProjectEventType, error) {
	rows, err := s.repo.FetchAllEventTypes(ctx, projectId)
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch all event types")
		return nil, util.NewServiceError(500, err)
	}

	eventTypes := make([]datastore.ProjectEventType, 0, len(rows))
	for _, row := range rows {
		eventType := rowToEventType(row)
		eventTypes = append(eventTypes, eventType)
	}

	return eventTypes, nil
}
