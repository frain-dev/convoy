package users

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCountUsers(t *testing.T) {
	ctx, service := setupTestDB(t)

	t.Run("should return zero for empty database", func(t *testing.T) {
		count, err := service.CountUsers(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(0), count)
	})

	t.Run("should count users correctly", func(t *testing.T) {
		// Create first user
		createTestUser(t, service, ctx)

		count, err := service.CountUsers(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(1), count)

		// Create second user
		createTestUser(t, service, ctx)

		count, err = service.CountUsers(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(2), count)

		// Create third user
		createTestUser(t, service, ctx)

		count, err = service.CountUsers(ctx)
		require.NoError(t, err)
		require.Equal(t, int64(3), count)
	})
}
