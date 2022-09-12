//go:build integration
// +build integration

package mongo

import (
	"context"
	"errors"
	"testing"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_CreateSource(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	sourceRepo := NewSourceRepo(store)
	source := generateSource(t)

	sourceCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SourceCollection)

	require.NoError(t, sourceRepo.CreateSource(sourceCtx, source))

	newSource, err := sourceRepo.FindSourceByID(sourceCtx, source.GroupID, source.UID)
	require.NoError(t, err)

	require.Equal(t, source.UID, newSource.UID)
	require.Equal(t, source.Name, newSource.Name)
	require.Equal(t, source.Verifier.HMac.Secret, newSource.Verifier.HMac.Secret)
	require.Equal(t, source.MaskID, newSource.MaskID)
}

func Test_FindSourceByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	sourceRepo := NewSourceRepo(store)
	source := generateSource(t)

	sourceCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SourceCollection)

	_, err := sourceRepo.FindSourceByID(sourceCtx, source.GroupID, source.UID)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

	require.NoError(t, sourceRepo.CreateSource(sourceCtx, source))

	newSource, err := sourceRepo.FindSourceByID(sourceCtx, source.GroupID, source.UID)
	require.NoError(t, err)

	require.Equal(t, source.UID, newSource.UID)
}

func Test_FindSourceByMaskID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	sourceRepo := NewSourceRepo(store)
	source := generateSource(t)

	sourceCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SourceCollection)

	_, err := sourceRepo.FindSourceByMaskID(sourceCtx, source.MaskID)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

	require.NoError(t, sourceRepo.CreateSource(sourceCtx, source))

	newSource, err := sourceRepo.FindSourceByMaskID(sourceCtx, source.MaskID)
	require.NoError(t, err)

	require.Equal(t, source.MaskID, newSource.MaskID)
}

func Test_UpdateSource(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	sourceRepo := NewSourceRepo(store)
	source := generateSource(t)

	sourceCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SourceCollection)

	require.NoError(t, sourceRepo.CreateSource(sourceCtx, source))

	name := "Convoy-Dev"
	source.Name = name

	require.NoError(t, sourceRepo.UpdateSource(sourceCtx, source.GroupID, source))

	newSource, err := sourceRepo.FindSourceByID(sourceCtx, source.GroupID, source.UID)
	require.NoError(t, err)

	require.Equal(t, name, newSource.Name)
}

func Test_DeleteSource(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	store := getStore(db)
	sourceRepo := NewSourceRepo(store)
	source := generateSource(t)

	sourceCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SourceCollection)

	require.NoError(t, sourceRepo.CreateSource(sourceCtx, source))

	_, err := sourceRepo.FindSourceByID(sourceCtx, source.GroupID, source.UID)
	require.NoError(t, err)

	require.NoError(t, sourceRepo.DeleteSourceByID(sourceCtx, source.GroupID, source.UID))

	_, err = sourceRepo.FindSourceByID(sourceCtx, source.GroupID, source.UID)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrSourceNotFound))
}

func Test_LoadSourcesPaged(t *testing.T) {
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
			name:     "Load Sources Paged - 10 records",
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
			name:     "Load Sources Paged - 12 records",
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
			name:     "Load Sources Paged - 5 records",
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
			sourceRepo := NewSourceRepo(store)
			groupId := uuid.NewString()

			sourceCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.SourceCollection)

			for i := 0; i < tc.count; i++ {
				source := &datastore.Source{
					UID:     uuid.NewString(),
					GroupID: groupId,
					Name:    "Convoy-Prod",
					MaskID:  uniuri.NewLen(16),
					Type:    datastore.HTTPSource,
					Verifier: &datastore.VerifierConfig{
						Type: datastore.HMacVerifier,
						HMac: &datastore.HMac{
							Header: "X-Paystack-Signature",
							Hash:   "SHA512",
							Secret: "Paystack Secret",
						},
					},
					DocumentStatus: datastore.ActiveDocumentStatus,
				}
				require.NoError(t, sourceRepo.CreateSource(sourceCtx, source))
			}

			_, pageable, err := sourceRepo.LoadSourcesPaged(sourceCtx, groupId, &datastore.SourceFilter{}, tc.pageData)

			require.NoError(t, err)

			require.Equal(t, tc.expected.paginationData.Total, pageable.Total)
			require.Equal(t, tc.expected.paginationData.TotalPage, pageable.TotalPage)
			require.Equal(t, tc.expected.paginationData.Page, pageable.Page)
			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
			require.Equal(t, tc.expected.paginationData.Prev, pageable.Prev)
			require.Equal(t, tc.expected.paginationData.Next, pageable.Next)
		})
	}
}

func generateSource(t *testing.T) *datastore.Source {
	return &datastore.Source{
		UID:     uuid.NewString(),
		GroupID: uuid.NewString(),
		Name:    "Convoy-Prod",
		MaskID:  uniuri.NewLen(16),
		Type:    datastore.HTTPSource,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.HMacVerifier,
			HMac: &datastore.HMac{
				Header: "X-Paystack-Signature",
				Hash:   "SHA512",
				Secret: "Paystack Secret",
			},
		},
		DocumentStatus: datastore.ActiveDocumentStatus,
	}
}
