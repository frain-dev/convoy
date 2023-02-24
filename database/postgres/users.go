package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

const (
	createUser = `
    INSERT INTO convoy.users (
		id,first_name,last_name,email,password,
        email_verified,reset_password_token, email_verification_token,
        reset_password_expires_at,email_verification_expires_at)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    `

	updateUser = `
    UPDATE convoy.users SET
         first_name = $2,
         last_name=$3,
         email=$4,
         password=$5,
         email_verified=$6,
         reset_password_token=$7,
         email_verification_token=$8,
         reset_password_expires_at=$9,
         email_verification_expires_at=$10
    WHERE id = $1 AND deleted_at IS NULL ;
    `

	fetchUsers = `
	SELECT * FROM convoy.users
	WHERE %s = $1 AND deleted_at IS NULL;
	`

	fetchUsersPaginated = `
	SELECT * FROM convoy.users
	WHERE deleted_at IS NULL ORDER BY id
	LIMIT $1
	OFFSET $2;
	`

	countUsers = `
	SELECT COUNT(id) FROM convoy.users WHERE deleted_at IS NULL;
	`
)

var (
	ErrUserNotCreated = errors.New("user could not be created")
	ErrUserNotUpdated = errors.New("user could not be updated")
)

type userRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db database.Database) datastore.UserRepository {
	return &userRepo{db: db.GetDB()}
}

func (u *userRepo) CreateUser(ctx context.Context, user *datastore.User) error {
	result, err := u.db.ExecContext(ctx,
		createUser,
		user.UID,
		user.FirstName,
		user.LastName,
		user.Email,
		user.Password,
		user.EmailVerified,
		user.ResetPasswordToken,
		user.EmailVerificationToken,
		user.ResetPasswordExpiresAt,
		user.EmailVerificationExpiresAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return datastore.ErrDuplicateEmail
		}
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrUserNotCreated
	}

	return nil
}

func (u *userRepo) UpdateUser(ctx context.Context, user *datastore.User) error {
	result, err := u.db.Exec(
		updateUser, user.UID, user.FirstName, user.LastName, user.Email, user.Password, user.EmailVerified, user.ResetPasswordToken,
		user.EmailVerificationToken, user.ResetPasswordExpiresAt, user.EmailVerificationExpiresAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrUserNotUpdated
	}

	return nil
}

func (u *userRepo) FindUserByEmail(ctx context.Context, email string) (*datastore.User, error) {
	user := &datastore.User{}
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf(fetchUsers, "email"), email).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (u *userRepo) FindUserByID(ctx context.Context, id string) (*datastore.User, error) {
	user := &datastore.User{}
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf(fetchUsers, "id"), id).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (u *userRepo) FindUserByToken(ctx context.Context, token string) (*datastore.User, error) {
	user := &datastore.User{}
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf(fetchUsers, "reset_password_token"), token).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (u *userRepo) FindUserByEmailVerificationToken(ctx context.Context, token string) (*datastore.User, error) {
	user := &datastore.User{}
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf(fetchUsers, "email_verification_token"), token).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (u *userRepo) LoadUsersPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.User, datastore.PaginationData, error) {
	skip := getSkip(pageable.Page, pageable.PerPage)
	rows, err := u.db.QueryxContext(ctx, fetchUsersPaginated, pageable.PerPage, skip)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	var users []datastore.User
	var user datastore.User
	for rows.Next() {
		err = rows.StructScan(&user)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		users = append(users, user)
	}

	var count int
	err = u.db.Get(&count, countUsers)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	pagination := calculatePaginationData(count, pageable.Page, pageable.PerPage)
	return users, pagination, nil
}
