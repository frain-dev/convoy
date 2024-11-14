package sqlite3

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
	"strings"
	"time"
)

const (
	createUser = `
    INSERT INTO users (
		id,first_name,last_name,email,password, email_verified,
		reset_password_token, email_verification_token, reset_password_expires_at,
		email_verification_expires_at, auth_type)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
    `

	updateUser = `
    UPDATE users SET
         first_name=$1,
         last_name=$2,
         email=$3,
         password=$4,
         email_verified=$5,
         reset_password_token=$6,
         email_verification_token=$7,
         reset_password_expires_at=$8,
         email_verification_expires_at=$9,
         updated_at=$10
    WHERE id = $11 AND deleted_at IS NULL;
    `

	fetchUsers = `
	SELECT * FROM users
	WHERE deleted_at IS NULL
	`

	countUsers = `
	SELECT COUNT(*) AS count
	FROM users
	WHERE deleted_at IS NULL`
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
		user.AuthType,
	)
	if err != nil {
		if strings.Contains(err.Error(), "constraint") {
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
	result, err := u.db.ExecContext(ctx,
		updateUser, user.FirstName, user.LastName, user.Email, user.Password, user.EmailVerified, user.ResetPasswordToken,
		user.EmailVerificationToken, user.ResetPasswordExpiresAt, user.EmailVerificationExpiresAt, time.Now(), user.UID,
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
	user := &dbUser{}
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND email = $1;", fetchUsers), email).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user.toDatastoreUser(), nil
}

func (u *userRepo) FindUserByID(ctx context.Context, id string) (*datastore.User, error) {
	user := &dbUser{}
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND id = $1;", fetchUsers), id).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user.toDatastoreUser(), nil
}

func (u *userRepo) FindUserByToken(ctx context.Context, token string) (*datastore.User, error) {
	user := &dbUser{}
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND reset_password_token = $1;", fetchUsers), token).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user.toDatastoreUser(), nil
}

func (u *userRepo) FindUserByEmailVerificationToken(ctx context.Context, token string) (*datastore.User, error) {
	user := &dbUser{}
	err := u.db.QueryRowxContext(ctx, fmt.Sprintf("%s AND email_verification_token = $1;", fetchUsers), token).StructScan(user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrUserNotFound
		}
		return nil, err
	}

	return user.toDatastoreUser(), nil
}

func (u *userRepo) CountUsers(ctx context.Context) (int64, error) {
	var userCount int64
	err := u.db.GetContext(ctx, &userCount, countUsers)
	if err != nil {
		return 0, err
	}

	return userCount, nil
}

type dbUser struct {
	UID                        string  `db:"id"`
	FirstName                  string  `db:"first_name"`
	LastName                   string  `db:"last_name"`
	Email                      string  `db:"email"`
	EmailVerified              bool    `db:"email_verified"`
	Password                   string  `db:"password"`
	ResetPasswordToken         string  `db:"reset_password_token"`
	EmailVerificationToken     string  `db:"email_verification_token"`
	CreatedAt                  string  `db:"created_at,omitempty"`
	UpdatedAt                  string  `db:"updated_at,omitempty"`
	DeletedAt                  *string `db:"deleted_at"`
	ResetPasswordExpiresAt     string  `db:"reset_password_expires_at,omitempty"`
	EmailVerificationExpiresAt string  `db:"email_verification_expires_at,omitempty"`
	AuthType                   string  `db:"auth_type"`
}

func (uu *dbUser) toDatastoreUser() *datastore.User {
	return &datastore.User{
		UID:                        uu.UID,
		FirstName:                  uu.FirstName,
		LastName:                   uu.LastName,
		Email:                      uu.Email,
		AuthType:                   uu.AuthType,
		EmailVerified:              uu.EmailVerified,
		Password:                   uu.Password,
		ResetPasswordToken:         uu.ResetPasswordToken,
		EmailVerificationToken:     uu.EmailVerificationToken,
		CreatedAt:                  asTime(uu.CreatedAt),
		UpdatedAt:                  asTime(uu.UpdatedAt),
		DeletedAt:                  asNullTime(uu.DeletedAt),
		ResetPasswordExpiresAt:     asTime(uu.ResetPasswordExpiresAt),
		EmailVerificationExpiresAt: asTime(uu.EmailVerificationExpiresAt),
	}
}
