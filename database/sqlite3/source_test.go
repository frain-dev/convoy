//go:build integration
// +build integration

package sqlite3

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestSourceRepo(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T, sourceRepo datastore.SourceRepository, source *datastore.Source)
	}{
		{
			name: "Create Source",
			testFunc: func(t *testing.T, sourceRepo datastore.SourceRepository, source *datastore.Source) {
				require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

				newSource, err := sourceRepo.FindSourceByID(context.Background(), source.ProjectID, source.UID)
				require.NoError(t, err)

				newSource.CreatedAt = time.Time{}
				newSource.UpdatedAt = time.Time{}

				require.Equal(t, source, newSource)
			},
		},
		{
			name: "Find Source By ID",
			testFunc: func(t *testing.T, sourceRepo datastore.SourceRepository, source *datastore.Source) {
				_, err := sourceRepo.FindSourceByID(context.Background(), source.ProjectID, source.UID)
				require.Error(t, err)
				require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

				require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

				newSource, err := sourceRepo.FindSourceByID(context.Background(), source.ProjectID, source.UID)
				require.NoError(t, err)

				newSource.CreatedAt = time.Time{}
				newSource.UpdatedAt = time.Time{}

				require.Equal(t, source, newSource)
			},
		},
		{
			name: "Find Source By Name",
			testFunc: func(t *testing.T, sourceRepo datastore.SourceRepository, source *datastore.Source) {
				_, err := sourceRepo.FindSourceByName(context.Background(), source.ProjectID, source.Name)
				require.Error(t, err)
				require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

				require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

				newSource, err := sourceRepo.FindSourceByName(context.Background(), source.ProjectID, source.Name)
				require.NoError(t, err)

				newSource.CreatedAt = time.Time{}
				newSource.UpdatedAt = time.Time{}

				require.Equal(t, source, newSource)
			},
		},
		{
			name: "Find Source By Mask ID",
			testFunc: func(t *testing.T, sourceRepo datastore.SourceRepository, source *datastore.Source) {
				_, err := sourceRepo.FindSourceByMaskID(context.Background(), source.MaskID)
				require.Error(t, err)
				require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

				require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

				newSource, err := sourceRepo.FindSourceByMaskID(context.Background(), source.MaskID)
				require.NoError(t, err)

				newSource.CreatedAt = time.Time{}
				newSource.UpdatedAt = time.Time{}

				require.Equal(t, source, newSource)
			},
		},
		{
			name: "Update Source",
			testFunc: func(t *testing.T, sourceRepo datastore.SourceRepository, source *datastore.Source) {
				require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

				name := "Convoy-Dev"
				source.Name = name
				source.IsDisabled = true
				source.CustomResponse = datastore.CustomResponse{
					Body:        "/ref/",
					ContentType: "application/json",
				}
				require.NoError(t, sourceRepo.UpdateSource(context.Background(), source.ProjectID, source))

				newSource, err := sourceRepo.FindSourceByID(context.Background(), source.ProjectID, source.UID)
				require.NoError(t, err)

				newSource.CreatedAt = time.Time{}
				newSource.UpdatedAt = time.Time{}

				require.Equal(t, source, newSource)
			},
		},
		{
			name: "Delete Source",
			testFunc: func(t *testing.T, sourceRepo datastore.SourceRepository, source *datastore.Source) {
				db, _ := getDB(t)

				subRepo := NewSubscriptionRepo(db)
				require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

				project := seedProject(t, db)
				endpoint := seedEndpoint(t, db)

				sub := generateSubscription(project, source, endpoint, &datastore.Device{})
				require.NoError(t, subRepo.CreateSubscription(context.Background(), sub.ProjectID, sub))

				_, err := sourceRepo.FindSourceByID(context.Background(), source.ProjectID, source.UID)
				require.NoError(t, err)

				require.NoError(t, sourceRepo.DeleteSourceByID(context.Background(), source.ProjectID, source.UID, source.VerifierID))

				_, err = sourceRepo.FindSourceByID(context.Background(), source.ProjectID, source.UID)
				require.Error(t, err)
				require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

				_, err = subRepo.FindSubscriptionByID(context.Background(), source.ProjectID, sub.UID)
				require.Error(t, err)
				require.True(t, errors.Is(err, datastore.ErrSubscriptionNotFound))
			},
		},
		{
			name: "Load Sources Paged",
			testFunc: func(t *testing.T, sourceRepo datastore.SourceRepository, source *datastore.Source) {
				pagingTests := []struct {
					name     string
					pageData datastore.Pageable
					count    int
					perPage  int64
				}{
					{
						name:     "10 records - 3 per page",
						pageData: datastore.Pageable{PerPage: 3},
						count:    10,
						perPage:  3,
					},
					{
						name:     "12 records - 4 per page",
						pageData: datastore.Pageable{PerPage: 4},
						count:    12,
						perPage:  4,
					},
					{
						name:     "5 records - 3 per page",
						pageData: datastore.Pageable{PerPage: 3},
						count:    5,
						perPage:  3,
					},
				}

				for _, pt := range pagingTests {
					t.Run(pt.name, func(t *testing.T) {
						for i := 0; i < pt.count; i++ {
							s := &datastore.Source{
								UID:       ulid.Make().String(),
								ProjectID: source.ProjectID,
								Name:      "Convoy-Prod",
								MaskID:    uniuri.NewLen(16),
								Type:      datastore.HTTPSource,
								Verifier: &datastore.VerifierConfig{
									Type: datastore.HMacVerifier,
									HMac: &datastore.HMac{
										Header: "X-Paystack-Signature",
										Hash:   "SHA512",
										Secret: "Paystack Secret",
									},
								},
							}
							require.NoError(t, sourceRepo.CreateSource(context.Background(), s))
						}

						_, pageable, err := sourceRepo.LoadSourcesPaged(context.Background(), source.ProjectID, &datastore.SourceFilter{}, pt.pageData)
						require.NoError(t, err)
						require.Equal(t, pt.perPage, pageable.PerPage)
					})
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			repo := NewSourceRepo(db)
			source := generateSource(t, db)

			tc.testFunc(t, repo, source)
		})
	}
}

func generateSource(t *testing.T, db database.Database) *datastore.Source {
	project := seedProject(t, db)

	return &datastore.Source{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      "Convoy-Prod",
		MaskID:    uniuri.NewLen(16),
		CustomResponse: datastore.CustomResponse{
			Body:        "/dover/",
			ContentType: "text/plain",
		},
		Type: datastore.HTTPSource,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.HMacVerifier,
			HMac: &datastore.HMac{
				Header: "X-Paystack-Signature",
				Hash:   "SHA512",
				Secret: "Paystack Secret",
			},
			ApiKey:    &datastore.ApiKey{},
			BasicAuth: &datastore.BasicAuth{},
		},
	}
}

func seedSource(t *testing.T, db database.Database) *datastore.Source {
	source := generateSource(t, db)

	require.NoError(t, NewSourceRepo(db).CreateSource(context.Background(), source))
	return source
}
