//go:build integration
// +build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func Test_CreateUser(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	tt := []struct {
		name             string
		users            []datastore.User
		isDuplicateEmail bool
	}{
		{
			name: "create user",
			users: []datastore.User{
				{
					UID:       ulid.Make().String(),
					FirstName: "test",
					LastName:  "test",
					Email:     fmt.Sprintf("%s@test.com", ulid.Make().String()),
				},
			},
		},
		{
			name:             "cannot create user with existing email",
			isDuplicateEmail: true,
			users: []datastore.User{
				{
					UID:                        ulid.Make().String(),
					FirstName:                  "test",
					LastName:                   "test",
					Email:                      "test@test.com",
					EmailVerified:              true,
					Password:                   "dvsdvdkhjskuis",
					ResetPasswordToken:         "dvsdvdkhjskuis",
					EmailVerificationToken:     "v878678768686868",
					ResetPasswordExpiresAt:     time.Now(),
					EmailVerificationExpiresAt: time.Now(),
				},
				{
					UID:       ulid.Make().String(),
					FirstName: "test",
					LastName:  "test",
					Email:     "test@test.com",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			userRepo := NewUserRepo(db, nil)

			for i, user := range tc.users {
				if i == 0 {
					require.NoError(t, userRepo.CreateUser(context.Background(), &user))
				}

				user := &datastore.User{
					UID:       user.UID,
					FirstName: user.FirstName,
					LastName:  user.LastName,
					Email:     user.Email,
				}

				if i > 0 && tc.isDuplicateEmail {
					err := userRepo.CreateUser(context.Background(), user)
					require.Error(t, err)
					require.ErrorIs(t, err, datastore.ErrDuplicateEmail)
				}
			}
		})
	}
}

func TestCountUsers(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db, nil)
	count := 10

	for i := 0; i < count; i++ {
		u := &datastore.User{
			UID:       ulid.Make().String(),
			FirstName: "test",
			LastName:  "test",
			Email:     fmt.Sprintf("%s@test.com", ulid.Make().String()),
		}

		err := userRepo.CreateUser(context.Background(), u)
		require.NoError(t, err)
	}

	userCount, err := userRepo.CountUsers(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(count), userCount)
}

func Test_FindUserByEmail(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db, nil)

	user := generateUser(t)

	_, err := userRepo.FindUserByEmail(context.Background(), user.Email)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrUserNotFound))

	require.NoError(t, userRepo.CreateUser(context.Background(), user))

	newUser, err := userRepo.FindUserByEmail(context.Background(), user.Email)
	require.NoError(t, err)

	require.NotEmpty(t, newUser.CreatedAt)
	require.NotEmpty(t, newUser.UpdatedAt)

	newUser.CreatedAt = time.Time{}
	newUser.UpdatedAt = time.Time{}

	require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), newUser.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
	require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), newUser.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

	user.EmailVerificationExpiresAt, user.ResetPasswordExpiresAt = time.Time{}, time.Time{}
	newUser.EmailVerificationExpiresAt, newUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}

	require.Equal(t, user, newUser)
}

func Test_FindUserByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db, nil)

	user := generateUser(t)

	_, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrUserNotFound))

	require.NoError(t, userRepo.CreateUser(context.Background(), user))
	newUser, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(t, err)

	require.NotEmpty(t, newUser.CreatedAt)
	require.NotEmpty(t, newUser.UpdatedAt)

	newUser.CreatedAt = time.Time{}
	newUser.UpdatedAt = time.Time{}

	require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), newUser.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
	require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), newUser.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

	user.EmailVerificationExpiresAt, user.ResetPasswordExpiresAt = time.Time{}, time.Time{}
	newUser.EmailVerificationExpiresAt, newUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}

	require.Equal(t, user, newUser)
}

func Test_FindUserByToken(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db, nil)

	user := generateUser(t)
	token := "fd7fidyfhdjhfdjhfjdh"

	user.ResetPasswordToken = token

	require.NoError(t, userRepo.CreateUser(context.Background(), user))
	newUser, err := userRepo.FindUserByToken(context.Background(), token)
	require.NoError(t, err)

	require.NotEmpty(t, newUser.CreatedAt)
	require.NotEmpty(t, newUser.UpdatedAt)

	newUser.CreatedAt = time.Time{}
	newUser.UpdatedAt = time.Time{}

	require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), newUser.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
	require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), newUser.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

	user.EmailVerificationExpiresAt, user.ResetPasswordExpiresAt = time.Time{}, time.Time{}
	newUser.EmailVerificationExpiresAt, newUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}

	require.Equal(t, user, newUser)
}

func Test_FindUserByEmailVerificationToken(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db, nil)

	user := generateUser(t)
	token := "fd7fidyfhdjhfdjhfjdh"

	user.EmailVerificationToken = token

	require.NoError(t, userRepo.CreateUser(context.Background(), user))
	newUser, err := userRepo.FindUserByEmailVerificationToken(context.Background(), token)
	require.NoError(t, err)

	require.NotEmpty(t, newUser.CreatedAt)
	require.NotEmpty(t, newUser.UpdatedAt)

	newUser.CreatedAt = time.Time{}
	newUser.UpdatedAt = time.Time{}

	require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), newUser.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
	require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), newUser.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

	user.EmailVerificationExpiresAt, user.ResetPasswordExpiresAt = time.Time{}, time.Time{}
	newUser.EmailVerificationExpiresAt, newUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}

	require.Equal(t, user, newUser)
}

func Test_UpdateUser(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db, nil)
	user := generateUser(t)

	require.NoError(t, userRepo.CreateUser(context.Background(), user))

	updatedUser := &datastore.User{
		UID:                        user.UID,
		FirstName:                  fmt.Sprintf("test%s", ulid.Make().String()),
		LastName:                   fmt.Sprintf("test%s", ulid.Make().String()),
		Email:                      fmt.Sprintf("%s@test.com", ulid.Make().String()),
		EmailVerified:              !user.EmailVerified,
		Password:                   ulid.Make().String(),
		ResetPasswordToken:         fmt.Sprintf("%s-reset-password-token", ulid.Make().String()),
		EmailVerificationToken:     fmt.Sprintf("%s-verification-token", ulid.Make().String()),
		ResetPasswordExpiresAt:     time.Now().Add(time.Hour).UTC(),
		EmailVerificationExpiresAt: time.Now().Add(time.Hour).UTC(),
	}

	require.NoError(t, userRepo.UpdateUser(context.Background(), updatedUser))

	dbUser, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(t, err)

	require.Equal(t, dbUser.UID, updatedUser.UID)

	dbUser.CreatedAt = time.Time{}
	dbUser.UpdatedAt = time.Time{}

	require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), dbUser.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
	require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), dbUser.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

	updatedUser.EmailVerificationExpiresAt, updatedUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}
	dbUser.EmailVerificationExpiresAt, dbUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}

	require.Equal(t, updatedUser, dbUser)
}

func generateUser(t *testing.T) *datastore.User {
	return &datastore.User{
		UID:                        ulid.Make().String(),
		FirstName:                  "test",
		LastName:                   "test",
		Email:                      fmt.Sprintf("%s@test.com", ulid.Make().String()),
		EmailVerified:              true,
		Password:                   "dvsdvdkhjskuis",
		ResetPasswordToken:         "dvsdvdkhjskuis",
		EmailVerificationToken:     "v878678768686868",
		ResetPasswordExpiresAt:     time.Now(),
		EmailVerificationExpiresAt: time.Now(),
	}
}
