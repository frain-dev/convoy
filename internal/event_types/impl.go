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
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/event_types/repo"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrEventTypeNotFound   = errors.New("event type not found")
	ErrEventTypeNotCreated = errors.New("event type could not be created")
	ErrEventTypeNotUpdated = errors.New("event type could not be updated")
)

type Service struct {
	logger log.Logger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.EventTypesRepository at compile time
var _ datastore.EventTypesRepository = (*Service)(nil)

func New(logger log.Logger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
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
		ID:          common.StringToPgText(eventType.UID),
		Name:        common.StringToPgText(eventType.Name),
		Description: common.StringToPgText(eventType.Description),
		Category:    common.StringToPgText(eventType.Category),
		ProjectID:   common.StringToPgText(eventType.ProjectId),
		JsonSchema:  eventType.JSONSchema,
	})
	if err != nil {
		s.logger.Error("failed to create event type", "error", err)
		return util.NewServiceError(500, err)
	}

	return nil
}

func (s *Service) CreateDefaultEventType(ctx context.Context, projectId string) error {
	err := s.repo.CreateDefaultEventType(ctx, repo.CreateDefaultEventTypeParams{
		ID:          common.StringToPgText(ulid.Make().String()),
		Name:        common.StringToPgText("*"),
		Description: common.StringToPgTextNullable(""), // NULL
		Category:    common.StringToPgTextNullable(""), // NULL
		ProjectID:   common.StringToPgText(projectId),
		JsonSchema:  []byte("{}"),
	})
	if err != nil {
		s.logger.Error("failed to create default event type", "error", err)
		return util.NewServiceError(500, err)
	}

	return nil
}

func (s *Service) UpdateEventType(ctx context.Context, eventType *datastore.ProjectEventType) error {
	if eventType == nil {
		return util.NewServiceError(400, ErrEventTypeNotUpdated)
	}

	result, err := s.repo.UpdateEventType(ctx, repo.UpdateEventTypeParams{
		Description: common.StringToPgText(eventType.Description),
		Category:    common.StringToPgText(eventType.Category),
		JsonSchema:  eventType.JSONSchema,
		ID:          common.StringToPgText(eventType.UID),
		ProjectID:   common.StringToPgText(eventType.ProjectId),
	})
	if err != nil {
		s.logger.Error("failed to update event type", "error", err)
		return util.NewServiceError(500, err)
	}

	if result.RowsAffected() == 0 {
		return util.NewServiceError(404, ErrEventTypeNotUpdated)
	}

	return nil
}

func (s *Service) DeprecateEventType(ctx context.Context, id, projectId string) (*datastore.ProjectEventType, error) {
	row, err := s.repo.DeprecateEventType(ctx, repo.DeprecateEventTypeParams{
		ID:        common.StringToPgText(id),
		ProjectID: common.StringToPgText(projectId),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, util.NewServiceError(404, ErrEventTypeNotFound)
		}
		s.logger.Error("failed to deprecate event type", "error", err)
		return nil, util.NewServiceError(500, err)
	}

	eventType := rowToEventType(row)
	return &eventType, nil
}

func (s *Service) FetchEventTypeById(ctx context.Context, id, projectId string) (*datastore.ProjectEventType, error) {
	row, err := s.repo.FetchEventTypeByID(ctx, repo.FetchEventTypeByIDParams{
		ID:        common.StringToPgText(id),
		ProjectID: common.StringToPgText(projectId),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, util.NewServiceError(404, ErrEventTypeNotFound)
		}
		s.logger.Error("failed to fetch event type by id", "error", err)
		return nil, util.NewServiceError(500, err)
	}

	eventType := rowToEventType(row)
	return &eventType, nil
}

func (s *Service) FetchEventTypeByName(ctx context.Context, name, projectId string) (*datastore.ProjectEventType, error) {
	row, err := s.repo.FetchEventTypeByName(ctx, repo.FetchEventTypeByNameParams{
		Name:      common.StringToPgText(name),
		ProjectID: common.StringToPgText(projectId),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, util.NewServiceError(404, ErrEventTypeNotFound)
		}
		s.logger.Error("failed to fetch event type by name", "error", err)
		return nil, util.NewServiceError(500, err)
	}

	eventType := rowToEventType(row)
	return &eventType, nil
}

func (s *Service) CheckEventTypeExists(ctx context.Context, name, projectId string) (bool, error) {
	exists, err := s.repo.CheckEventTypeExists(ctx, repo.CheckEventTypeExistsParams{
		Name:      common.StringToPgText(name),
		ProjectID: common.StringToPgText(projectId),
	})
	if err != nil {
		s.logger.Error("failed to check event type exists", "error", err)
		return false, util.NewServiceError(500, err)
	}

	return exists, nil
}

func (s *Service) FetchAllEventTypes(ctx context.Context, projectId string) ([]datastore.ProjectEventType, error) {
	rows, err := s.repo.FetchAllEventTypes(ctx, common.StringToPgText(projectId))
	if err != nil {
		s.logger.Error("failed to fetch all event types", "error", err)
		return nil, util.NewServiceError(500, err)
	}

	eventTypes := make([]datastore.ProjectEventType, 0, len(rows))
	for _, row := range rows {
		eventType := rowToEventType(row)
		eventTypes = append(eventTypes, eventType)
	}

	return eventTypes, nil
}
