package users

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
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
	row, err := s.repo.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		s.logger.WithError(err).Error("failed to find user by ID")
		return nil, err
	}

	return rowToUser(row), nil
}

func (s *Service) FindUserByEmail(ctx context.Context, email string) (*datastore.User, error) {
	row, err := s.repo.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		s.logger.WithError(err).Error("failed to find user by email")
		return nil, err
	}

	return rowToUser(row), nil
}

func (s *Service) FindUserByToken(ctx context.Context, token string) (*datastore.User, error) {
	row, err := s.repo.FindUserByToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		s.logger.WithError(err).Error("failed to find user by token")
		return nil, err
	}

	return rowToUser(row), nil
}

func (s *Service) FindUserByEmailVerificationToken(ctx context.Context, token string) (*datastore.User, error) {
	row, err := s.repo.FindUserByEmailVerificationToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		s.logger.WithError(err).Error("failed to find user by email verification token")
		return nil, err
	}

	return rowToUser(row), nil
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

	return count, nil
}

// ============================================================================
// Type Conversion Helpers
// ============================================================================

// userToCreateParams converts datastore.User to repo.CreateUserParams
func userToCreateParams(user *datastore.User) repo.CreateUserParams {
	return repo.CreateUserParams{
		ID:                         user.UID,
		FirstName:                  user.FirstName,
		LastName:                   user.LastName,
		Email:                      user.Email,
		Password:                   user.Password,
		EmailVerified:              user.EmailVerified,
		ResetPasswordToken:         common.StringToPgText(user.ResetPasswordToken),
		EmailVerificationToken:     common.StringToPgText(user.EmailVerificationToken),
		ResetPasswordExpiresAt:     common.TimeToPgTimestamptz(user.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: common.TimeToPgTimestamptz(user.EmailVerificationExpiresAt),
		AuthType:                   user.AuthType,
	}
}

// userToUpdateParams converts datastore.User to repo.UpdateUserParams
func userToUpdateParams(user *datastore.User) repo.UpdateUserParams {
	return repo.UpdateUserParams{
		ID:                         user.UID,
		FirstName:                  user.FirstName,
		LastName:                   user.LastName,
		Email:                      user.Email,
		Password:                   user.Password,
		EmailVerified:              user.EmailVerified,
		ResetPasswordToken:         common.StringToPgText(user.ResetPasswordToken),
		EmailVerificationToken:     common.StringToPgText(user.EmailVerificationToken),
		ResetPasswordExpiresAt:     common.TimeToPgTimestamptz(user.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: common.TimeToPgTimestamptz(user.EmailVerificationExpiresAt),
	}
}

// rowToUser converts repo.ConvoyUser to datastore.User
func rowToUser(row repo.ConvoyUser) *datastore.User {
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
