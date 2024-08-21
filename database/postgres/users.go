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

func (u *userRepo) LoadUsersPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.User, datastore.PaginationData, error) {
	arg := map[string]interface{}{
		"limit":  pageable.Limit(),
		"cursor": pageable.Cursor(),
	}

	var query string
	if pageable.Direction == datastore.Next {
		query = fetchUsersPagedForward
	} else {
		query = fetchUsersPagedBackward
	}

	query = fmt.Sprintf(query, fetchUsersPaginated)

	query, args, err := sqlx.Named(query, arg)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query, args, err = sqlx.In(query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	query = u.db.Rebind(query)

	rows, err := u.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}
	defer closeWithError(rows)

	var users []datastore.User
	for rows.Next() {
		var user datastore.User
		err = rows.StructScan(&user)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		users = append(users, user)
	}

	var count datastore.PrevRowCount
	if len(users) > 0 {
		var countQuery string
		var qargs []interface{}
		first := users[0]
		qarg := arg
		qarg["cursor"] = first.UID

		countQuery, qargs, err = sqlx.Named(countPrevUsers, qarg)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}

		countQuery = u.db.Rebind(countQuery)

		// count the row number before the first row
		rows, err := u.db.QueryxContext(ctx, countQuery, qargs...)
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		defer closeWithError(rows)

		if rows.Next() {
			err = rows.StructScan(&count)
			if err != nil {
				return nil, datastore.PaginationData{}, err
			}
		}
	}

	ids := make([]string, len(users))
	for i := range users {
		ids[i] = users[i].UID
	}

	if len(users) > pageable.PerPage {
		users = users[:len(users)-1]
	}

	pagination := &datastore.PaginationData{PrevRowCount: count}
	pagination = pagination.Build(pageable, ids)

	return users, *pagination, nil
}

func (o *userRepo) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := o.db.GetContext(ctx, &count, countUsers)
	if err != nil {
		return 0, err
	}

	return count, nil
}
