package sources

import (
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestLoadPubSubSourcesByProjectIDs_SingleProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create PubSub sources
	for i := 0; i < 3; i++ {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      fmt.Sprintf("PubSubSource-%d", i),
			Type:      datastore.PubSubSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.GithubSourceProvider,
			ProjectID: project.UID,
			PubSub: &datastore.PubSubConfig{
				Type:    datastore.GooglePubSub,
				Workers: 5,
				Google: &datastore.GooglePubSubConfig{
					ProjectID:      fmt.Sprintf("google-project-%d", i),
					SubscriptionID: fmt.Sprintf("subscription-%d", i),
				},
			},
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// Create HTTP sources (should be excluded)
	SeedSource(t, db, project, datastore.NoopVerifier)
	SeedSource(t, db, project, datastore.APIKeyVerifier)

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{project.UID}, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 3)

	// Verify all are PubSub sources
	for _, s := range sources {
		require.Equal(t, datastore.PubSubSource, s.Type)
		require.NotNil(t, s.PubSub)
	}
}

func TestLoadPubSubSourcesByProjectIDs_MultipleProjects(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project1 := seedTestData(t, db)
	project2 := seedTestData(t, db)
	project3 := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create PubSub sources in project1
	for i := 0; i < 2; i++ {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      fmt.Sprintf("Project1-PubSub-%d", i),
			Type:      datastore.PubSubSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.GithubSourceProvider,
			ProjectID: project1.UID,
			PubSub: &datastore.PubSubConfig{
				Type:    datastore.SqsPubSub,
				Workers: 3,
				Sqs: &datastore.SQSPubSubConfig{
					QueueName: fmt.Sprintf("queue-%d", i),
				},
			},
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// Create PubSub sources in project2
	for i := 0; i < 3; i++ {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      fmt.Sprintf("Project2-PubSub-%d", i),
			Type:      datastore.PubSubSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.GithubSourceProvider,
			ProjectID: project2.UID,
			PubSub: &datastore.PubSubConfig{
				Type:    datastore.KafkaPubSub,
				Workers: 5,
				Kafka: &datastore.KafkaPubSubConfig{
					Brokers:   []string{"localhost:9092"},
					TopicName: "topic1",
				},
			},
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// Create PubSub sources in project3 (should be excluded)
	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "Project3-PubSub",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project3.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.GooglePubSub,
			Workers: 5,
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}
	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Query for project1 and project2 only
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadPubSubSourcesByProjectIDs(
		ctx,
		[]string{project1.UID, project2.UID},
		pageable,
	)
	require.NoError(t, err)
	require.Len(t, sources, 5)

	// Verify project distribution
	project1Count := 0
	project2Count := 0
	for _, s := range sources {
		switch s.ProjectID {
		case project1.UID:
			project1Count++
		case project2.UID:
			project2Count++
		}
	}
	require.Equal(t, 2, project1Count)
	require.Equal(t, 3, project2Count)
}

func TestLoadPubSubSourcesByProjectIDs_Pagination(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create 10 PubSub sources
	for i := 0; i < 10; i++ {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      fmt.Sprintf("PubSubSource-%d", i),
			Type:      datastore.PubSubSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.GithubSourceProvider,
			ProjectID: project.UID,
			PubSub: &datastore.PubSubConfig{
				Type:    datastore.GooglePubSub,
				Workers: 5,
			},
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	// First page
	pageable := datastore.Pageable{
		PerPage:    3,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	page1, pagination1, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{project.UID}, pageable)
	require.NoError(t, err)
	require.Len(t, page1, 3)
	require.True(t, pagination1.HasNextPage)
	require.NotEmpty(t, pagination1.NextPageCursor)

	// Second page
	pageable.NextCursor = pagination1.NextPageCursor
	page2, pagination2, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{project.UID}, pageable)
	require.NoError(t, err)
	require.Len(t, page2, 3)
	require.True(t, pagination2.HasNextPage)

	// Verify pages are different
	require.NotEqual(t, page1[0].UID, page2[0].UID)
}

func TestLoadPubSubSourcesByProjectIDs_EmptyProjectList(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := createSourceService(t, db)

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{}, pageable)
	require.NoError(t, err)
	require.Empty(t, sources)
}

