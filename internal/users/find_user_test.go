package users

import (
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestFindUserByID(t *testing.T) {
	ctx, service := setupTestDB(t)

	t.Run("should find user by ID", func(t *testing.T) {
		user := createTestUser(t, service, ctx)

		found, err := service.FindUserByID(ctx, user.UID)
		require.NoError(t, err)
		assertUserEqual(t, user, found)
	})

	t.Run("should return error for non-existent ID", func(t *testing.T) {
		_, err := service.FindUserByID(ctx, "non-existent-id")
		require.Error(t, err)
		require.ErrorIs(t, err, datastore.ErrUserNotFound)
	})
}

func TestFindUserByEmail(t *testing.T) {
	ctx, service := setupTestDB(t)

	t.Run("should find user by email", func(t *testing.T) {
		user := createTestUser(t, service, ctx)

		found, err := service.FindUserByEmail(ctx, user.Email)
		require.NoError(t, err)
		assertUserEqual(t, user, found)
	})

	t.Run("should return error for non-existent email", func(t *testing.T) {
		_, err := service.FindUserByEmail(ctx, "nonexistent@example.com")
		require.Error(t, err)
		require.ErrorIs(t, err, datastore.ErrUserNotFound)
	})
}

func TestFindUserByToken(t *testing.T) {
	ctx, service := setupTestDB(t)

	t.Run("should find user by reset password token", func(t *testing.T) {
		uid := ulid.Make().String()
		resetToken := fmt.Sprintf("reset-token-%s", uid)

		user := &datastore.User{
			UID:                    uid,
			FirstName:              "Token",
			LastName:               "User",
			Email:                  fmt.Sprintf("token-%s@example.com", uid),
			Password:               "hashedpassword",
			ResetPasswordToken:     resetToken,
			ResetPasswordExpiresAt: time.Now().Add(24 * time.Hour),
			AuthType:               "local",
		}

		err := service.CreateUser(ctx, user)
		require.NoError(t, err)

		found, err := service.FindUserByToken(ctx, resetToken)
		require.NoError(t, err)
		require.Equal(t, user.UID, found.UID)
		require.Equal(t, resetToken, found.ResetPasswordToken)
	})

	t.Run("should return error for non-existent token", func(t *testing.T) {
		_, err := service.FindUserByToken(ctx, "non-existent-token")
		require.Error(t, err)
		require.ErrorIs(t, err, datastore.ErrUserNotFound)
	})
}

func TestFindUserByEmailVerificationToken(t *testing.T) {
	ctx, service := setupTestDB(t)

	t.Run("should find user by email verification token", func(t *testing.T) {
		uid := ulid.Make().String()
		verificationToken := fmt.Sprintf("verification-token-%s", uid)

		user := &datastore.User{
			UID:                        uid,
			FirstName:                  "Verify",
			LastName:                   "User",
			Email:                      fmt.Sprintf("verify-%s@example.com", uid),
			Password:                   "hashedpassword",
			EmailVerificationToken:     verificationToken,
			EmailVerificationExpiresAt: time.Now().Add(48 * time.Hour),
			AuthType:                   "local",
		}

		err := service.CreateUser(ctx, user)
		require.NoError(t, err)

		found, err := service.FindUserByEmailVerificationToken(ctx, verificationToken)
		require.NoError(t, err)
		require.Equal(t, user.UID, found.UID)
		require.Equal(t, verificationToken, found.EmailVerificationToken)
	})

	t.Run("should return error for non-existent verification token", func(t *testing.T) {
		_, err := service.FindUserByEmailVerificationToken(ctx, "non-existent-token")
		require.Error(t, err)
		require.ErrorIs(t, err, datastore.ErrUserNotFound)
	})
}
