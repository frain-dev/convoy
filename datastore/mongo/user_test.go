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

	store := getStore(db)

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
			name: "cannot create user with existing email",
			users: []datastore.User{
				{
					UID:       ulid.Make().String(),
					FirstName: "test",
					LastName:  "test",
					Email:     "test@test.com",
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
			userRepo := NewUserRepo(store)

			for i, user := range tc.users {
				user := &datastore.User{
					UID:       user.UID,
					FirstName: user.FirstName,
					LastName:  user.LastName,
					Email:     user.Email,
				}

				if i == 0 {
					require.NoError(t, userRepo.CreateUser(context.Background(), user))
					newUser, err := userRepo.FindUserByID(context.Background(), user.UID)
					require.NoError(t, err)

					require.Equal(t, user.UID, newUser.UID)
					require.Equal(t, user.FirstName, newUser.FirstName)
					require.Equal(t, user.LastName, newUser.LastName)
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

func Test_FindUserByEmail(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	userRepo := NewUserRepo(store)
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

	store := getStore(db)
	userRepo := NewUserRepo(store)
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

			store := getStore(db)
			userRepo := NewUserRepo(store)
			for i := 0; i < tc.count; i++ {
				user := &datastore.User{
					UID:       ulid.Make().String(),
					FirstName: "test",
					LastName:  "test",
					Email:     fmt.Sprintf("%s@test.com", ulid.Make().String()),
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

	store := getStore(db)
	userRepo := NewUserRepo(store)
	user := generateUser(t)

	require.NoError(t, userRepo.CreateUser(context.Background(), user))

	firstName := fmt.Sprintf("test%s", ulid.Make().String())
	lastName := fmt.Sprintf("test%s", ulid.Make().String())
	email := fmt.Sprintf("%s@test.com", ulid.Make().String())

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
		UID:       ulid.Make().String(),
		FirstName: "test",
		LastName:  "test",
		Email:     fmt.Sprintf("%s@test.com", ulid.Make().String()),
	}
}
