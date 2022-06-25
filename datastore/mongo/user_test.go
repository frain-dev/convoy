//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_CreateUser(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db)
	user := generateUser(t)

	require.NoError(t, userRepo.CreateUser(context.Background(), user))
	newUser, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(t, err)

	require.Equal(t, user.UID, newUser.UID)
	require.Equal(t, user.FirstName, newUser.FirstName)
	require.Equal(t, user.LastName, newUser.LastName)
}

func Test_FindUserByEmail(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db)
	user := generateUser(t)

	_, err := userRepo.FindUserByEmail(context.Background(), user.Email)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrUserNotFound))

	require.NoError(t, userRepo.CreateUser(context.Background(), user))

	newUser, err := userRepo.FindUserByEmail(context.Background(), user.Email)
	require.NoError(t, err)

	require.Equal(t, user.UID, newUser.UID)
	require.Equal(t, user.FirstName, newUser.FirstName)
	require.Equal(t, user.Email, newUser.Email)
}

func Test_FindUserByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db)
	user := generateUser(t)

	_, err := userRepo.FindUserByID(context.Background(), user.UID)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrUserNotFound))

	require.NoError(t, userRepo.CreateUser(context.Background(), user))

	newUser, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(t, err)

	require.Equal(t, user.UID, newUser.UID)
	require.Equal(t, user.FirstName, newUser.FirstName)
	require.Equal(t, user.Email, newUser.Email)
}

func Test_LoadUsersPaged(t *testing.T) {
	type Expected struct {
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		pageData datastore.Pageable
		count    int
		expected Expected
	}{
		{
			name:     "Load Users Paged - 10 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			count:    10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     10,
					TotalPage: 4,
					Page:      1,
					PerPage:   3,
					Prev:      0,
					Next:      2,
				},
			},
		},

		{
			name:     "Load Users Paged - 12 records",
			pageData: datastore.Pageable{Page: 2, PerPage: 4},
			count:    12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     12,
					TotalPage: 3,
					Page:      2,
					PerPage:   4,
					Prev:      1,
					Next:      3,
				},
			},
		},

		{
			name:     "Load Users Paged - 5 records",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			count:    5,
			expected: Expected{
				paginationData: datastore.PaginationData{
					Total:     5,
					TotalPage: 2,
					Page:      1,
					PerPage:   3,
					Prev:      0,
					Next:      2,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			userRepo := NewUserRepo(db)
			for i := 0; i < tc.count; i++ {
				user := &datastore.User{
					UID:            uuid.NewString(),
					FirstName:      "test",
					LastName:       "test",
					Email:          fmt.Sprintf("%s@test.com", uuid.NewString()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				}
				require.NoError(t, userRepo.CreateUser(context.Background(), user))
			}

			_, pageable, err := userRepo.LoadUsersPaged(context.Background(), tc.pageData)

			require.NoError(t, err)
			require.Equal(t, tc.expected.paginationData.Page, pageable.Page)
			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
			require.Equal(t, tc.expected.paginationData.Prev, pageable.Prev)
			require.Equal(t, tc.expected.paginationData.Next, pageable.Next)
		})
	}
}

func Test_UpdateUser(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	userRepo := NewUserRepo(db)
	user := generateUser(t)

	require.NoError(t, userRepo.CreateUser(context.Background(), user))

	firstName := fmt.Sprintf("test%s", uuid.NewString())
	lastName := fmt.Sprintf("test%s", uuid.NewString())
	email := fmt.Sprintf("%s@test.com", uuid.NewString())

	user.FirstName = firstName
	user.LastName = lastName
	user.Email = email

	require.NoError(t, userRepo.UpdateUser(context.Background(), user))

	newUser, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(t, err)

	require.Equal(t, firstName, newUser.FirstName)
	require.Equal(t, lastName, newUser.LastName)
	require.Equal(t, email, newUser.Email)

}

func generateUser(t *testing.T) *datastore.User {
	return &datastore.User{
		UID:            uuid.NewString(),
		FirstName:      "test",
		LastName:       "test",
		Email:          fmt.Sprintf("%s@test.com", uuid.NewString()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}
}
