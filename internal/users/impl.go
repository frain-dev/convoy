package users

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/users/repo"
	"github.com/frain-dev/convoy/pkg/log"
)

var (
	ErrUserNotCreated = errors.New("user could not be created")
	ErrUserNotUpdated = errors.New("user could not be updated")
)

// Service implements the UserRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.UserRepository at compile time
var _ datastore.UserRepository = (*Service)(nil)

// New creates a new User Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// ============================================================================
// CREATE Operations
// ============================================================================

func (s *Service) CreateUser(ctx context.Context, user *datastore.User) error {
	params := userToCreateParams(user)

	err := s.repo.CreateUser(ctx, params)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique constraint") {
			return datastore.ErrDuplicateEmail
		}
		s.logger.WithError(err).Error("failed to create user")
		return err
	}

	return nil
}

// ============================================================================
// UPDATE Operations
// ============================================================================

func (s *Service) UpdateUser(ctx context.Context, user *datastore.User) error {
	params := userToUpdateParams(user)

	result, err := s.repo.UpdateUser(ctx, params)
	if err != nil {
		s.logger.WithError(err).Error("failed to update user")
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected < 1 {
		return ErrUserNotUpdated
	}

	return nil
}

// ============================================================================
// FETCH Operations
// ============================================================================

func (s *Service) FindUserByID(ctx context.Context, id string) (*datastore.User, error) {
	row, err := s.repo.FindUserByID(ctx, pgtype.Text{String: id, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		s.logger.WithError(err).Error("failed to find user by ID")
		return nil, err
	}

	return rowToUserFromFindByID(row), nil
}

func (s *Service) FindUserByEmail(ctx context.Context, email string) (*datastore.User, error) {
	row, err := s.repo.FindUserByEmail(ctx, pgtype.Text{String: email, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		s.logger.WithError(err).Error("failed to find user by email")
		return nil, err
	}

	return rowToUserFromFindByEmail(row), nil
}

func (s *Service) FindUserByToken(ctx context.Context, token string) (*datastore.User, error) {
	row, err := s.repo.FindUserByToken(ctx, pgtype.Text{String: token, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		s.logger.WithError(err).Error("failed to find user by token")
		return nil, err
	}

	return rowToUserFromFindByToken(row), nil
}

func (s *Service) FindUserByEmailVerificationToken(ctx context.Context, token string) (*datastore.User, error) {
	row, err := s.repo.FindUserByEmailVerificationToken(ctx, pgtype.Text{String: token, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		s.logger.WithError(err).Error("failed to find user by email verification token")
		return nil, err
	}

	return rowToUserFromFindByEmailVerificationToken(row), nil
}

// ============================================================================
// COUNT Operations
// ============================================================================

func (s *Service) CountUsers(ctx context.Context) (int64, error) {
	count, err := s.repo.CountUsers(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to count users")
		return 0, err
	}

	return count.Int64, nil
}

// ============================================================================
// Type Conversion Helpers
// ============================================================================

// userToCreateParams converts datastore.User to repo.CreateUserParams
func userToCreateParams(user *datastore.User) repo.CreateUserParams {
	return repo.CreateUserParams{
		ID:                         pgtype.Text{String: user.UID, Valid: true},
		FirstName:                  pgtype.Text{String: user.FirstName, Valid: true},
		LastName:                   pgtype.Text{String: user.LastName, Valid: true},
		Email:                      pgtype.Text{String: user.Email, Valid: true},
		Password:                   pgtype.Text{String: user.Password, Valid: true},
		EmailVerified:              pgtype.Bool{Bool: user.EmailVerified, Valid: true},
		ResetPasswordToken:         common.StringToPgText(user.ResetPasswordToken),
		EmailVerificationToken:     common.StringToPgText(user.EmailVerificationToken),
		ResetPasswordExpiresAt:     common.TimeToPgTimestamptz(user.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: common.TimeToPgTimestamptz(user.EmailVerificationExpiresAt),
		AuthType:                   pgtype.Text{String: user.AuthType, Valid: true},
	}
}

// userToUpdateParams converts datastore.User to repo.UpdateUserParams
func userToUpdateParams(user *datastore.User) repo.UpdateUserParams {
	return repo.UpdateUserParams{
		ID:                         pgtype.Text{String: user.UID, Valid: true},
		FirstName:                  pgtype.Text{String: user.FirstName, Valid: true},
		LastName:                   pgtype.Text{String: user.LastName, Valid: true},
		Email:                      pgtype.Text{String: user.Email, Valid: true},
		Password:                   pgtype.Text{String: user.Password, Valid: true},
		EmailVerified:              pgtype.Bool{Bool: user.EmailVerified, Valid: true},
		ResetPasswordToken:         common.StringToPgText(user.ResetPasswordToken),
		EmailVerificationToken:     common.StringToPgText(user.EmailVerificationToken),
		ResetPasswordExpiresAt:     common.TimeToPgTimestamptz(user.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: common.TimeToPgTimestamptz(user.EmailVerificationExpiresAt),
	}
}

// rowToUserFromFindByID converts repo.FindUserByIDRow to datastore.User
func rowToUserFromFindByID(row repo.FindUserByIDRow) *datastore.User {
	return &datastore.User{
		UID:                        row.ID,
		FirstName:                  row.FirstName,
		LastName:                   row.LastName,
		Email:                      row.Email,
		Password:                   row.Password,
		EmailVerified:              row.EmailVerified,
		ResetPasswordToken:         common.PgTextToString(row.ResetPasswordToken),
		EmailVerificationToken:     common.PgTextToString(row.EmailVerificationToken),
		CreatedAt:                  common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:                  common.PgTimestamptzToTime(row.UpdatedAt),
		DeletedAt:                  common.PgTimestamptzToNullTime(row.DeletedAt),
		ResetPasswordExpiresAt:     common.PgTimestamptzToTime(row.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: common.PgTimestamptzToTime(row.EmailVerificationExpiresAt),
		AuthType:                   row.AuthType,
	}
}

// rowToUserFromFindByEmail converts repo.FindUserByEmailRow to datastore.User
func rowToUserFromFindByEmail(row repo.FindUserByEmailRow) *datastore.User {
	return &datastore.User{
		UID:                        row.ID,
		FirstName:                  row.FirstName,
		LastName:                   row.LastName,
		Email:                      row.Email,
		Password:                   row.Password,
		EmailVerified:              row.EmailVerified,
		ResetPasswordToken:         common.PgTextToString(row.ResetPasswordToken),
		EmailVerificationToken:     common.PgTextToString(row.EmailVerificationToken),
		CreatedAt:                  common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:                  common.PgTimestamptzToTime(row.UpdatedAt),
		DeletedAt:                  common.PgTimestamptzToNullTime(row.DeletedAt),
		ResetPasswordExpiresAt:     common.PgTimestamptzToTime(row.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: common.PgTimestamptzToTime(row.EmailVerificationExpiresAt),
		AuthType:                   row.AuthType,
	}
}

// rowToUserFromFindByToken converts repo.FindUserByTokenRow to datastore.User
func rowToUserFromFindByToken(row repo.FindUserByTokenRow) *datastore.User {
	return &datastore.User{
		UID:                        row.ID,
		FirstName:                  row.FirstName,
		LastName:                   row.LastName,
		Email:                      row.Email,
		Password:                   row.Password,
		EmailVerified:              row.EmailVerified,
		ResetPasswordToken:         common.PgTextToString(row.ResetPasswordToken),
		EmailVerificationToken:     common.PgTextToString(row.EmailVerificationToken),
		CreatedAt:                  common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:                  common.PgTimestamptzToTime(row.UpdatedAt),
		DeletedAt:                  common.PgTimestamptzToNullTime(row.DeletedAt),
		ResetPasswordExpiresAt:     common.PgTimestamptzToTime(row.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: common.PgTimestamptzToTime(row.EmailVerificationExpiresAt),
		AuthType:                   row.AuthType,
	}
}

// rowToUserFromFindByEmailVerificationToken converts repo.FindUserByEmailVerificationTokenRow to datastore.User
func rowToUserFromFindByEmailVerificationToken(row repo.FindUserByEmailVerificationTokenRow) *datastore.User {
	return &datastore.User{
		UID:                        row.ID,
		FirstName:                  row.FirstName,
		LastName:                   row.LastName,
		Email:                      row.Email,
		Password:                   row.Password,
		EmailVerified:              row.EmailVerified,
		ResetPasswordToken:         common.PgTextToString(row.ResetPasswordToken),
		EmailVerificationToken:     common.PgTextToString(row.EmailVerificationToken),
		CreatedAt:                  common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:                  common.PgTimestamptzToTime(row.UpdatedAt),
		DeletedAt:                  common.PgTimestamptzToNullTime(row.DeletedAt),
		ResetPasswordExpiresAt:     common.PgTimestamptzToTime(row.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: common.PgTimestamptzToTime(row.EmailVerificationExpiresAt),
		AuthType:                   row.AuthType,
	}
}
