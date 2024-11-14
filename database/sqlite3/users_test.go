//go:build integration
// +build integration

package sqlite3

import (
	"context"
	"errors"
	"fmt"
	"gopkg.in/guregu/null.v4"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestUsers(t *testing.T) {
	t.Run("Test_CreateUser", func(t *testing.T) {
		db, closeFn := getDB(t)
		defer closeFn()

		tt := []struct {
			name             string
			user             datastore.User
			fn               func(datastore.UserRepository)
			isDuplicateEmail bool
		}{
			{
				name: "create user",
				user: datastore.User{
					UID:       ulid.Make().String(),
					FirstName: "test",
					LastName:  "test",
					Email:     fmt.Sprintf("%s@test.com", ulid.Make().String()),
				},
			},
			{
				name:             "cannot create user with existing email",
				isDuplicateEmail: true,
				fn: func(ur datastore.UserRepository) {
					err := ur.CreateUser(context.Background(), &datastore.User{
						UID:       ulid.Make().String(),
						FirstName: "test",
						LastName:  "test",
						Email:     "test@test.com",
					})
					require.NoError(t, err)
				},
				user: datastore.User{
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
			},
		}

		for _, tc := range tt {
			t.Run(tc.name, func(t *testing.T) {
				repo := NewUserRepo(db)

				if tc.fn != nil {
					tc.fn(repo)
				}

				u := &datastore.User{
					UID:       tc.user.UID,
					FirstName: tc.user.FirstName,
					LastName:  tc.user.LastName,
					Email:     tc.user.Email,
				}

				if tc.isDuplicateEmail {
					err := repo.CreateUser(context.Background(), u)
					require.Error(t, err)
					require.ErrorIs(t, err, datastore.ErrDuplicateEmail)
				}
			})
		}
	})

	t.Run("TestCountUsers", func(t *testing.T) {
		db, closeFn := getDB(t)
		defer closeFn()

		repo := NewUserRepo(db)
		c := 10

		for i := 0; i < c; i++ {
			u := &datastore.User{
				UID:       ulid.Make().String(),
				FirstName: "test",
				LastName:  "test",
				Email:     fmt.Sprintf("%s@test.com", ulid.Make().String()),
			}

			err := repo.CreateUser(context.Background(), u)
			require.NoError(t, err)
		}

		userCount, err := repo.CountUsers(context.Background())

		require.NoError(t, err)
		require.Equal(t, int64(c), userCount)
	})

	t.Run("Test_FindUserByEmail", func(t *testing.T) {
		db, closeFn := getDB(t)
		defer closeFn()

		repo := NewUserRepo(db)

		user := generateUser(t)

		_, err := repo.FindUserByEmail(context.Background(), user.Email)
		require.Error(t, err)
		require.True(t, errors.Is(err, datastore.ErrUserNotFound))

		require.NoError(t, repo.CreateUser(context.Background(), user))

		newUser, err := repo.FindUserByEmail(context.Background(), user.Email)
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
	})

	t.Run("Test_FindUserByID", func(t *testing.T) {
		db, closeFn := getDB(t)
		defer closeFn()

		repo := NewUserRepo(db)

		user := generateUser(t)

		_, err := repo.FindUserByID(context.Background(), user.UID)
		require.Error(t, err)
		require.True(t, errors.Is(err, datastore.ErrUserNotFound))

		require.NoError(t, repo.CreateUser(context.Background(), user))
		newUser, err := repo.FindUserByID(context.Background(), user.UID)
		require.NoError(t, err)

		require.NotEmpty(t, newUser.CreatedAt)
		require.NotEmpty(t, newUser.UpdatedAt)

		newUser.CreatedAt = time.Time{}
		newUser.UpdatedAt = time.Time{}
		newUser.DeletedAt = null.NewTime(time.Time{}, false)

		require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), newUser.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
		require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), newUser.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

		user.EmailVerificationExpiresAt, user.ResetPasswordExpiresAt = time.Time{}, time.Time{}
		newUser.EmailVerificationExpiresAt, newUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}

		require.Equal(t, user, newUser)
	})

	t.Run("Test_FindUserByToken", func(t *testing.T) {
		db, closeFn := getDB(t)
		defer closeFn()

		repo := NewUserRepo(db)

		user := generateUser(t)
		token := "fd7fidyfhdjhfdjhfjdh"

		user.ResetPasswordToken = token

		require.NoError(t, repo.CreateUser(context.Background(), user))
		newUser, err := repo.FindUserByToken(context.Background(), token)
		require.NoError(t, err)

		require.NotEmpty(t, newUser.CreatedAt)
		require.NotEmpty(t, newUser.UpdatedAt)

		newUser.CreatedAt = time.Time{}
		newUser.UpdatedAt = time.Time{}
		newUser.DeletedAt = null.NewTime(time.Time{}, false)

		require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), newUser.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
		require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), newUser.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

		user.EmailVerificationExpiresAt, user.ResetPasswordExpiresAt = time.Time{}, time.Time{}
		newUser.EmailVerificationExpiresAt, newUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}

		require.Equal(t, user, newUser)
	})

	t.Run("Test_FindUserByEmailVerificationToken", func(t *testing.T) {
		db, closeFn := getDB(t)
		defer closeFn()

		repo := NewUserRepo(db)

		user := generateUser(t)
		token := "fd7fidyfhdjhfdjhfjdh"

		user.EmailVerificationToken = token

		require.NoError(t, repo.CreateUser(context.Background(), user))
		newUser, err := repo.FindUserByEmailVerificationToken(context.Background(), token)
		require.NoError(t, err)

		require.NotEmpty(t, newUser.CreatedAt)
		require.NotEmpty(t, newUser.UpdatedAt)

		newUser.CreatedAt = time.Time{}
		newUser.UpdatedAt = time.Time{}
		newUser.DeletedAt = null.NewTime(time.Time{}, false)

		require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), newUser.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
		require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), newUser.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

		user.EmailVerificationExpiresAt, user.ResetPasswordExpiresAt = time.Time{}, time.Time{}
		newUser.EmailVerificationExpiresAt, newUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}

		require.Equal(t, user, newUser)
	})

	t.Run("Test_UpdateUser", func(t *testing.T) {
		db, closeFn := getDB(t)
		defer closeFn()

		repo := NewUserRepo(db)
		user := generateUser(t)

		require.NoError(t, repo.CreateUser(context.Background(), user))

		updatedUser := &datastore.User{
			UID:                        user.UID,
			FirstName:                  fmt.Sprintf("test%s", ulid.Make().String()),
			LastName:                   fmt.Sprintf("test%s", ulid.Make().String()),
			Email:                      user.Email,
			EmailVerified:              !user.EmailVerified,
			Password:                   ulid.Make().String(),
			ResetPasswordToken:         fmt.Sprintf("%s-reset-password-token", ulid.Make().String()),
			EmailVerificationToken:     fmt.Sprintf("%s-verification-token", ulid.Make().String()),
			ResetPasswordExpiresAt:     time.Now().Add(time.Hour).UTC(),
			EmailVerificationExpiresAt: time.Now().Add(time.Hour).UTC(),
		}

		require.NoError(t, repo.UpdateUser(context.Background(), updatedUser))

		userByID, err := repo.FindUserByID(context.Background(), user.UID)
		require.NoError(t, err)

		require.Equal(t, userByID.UID, updatedUser.UID)

		userByID.CreatedAt = time.Time{}
		userByID.UpdatedAt = time.Time{}
		userByID.DeletedAt = null.NewTime(time.Time{}, false)

		require.InDelta(t, user.EmailVerificationExpiresAt.Unix(), userByID.EmailVerificationExpiresAt.Unix(), float64(time.Hour))
		require.InDelta(t, user.ResetPasswordExpiresAt.Unix(), userByID.ResetPasswordExpiresAt.Unix(), float64(time.Hour))

		updatedUser.EmailVerificationExpiresAt, updatedUser.ResetPasswordExpiresAt = time.Time{}, time.Time{}
		userByID.EmailVerificationExpiresAt, userByID.ResetPasswordExpiresAt = time.Time{}, time.Time{}

		require.Equal(t, updatedUser, userByID)
	})
}

func generateUser(t *testing.T) *datastore.User {
	t.Helper()
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
