//go:build integration
// +build integration

package postgres

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

func Test_CreateSource(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	sourceRepo := NewSourceRepo(db, nil)
	source := generateSource(t, db)

	require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

	newSource, err := sourceRepo.FindSourceByID(context.Background(), source.UID)
	require.NoError(t, err)

	newSource.CreatedAt = time.Time{}
	newSource.UpdatedAt = time.Time{}

	require.Equal(t, source, newSource)
}

func Test_FindSourceByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	sourceRepo := NewSourceRepo(db, nil)
	source := generateSource(t, db)

	_, err := sourceRepo.FindSourceByID(context.Background(), source.UID)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

	require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

	newSource, err := sourceRepo.FindSourceByID(context.Background(), source.UID)
	require.NoError(t, err)

	newSource.CreatedAt = time.Time{}
	newSource.UpdatedAt = time.Time{}

	require.Equal(t, source, newSource)
}

func Test_FindSourceByName(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	sourceRepo := NewSourceRepo(db, nil)
	source := generateSource(t, db)

	_, err := sourceRepo.FindSourceByName(context.Background(), source.ProjectID, source.Name)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

	require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

	newSource, err := sourceRepo.FindSourceByName(context.Background(), source.ProjectID, source.Name)
	require.NoError(t, err)

	newSource.CreatedAt = time.Time{}
	newSource.UpdatedAt = time.Time{}

	require.Equal(t, source, newSource)
}

func Test_FindSourceByMaskID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	sourceRepo := NewSourceRepo(db, nil)
	source := generateSource(t, db)

	_, err := sourceRepo.FindSourceByMaskID(context.Background(), source.MaskID)

	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

	require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

	newSource, err := sourceRepo.FindSourceByMaskID(context.Background(), source.MaskID)
	require.NoError(t, err)

	newSource.CreatedAt = time.Time{}
	newSource.UpdatedAt = time.Time{}

	require.Equal(t, source, newSource)
}

func Test_UpdateSource(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	sourceRepo := NewSourceRepo(db, nil)
	source := generateSource(t, db)

	require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

	name := "Convoy-Dev"
	source.Name = name
	source.IsDisabled = true
	source.CustomResponse = datastore.CustomResponse{
		Body:        "/ref/",
		ContentType: "application/json",
	}
	require.NoError(t, sourceRepo.UpdateSource(context.Background(), source))

	newSource, err := sourceRepo.FindSourceByID(context.Background(), source.UID)
	require.NoError(t, err)

	newSource.CreatedAt = time.Time{}
	newSource.UpdatedAt = time.Time{}

	require.Equal(t, source, newSource)
}

func Test_DeleteSource(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	sourceRepo := NewSourceRepo(db, nil)
	subRepo := NewSubscriptionRepo(db, nil)
	source := generateSource(t, db)

	require.NoError(t, sourceRepo.CreateSource(context.Background(), source))

	sub := &datastore.Subscription{
		Name:        "test_sub",
		Type:        datastore.SubscriptionTypeAPI,
		ProjectID:   source.ProjectID,
		SourceID:    source.UID,
		AlertConfig: &datastore.DefaultAlertConfig,
		RetryConfig: &datastore.DefaultRetryConfig,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
			Filter: datastore.FilterSchema{
				Headers: datastore.M{},
				Body:    datastore.M{},
			},
		},
		RateLimitConfig: &datastore.DefaultRateLimitConfig,
	}

	err := subRepo.CreateSubscription(context.Background(), source.ProjectID, sub)
	require.NoError(t, err)

	_, err = sourceRepo.FindSourceByID(context.Background(), source.UID)
	require.NoError(t, err)

	require.NoError(t, sourceRepo.DeleteSourceByID(context.Background(), source.UID, source.VerifierID))

	_, err = sourceRepo.FindSourceByID(context.Background(), source.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrSourceNotFound))

	_, err = subRepo.FindSubscriptionByID(context.Background(), source.ProjectID, sub.UID)
	require.Error(t, err)
	require.True(t, errors.Is(err, datastore.ErrSubscriptionNotFound))
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
			pageData: datastore.Pageable{PerPage: 3},
			count:    10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},

		{
			name:     "Load Sources Paged - 12 records",
			pageData: datastore.Pageable{PerPage: 4},
			count:    12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 4,
				},
			},
		},

		{
			name:     "Load Sources Paged - 5 records",
			pageData: datastore.Pageable{PerPage: 3},
			count:    5,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage: 3,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			sourceRepo := NewSourceRepo(db, nil)
			project := seedProject(t, db)

			for i := 0; i < tc.count; i++ {
				source := &datastore.Source{
					UID:       ulid.Make().String(),
					ProjectID: project.UID,
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
				require.NoError(t, sourceRepo.CreateSource(context.Background(), source))
			}

			_, pageable, err := sourceRepo.LoadSourcesPaged(context.Background(), project.UID, &datastore.SourceFilter{}, tc.pageData)

			require.NoError(t, err)
			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
		})
	}
}

