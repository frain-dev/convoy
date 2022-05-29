//go:build integration
// +build integration

package mongo

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_CreateUser(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db)
	user := &datastore.User{
		UID:            uuid.NewString(),
		FirstName:      "test",
		LastName:       "test",
		Email:          "test@test.com",
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.NoError(t, userRepo.CreateUser(context.Background(), user))
	newUser, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(t, err)

	require.Equal(t, user.UID, newUser.UID)
	require.Equal(t, user.FirstName, newUser.FirstName)
	require.Equal(t, user.LastName, newUser.LastName)
}
