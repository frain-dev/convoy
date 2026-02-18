package users

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUpdateUser(t *testing.T) {
	ctx, service := setupTestDB(t)

	t.Run("should update user successfully", func(t *testing.T) {
		user := createTestUser(t, service, ctx)

		// Update user fields
		user.FirstName = "Updated"
		user.LastName = "Name"
		user.EmailVerified = true

		err := service.UpdateUser(ctx, user)
		require.NoError(t, err)

		// Verify the update
		found, err := service.FindUserByID(ctx, user.UID)
		require.NoError(t, err)
		require.Equal(t, "Updated", found.FirstName)
		require.Equal(t, "Name", found.LastName)
		require.True(t, found.EmailVerified)
	})

	t.Run("should update password reset token", func(t *testing.T) {
		user := createTestUser(t, service, ctx)

		resetToken := "new-reset-token"
		expiresAt := time.Now().Add(24 * time.Hour)

		user.ResetPasswordToken = resetToken
		user.ResetPasswordExpiresAt = expiresAt

		err := service.UpdateUser(ctx, user)
		require.NoError(t, err)

		// Verify the token was updated
		found, err := service.FindUserByToken(ctx, resetToken)
		require.NoError(t, err)
		require.Equal(t, user.UID, found.UID)
		require.Equal(t, resetToken, found.ResetPasswordToken)
	})

	t.Run("should update email verification token", func(t *testing.T) {
		user := createTestUser(t, service, ctx)

		verificationToken := "new-verification-token"
		expiresAt := time.Now().Add(48 * time.Hour)

		user.EmailVerificationToken = verificationToken
		user.EmailVerificationExpiresAt = expiresAt

		err := service.UpdateUser(ctx, user)
		require.NoError(t, err)

		// Verify the token was updated
		found, err := service.FindUserByEmailVerificationToken(ctx, verificationToken)
		require.NoError(t, err)
		require.Equal(t, user.UID, found.UID)
		require.Equal(t, verificationToken, found.EmailVerificationToken)
	})

	t.Run("should clear reset password token", func(t *testing.T) {
		user := createTestUser(t, service, ctx)

		// First set a token
		user.ResetPasswordToken = "temp-token"
		user.ResetPasswordExpiresAt = time.Now().Add(24 * time.Hour)
		err := service.UpdateUser(ctx, user)
		require.NoError(t, err)

		// Now clear it
		user.ResetPasswordToken = ""
		user.ResetPasswordExpiresAt = time.Time{}
		err = service.UpdateUser(ctx, user)
		require.NoError(t, err)

		// Verify the token was cleared
		found, err := service.FindUserByID(ctx, user.UID)
		require.NoError(t, err)
		require.Empty(t, found.ResetPasswordToken)
	})

	t.Run("should return error for non-existent user", func(t *testing.T) {
		user := createTestUser(t, service, ctx)

		// Change the UID to a non-existent one
		user.UID = "non-existent-uid"

		err := service.UpdateUser(ctx, user)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrUserNotUpdated)
	})
}
