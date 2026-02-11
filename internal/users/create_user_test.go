package users

import (
	"fmt"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestCreateUser(t *testing.T) {
	ctx, service := setupTestDB(t)

	t.Run("should create user successfully", func(t *testing.T) {
		uid := ulid.Make().String()
		user := &datastore.User{
			UID:           uid,
			FirstName:     "John",
			LastName:      "Doe",
			Email:         fmt.Sprintf("john.doe-%s@example.com", uid),
			Password:      "hashedpassword123",
			EmailVerified: false,
			AuthType:      "local",
		}

		err := service.CreateUser(ctx, user)
		require.NoError(t, err)

		// Verify the user was created by fetching it
		found, err := service.FindUserByID(ctx, uid)
		require.NoError(t, err)
		require.Equal(t, user.UID, found.UID)
		require.Equal(t, user.FirstName, found.FirstName)
		require.Equal(t, user.LastName, found.LastName)
		require.Equal(t, user.Email, found.Email)
	})

	t.Run("should return error for duplicate email", func(t *testing.T) {
		uid1 := ulid.Make().String()
		email := fmt.Sprintf("duplicate-%s@example.com", uid1)

		user1 := &datastore.User{
			UID:       uid1,
			FirstName: "First",
			LastName:  "User",
			Email:     email,
			Password:  "password1",
			AuthType:  "local",
		}

		err := service.CreateUser(ctx, user1)
		require.NoError(t, err)

		// Try to create another user with the same email
		uid2 := ulid.Make().String()
		user2 := &datastore.User{
			UID:       uid2,
			FirstName: "Second",
			LastName:  "User",
			Email:     email, // Same email
			Password:  "password2",
			AuthType:  "local",
		}

		err = service.CreateUser(ctx, user2)
		require.Error(t, err)
		require.ErrorIs(t, err, datastore.ErrDuplicateEmail)
	})

	t.Run("should create user with reset password token", func(t *testing.T) {
		uid := ulid.Make().String()
		resetToken := "reset-token-123"
		expiresAt := time.Now().Add(24 * time.Hour)

		user := &datastore.User{
			UID:                    uid,
			FirstName:              "Reset",
			LastName:               "User",
			Email:                  fmt.Sprintf("reset-%s@example.com", uid),
			Password:               "hashedpassword",
			ResetPasswordToken:     resetToken,
			ResetPasswordExpiresAt: expiresAt,
			AuthType:               "local",
		}

		err := service.CreateUser(ctx, user)
		require.NoError(t, err)

		// Verify the reset token was saved
		found, err := service.FindUserByToken(ctx, resetToken)
		require.NoError(t, err)
		require.Equal(t, user.UID, found.UID)
		require.Equal(t, resetToken, found.ResetPasswordToken)
	})

	t.Run("should create user with email verification token", func(t *testing.T) {
		uid := ulid.Make().String()
		verificationToken := "verification-token-456"
		expiresAt := time.Now().Add(48 * time.Hour)

		user := &datastore.User{
			UID:                        uid,
			FirstName:                  "Verify",
			LastName:                   "User",
			Email:                      fmt.Sprintf("verify-%s@example.com", uid),
			Password:                   "hashedpassword",
			EmailVerificationToken:     verificationToken,
			EmailVerificationExpiresAt: expiresAt,
			AuthType:                   "local",
		}

		err := service.CreateUser(ctx, user)
		require.NoError(t, err)

		// Verify the verification token was saved
		found, err := service.FindUserByEmailVerificationToken(ctx, verificationToken)
		require.NoError(t, err)
		require.Equal(t, user.UID, found.UID)
		require.Equal(t, verificationToken, found.EmailVerificationToken)
	})
}