func TestLoadPubSubSourcesByProjectIDs_NoSources(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create only HTTP sources
	SeedSource(t, db, project, datastore.NoopVerifier)
	SeedSource(t, db, project, datastore.APIKeyVerifier)

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, pagination, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{project.UID}, pageable)
	require.NoError(t, err)
	require.Empty(t, sources)
	require.False(t, pagination.HasNextPage)
}

func TestLoadPubSubSourcesByProjectIDs_DifferentPubSubTypes(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create Google PubSub source
	googleSource := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "GooglePubSub",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.GooglePubSub,
			Workers: 5,
			Google: &datastore.GooglePubSubConfig{
				ProjectID:      "google-project",
				SubscriptionID: "subscription",
			},
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}
	err := service.CreateSource(ctx, googleSource)
	require.NoError(t, err)

	// Create SQS source
	sqsSource := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "SQSPubSub",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.SqsPubSub,
			Workers: 3,
			Sqs: &datastore.SQSPubSubConfig{
				QueueName: "test-queue",
			},
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}
	err = service.CreateSource(ctx, sqsSource)
	require.NoError(t, err)

	// Create Kafka source
	kafkaSource := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "KafkaPubSub",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.KafkaPubSub,
			Workers: 7,
			Kafka: &datastore.KafkaPubSubConfig{
				Brokers:   []string{"localhost:9092"},
				TopicName: "events",
			},
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}
	err = service.CreateSource(ctx, kafkaSource)
	require.NoError(t, err)

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{project.UID}, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 3)

	// Verify all types are present
	typeMap := make(map[datastore.PubSubType]bool)
	for _, s := range sources {
		typeMap[s.PubSub.Type] = true
	}
	require.True(t, typeMap[datastore.GooglePubSub])
	require.True(t, typeMap[datastore.SqsPubSub])
	require.True(t, typeMap[datastore.KafkaPubSub])
}

func TestLoadPubSubSourcesByProjectIDs_ExcludesDeleted(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create PubSub sources
	source1 := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "PubSub1",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.GooglePubSub,
			Workers: 5,
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}
	err := service.CreateSource(ctx, source1)
	require.NoError(t, err)

	source2 := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "PubSub2",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.SqsPubSub,
			Workers: 3,
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}
	err = service.CreateSource(ctx, source2)
	require.NoError(t, err)

	// Delete one source
	err = service.DeleteSourceByID(ctx, project.UID, source1.UID, source1.VerifierID)
	require.NoError(t, err)

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{project.UID}, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, source2.UID, sources[0].UID)
}

func TestLoadPubSubSourcesByProjectIDs_OrderByID(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create sources
	for i := 0; i < 5; i++ {
		source := &datastore.Source{
			UID:       ulid.Make().String(),
			Name:      fmt.Sprintf("PubSub-%d", i),
			Type:      datastore.PubSubSource,
			MaskID:    ulid.Make().String(),
			Provider:  datastore.GithubSourceProvider,
			ProjectID: project.UID,
			PubSub: &datastore.PubSubConfig{
				Type:    datastore.GooglePubSub,
				Workers: 5,
			},
			Verifier: &datastore.VerifierConfig{
				Type: datastore.NoopVerifier,
			},
		}
		err := service.CreateSource(ctx, source)
		require.NoError(t, err)
	}

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{project.UID}, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 5)

	// Verify descending order by ID
	for i := 0; i < len(sources)-1; i++ {
		require.True(t, sources[i].UID > sources[i+1].UID)
	}
}

func TestLoadPubSubSourcesByProjectIDs_IncludesPubSubConfig(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "DetailedPubSub",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.GooglePubSub,
			Workers: 10,
			Google: &datastore.GooglePubSubConfig{
				ProjectID:      "test-project-123",
				SubscriptionID: "test-subscription-456",
			},
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}
	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: datastore.DefaultCursor,
	}

	sources, _, err := service.LoadPubSubSourcesByProjectIDs(ctx, []string{project.UID}, pageable)
	require.NoError(t, err)
	require.Len(t, sources, 1)

	// Verify PubSub config is fully populated
	fetched := sources[0]
	require.NotNil(t, fetched.PubSub)
	require.Equal(t, datastore.GooglePubSub, fetched.PubSub.Type)
	require.Equal(t, 10, fetched.PubSub.Workers)
	require.NotNil(t, fetched.PubSub.Google)
	require.Equal(t, "test-project-123", fetched.PubSub.Google.ProjectID)
	require.Equal(t, "test-subscription-456", fetched.PubSub.Google.SubscriptionID)
}