func Test_LoadPubSubSourcesByProjectIDs(t *testing.T) {
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
			pageData: datastore.Pageable{PerPage: 3},
			count:    10,
			expected: Expected{
				paginationData: datastore.PaginationData{
					HasNextPage: true,
					PerPage:     3,
				},
			},
		},

		{
			name:     "Load Sources Paged - 12 records",
			pageData: datastore.Pageable{PerPage: 4},
			count:    12,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage:     4,
					HasNextPage: true,
				},
			},
		},

		{
			name:     "Load Sources Paged - 5 records",
			pageData: datastore.Pageable{PerPage: 3},
			count:    5,
			expected: Expected{
				paginationData: datastore.PaginationData{
					PerPage:     3,
					HasNextPage: true,
				},
			},
		},
	}

	pb := &datastore.PubSubConfig{
		Type:    datastore.GooglePubSub,
		Workers: 2,
		Sqs: &datastore.SQSPubSubConfig{
			AccessKeyID:   "3",
			SecretKey:     "***",
			DefaultRegion: "region",
			QueueName:     "q",
		},
		Google: &datastore.GooglePubSubConfig{
			SubscriptionID: "3333",
			ServiceAccount: []byte("service"),
			ProjectID:      "1234",
		},
		Kafka: &datastore.KafkaPubSubConfig{
			Brokers:         []string{"12345"},
			ConsumerGroupID: "3333",
			TopicName:       "topic",
			Auth: &datastore.KafkaAuth{
				Type:     "33",
				Hash:     "33",
				TLS:      true,
				Username: "1234",
				Password: "1233",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			var projectIDs []string
			sourceRepo := NewSourceRepo(db, nil)

			for i := 0; i < tc.count; i++ {
				project := seedProject(t, db)
				projectIDs = append(projectIDs, project.UID)

				source := &datastore.Source{
					UID:       ulid.Make().String(),
					ProjectID: project.UID,
					Name:      "Convoy-Prod",
					MaskID:    uniuri.NewLen(16),
					Type:      datastore.PubSubSource,
					PubSub:    pb,
					Verifier: &datastore.VerifierConfig{
						Type: datastore.HMacVerifier,
						HMac: &datastore.HMac{
							Header: "X-Paystack-Signature",
							Hash:   "SHA512",
							Secret: "Paystack Secret",
						},
					},
				}
				require.NoError(t, sourceRepo.CreateSource(context.Background(), source))
			}

			sources, pageable, err := sourceRepo.LoadPubSubSourcesByProjectIDs(context.Background(), projectIDs, tc.pageData)

			require.NoError(t, err)

			for _, s := range sources {
				require.Equal(t, *pb, *s.PubSub)
			}

			require.Equal(t, tc.expected.paginationData.PerPage, int64(len(sources)))
			require.Equal(t, tc.expected.paginationData.PerPage, pageable.PerPage)
			require.Equal(t, tc.expected.paginationData.HasNextPage, pageable.HasNextPage)
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

	require.NoError(t, NewSourceRepo(db, nil).CreateSource(context.Background(), source))
	return source
}
