package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/frain-dev/convoy/cache"
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
    WHERE id = $1 AND deleted_at IS NULL;
    `

	fetchUsers = `
	SELECT * FROM convoy.users
	WHERE deleted_at IS NULL
	`

	fetchUsersPaginated = `
	SELECT * FROM convoy.users WHERE deleted_at IS NULL`

	fetchUsersPagedForward = `
	%s
	AND id <= :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT :limit`

	fetchUsersPagedBackward = `
	WITH users AS (
		%s
		AND id >= :cursor
		GROUP BY id
		ORDER BY id ASC
		LIMIT :limit
	)

	SELECT * FROM users ORDER BY id DESC`

	countPrevUsers = `
	SELECT COUNT(DISTINCT(id)) AS count
	FROM convoy.users
	WHERE deleted_at IS NULL
	AND id > :cursor
	GROUP BY id
	ORDER BY id DESC
	LIMIT 1`

	countUsers = `
	SELECT COUNT(*) AS count
	FROM convoy.users
	WHERE deleted_at IS NULL`
)

var (
	ErrUserNotCreated = errors.New("user could not be created")
	ErrUserNotUpdated = errors.New("user could not be updated")
)

type userRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db database.Database, ca cache.Cache) datastore.UserRepository {
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
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND email = $1;", fetchUsers), email).StructScan(user)
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
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND id = $1;", fetchUsers), id).StructScan(user)
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
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND reset_password_token = $1;", fetchUsers), token).StructScan(user)
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
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND email_verification_token = $1;", fetchUsers), token).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (o *userRepo) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := o.db.GetContext(ctx, &count, countUsers)
	if err != nil {
		return 0, err
	}

	return count, nil
}
