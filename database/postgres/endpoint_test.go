//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/jaswdr/faker"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func Test_UpdateEndpoint(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)

	project := seedProject(t, db)
	endpoint := generateEndpoint(project)

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	updatedEndpoint := &datastore.Endpoint{
		UID:                endpoint.UID,
		ProjectID:          endpoint.ProjectID,
		OwnerID:            "4304jj39h43h",
		TargetURL:          "https//uere.ccm",
		Title:              "testing_endpoint_repo",
		Secrets:            endpoint.Secrets,
		AdvancedSignatures: true,
		AppID:              endpoint.AppID,
		Description:        "9897fdkhkhd",
		SlackWebhookURL:    "https:/899gfnnn",
		SupportEmail:       "ex@convoybbb.com",
		HttpTimeout:        "88s",
		RateLimit:          8898,
		Status:             datastore.ActiveEndpointStatus,
		RateLimitDuration:  "48s",
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.APIKeyAuthentication,
			ApiKey: &datastore.ApiKey{
				HeaderValue: "97if7dgfg",
				HeaderName:  "x-header-p",
			},
		},
	}

	require.NoError(t, endpointRepo.UpdateEndpoint(context.Background(), updatedEndpoint, updatedEndpoint.ProjectID))

	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbEndpoint.CreatedAt)
	require.NotEmpty(t, dbEndpoint.UpdatedAt)

	dbEndpoint.CreatedAt = time.Time{}
	dbEndpoint.UpdatedAt = time.Time{}

	for i := range dbEndpoint.Secrets {
		secret := &dbEndpoint.Secrets[i]

		require.Equal(t, updatedEndpoint.Secrets[i].Value, secret.Value)
		require.NotEmpty(t, secret.CreatedAt)
		require.NotEmpty(t, secret.UpdatedAt)

		secret.CreatedAt, secret.UpdatedAt = time.Time{}, time.Time{}
		updatedEndpoint.Secrets[i].CreatedAt, updatedEndpoint.Secrets[i].UpdatedAt = time.Time{}, time.Time{}
	}

	require.Equal(t, updatedEndpoint, dbEndpoint)
}

func Test_UpdateEndpointStatus(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)

	project := seedProject(t, db)

	endpoint := generateEndpoint(project)

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	status := datastore.InactiveEndpointStatus

	endpoint.Status = status

	require.NoError(t, endpointRepo.UpdateEndpointStatus(context.Background(), project.UID, endpoint.UID, status))

	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	require.Equal(t, status, dbEndpoint.Status)
}

func Test_DeleteEndpoint(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)

	project := seedProject(t, db)

	endpoint := generateEndpoint(project)

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	sub := &datastore.Subscription{
		UID:         ulid.Make().String(),
		Name:        "test_sub",
		Type:        datastore.SubscriptionTypeAPI,
		ProjectID:   project.UID,
		EndpointID:  endpoint.UID,
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

	subRepo := NewSubscriptionRepo(db)
	err = subRepo.CreateSubscription(context.Background(), project.UID, sub)
	require.NoError(t, err)

	err = endpointRepo.DeleteEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	_, err = endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.Equal(t, datastore.ErrEndpointNotFound, err)

	_, err = subRepo.FindSubscriptionByID(context.Background(), project.UID, sub.UID)
	require.Equal(t, datastore.ErrSubscriptionNotFound, err)
}

func Test_CreateEndpoint(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	projectRepo := NewProjectRepo(db)
	endpointRepo := NewEndpointRepo(db)

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Yet another project",
		LogoURL:        "s3.com/dsiuirueiy",
		OrganisationID: seedOrg(t, db).UID,
		Type:           datastore.IncomingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), project))
	endpoint := generateEndpoint(project)

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbEndpoint.CreatedAt)
	require.NotEmpty(t, dbEndpoint.UpdatedAt)

	dbEndpoint.CreatedAt = time.Time{}
	dbEndpoint.UpdatedAt = time.Time{}

	for i := range dbEndpoint.Secrets {
		secret := &dbEndpoint.Secrets[i]

		require.Equal(t, endpoint.Secrets[i].Value, secret.Value)
		require.NotEmpty(t, secret.CreatedAt)
		require.NotEmpty(t, secret.UpdatedAt)

		secret.CreatedAt, secret.UpdatedAt = time.Time{}, time.Time{}
		endpoint.Secrets[i].CreatedAt, endpoint.Secrets[i].UpdatedAt = time.Time{}, time.Time{}
	}

	require.Equal(t, endpoint, dbEndpoint)
}

func Test_LoadEndpointsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)
	eventRepo := NewEventRepo(db)

	project := seedProject(t, db)

	for i := 0; i < 7; i++ {
		endpoint := generateEndpoint(project)
		if i == 1 || i == 2 || i == 4 {
			endpoint.Title += fmt.Sprintf(" daniel-%+v", ulid.Make().String())
		}

		fmt.Printf("%+v\n", endpoint.Title)

		err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
		require.NoError(t, err)

		event := generateEvent(t, db)
		event.Endpoints = []string{endpoint.UID}
		require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
	}

	endpoints, _, err := endpointRepo.LoadEndpointsPaged(context.Background(), project.UID, &datastore.Filter{Query: "daniel"}, datastore.Pageable{
		PerPage: 10,
	})

	require.NoError(t, err)
	require.Equal(t, 3, len(endpoints))

	endpoints, _, err = endpointRepo.LoadEndpointsPaged(context.Background(), project.UID, &datastore.Filter{}, datastore.Pageable{
		PerPage: 10,
	})

	require.NoError(t, err)

	require.True(t, len(endpoints) == 7)
}

func Test_FindEndpointsByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)
	eventRepo := NewEventRepo(db)

	project := seedProject(t, db)
	ids := []string{}
	endpointMap := map[string]*datastore.Endpoint{}
	for i := 0; i < 7; i++ {
		endpoint := generateEndpoint(project)

		if i == 0 || i == 3 || i == 4 {
			endpoint.Secrets[0].Value += fmt.Sprintf("ddhdhhss-%d", i)
			endpointMap[endpoint.UID] = endpoint
			ids = append(ids, endpoint.UID)
		}

		err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
		require.NoError(t, err)

		event := generateEvent(t, db)
		event.Endpoints = []string{endpoint.UID}
		require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
	}

	emptyEndpoints, err := endpointRepo.FindEndpointsByID(context.Background(), ids, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(emptyEndpoints))

	dbEndpoints, err := endpointRepo.FindEndpointsByID(context.Background(), ids, project.UID)
	require.NoError(t, err)
	require.Equal(t, 3, len(dbEndpoints))

	for _, dbEndpoint := range dbEndpoints {
		endpoint, ok := endpointMap[dbEndpoint.UID]
		require.True(t, ok)

		require.NotEmpty(t, dbEndpoint.CreatedAt)
		require.NotEmpty(t, dbEndpoint.UpdatedAt)

		dbEndpoint.CreatedAt, dbEndpoint.UpdatedAt = time.Time{}, time.Time{}

		for i := range dbEndpoint.Secrets {
			s := &dbEndpoint.Secrets[i]
			require.NotEmpty(t, s.CreatedAt)
			require.NotEmpty(t, s.UpdatedAt)

			s.CreatedAt, s.UpdatedAt = time.Time{}, time.Time{}
			endpoint.Secrets[i].CreatedAt, endpoint.Secrets[i].UpdatedAt = time.Time{}, time.Time{}
		}

		dbEndpoint.Events = 0
		require.Equal(t, *endpoint, dbEndpoint)
	}
}

func Test_FindEndpointsByAppID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)
	eventRepo := NewEventRepo(db)

	project := seedProject(t, db)
	appID := "vvbbbb"
	endpointMap := map[string]*datastore.Endpoint{}
	for i := 0; i < 7; i++ {
		endpoint := generateEndpoint(project)

		if i < 4 {
			endpoint.AppID = appID
			endpointMap[endpoint.UID] = endpoint
		}

		err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
		require.NoError(t, err)

		event := generateEvent(t, db)
		event.Endpoints = []string{endpoint.UID}
		require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
	}

	emptyEndpoints, err := endpointRepo.FindEndpointsByAppID(context.Background(), appID, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(emptyEndpoints))

	dbEndpoints, err := endpointRepo.FindEndpointsByAppID(context.Background(), appID, project.UID)
	require.NoError(t, err)
	require.Equal(t, 4, len(dbEndpoints))

	for _, dbEndpoint := range dbEndpoints {
		endpoint, ok := endpointMap[dbEndpoint.UID]
		require.True(t, ok)

		require.NotEmpty(t, dbEndpoint.CreatedAt)
		require.NotEmpty(t, dbEndpoint.UpdatedAt)

		dbEndpoint.CreatedAt, dbEndpoint.UpdatedAt = time.Time{}, time.Time{}

		for i := range dbEndpoint.Secrets {
			s := &dbEndpoint.Secrets[i]
			require.NotEmpty(t, s.CreatedAt)
			require.NotEmpty(t, s.UpdatedAt)

			s.CreatedAt, s.UpdatedAt = time.Time{}, time.Time{}
			endpoint.Secrets[i].CreatedAt, endpoint.Secrets[i].UpdatedAt = time.Time{}, time.Time{}
		}

		require.Equal(t, *endpoint, dbEndpoint)
	}
}

func Test_FindEndpointsByOwnerID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)
	eventRepo := NewEventRepo(db)

	project := seedProject(t, db)
	ownerID := "owner-ffdjj"
	endpointMap := map[string]*datastore.Endpoint{}
	for i := 0; i < 7; i++ {
		endpoint := generateEndpoint(project)

		if i < 4 {
			endpoint.OwnerID = ownerID
			endpointMap[endpoint.UID] = endpoint
		}

		err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
		require.NoError(t, err)

		event := generateEvent(t, db)
		event.Endpoints = []string{endpoint.UID}
		require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
	}

	emptyEndpoints, err := endpointRepo.FindEndpointsByOwnerID(context.Background(), "", ownerID)
	require.NoError(t, err)
	require.Equal(t, 0, len(emptyEndpoints))

	dbEndpoints, err := endpointRepo.FindEndpointsByOwnerID(context.Background(), project.UID, ownerID)
	require.NoError(t, err)
	require.Equal(t, 4, len(dbEndpoints))

	for _, dbEndpoint := range dbEndpoints {
		endpoint, ok := endpointMap[dbEndpoint.UID]
		require.True(t, ok)

		require.NotEmpty(t, dbEndpoint.CreatedAt)
		require.NotEmpty(t, dbEndpoint.UpdatedAt)

		dbEndpoint.CreatedAt, dbEndpoint.UpdatedAt = time.Time{}, time.Time{}

		for i := range dbEndpoint.Secrets {
			s := &dbEndpoint.Secrets[i]
			require.NotEmpty(t, s.CreatedAt)
			require.NotEmpty(t, s.UpdatedAt)

			s.CreatedAt, s.UpdatedAt = time.Time{}, time.Time{}
			endpoint.Secrets[i].CreatedAt, endpoint.Secrets[i].UpdatedAt = time.Time{}, time.Time{}
		}

		require.Equal(t, *endpoint, dbEndpoint)
	}
}

func Test_CountProjectEndpoints(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)

	project := seedProject(t, db)
	for i := 0; i < 6; i++ {
		endpoint := generateEndpoint(project)
		err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
		require.NoError(t, err)
	}

	for i := 0; i < 3; i++ {
		endpoint := generateEndpoint(project)
		p := seedProject(t, db)
		endpoint.ProjectID = p.UID

		err := endpointRepo.CreateEndpoint(context.Background(), endpoint, p.UID)
		require.NoError(t, err)
	}

	c, err := endpointRepo.CountProjectEndpoints(context.Background(), project.UID)
	require.NoError(t, err)

	require.Equal(t, int64(6), c)
}

func Test_FindEndpointByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)
	eventRepo := NewEventRepo(db)

	_, err := endpointRepo.FindEndpointByID(context.Background(), ulid.Make().String(), "")
	require.Equal(t, datastore.ErrEndpointNotFound, err)

	project := seedProject(t, db)
	endpoint := generateEndpoint(project)

	err = endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	event := generateEvent(t, db)
	event.Endpoints = []string{endpoint.UID}
	require.NoError(t, eventRepo.CreateEvent(context.Background(), event))

	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	require.NotEmpty(t, dbEndpoint.CreatedAt)
	require.NotEmpty(t, dbEndpoint.UpdatedAt)

	dbEndpoint.CreatedAt = time.Time{}
	dbEndpoint.UpdatedAt = time.Time{}

	for i := range dbEndpoint.Secrets {
		secret := &dbEndpoint.Secrets[i]

		require.Equal(t, endpoint.Secrets[i].Value, secret.Value)
		require.NotEmpty(t, secret.CreatedAt)
		require.NotEmpty(t, secret.UpdatedAt)

		secret.CreatedAt, secret.UpdatedAt = time.Time{}, time.Time{}
		endpoint.Secrets[i].CreatedAt, endpoint.Secrets[i].UpdatedAt = time.Time{}, time.Time{}
	}

	require.Equal(t, endpoint, dbEndpoint)
}

func Test_UpdateSecrets(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)

	project := seedProject(t, db)
	endpoint := generateEndpoint(project)

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	newSecret := datastore.Secret{
		UID:       ulid.Make().String(),
		Value:     "new_secret",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	endpoint.Secrets[0].ExpiresAt = null.NewTime(time.Now(), true)
	endpoint.Secrets = append(endpoint.Secrets, newSecret)

	err = endpointRepo.UpdateSecrets(context.Background(), endpoint.UID, project.UID, endpoint.Secrets)
	require.NoError(t, err)

	newSecretEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	require.Equal(t, endpoint.Secrets[0].UID, newSecretEndpoint.Secrets[0].UID)
	require.Equal(t, endpoint.Secrets[0].Value, newSecretEndpoint.Secrets[0].Value)
	require.NotEmpty(t, newSecretEndpoint.Secrets[0].ExpiresAt)
	require.NotEmpty(t, newSecretEndpoint.Secrets[0].CreatedAt)
	require.NotEmpty(t, newSecretEndpoint.Secrets[0].UpdatedAt)

	require.Equal(t, newSecret.UID, newSecretEndpoint.Secrets[1].UID)
	require.Equal(t, newSecret.Value, newSecretEndpoint.Secrets[1].Value)
	require.Empty(t, newSecretEndpoint.Secrets[1].ExpiresAt)
	require.NotEmpty(t, newSecretEndpoint.Secrets[1].CreatedAt)
	require.NotEmpty(t, newSecretEndpoint.Secrets[1].UpdatedAt)
}

func Test_DeleteSecret(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	endpointRepo := NewEndpointRepo(db)

	project := seedProject(t, db)
	endpoint := generateEndpoint(project)

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	newSecret := datastore.Secret{
		UID:       ulid.Make().String(),
		Value:     "new_secret",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	endpoint.Secrets[0].ExpiresAt = null.NewTime(time.Now(), true)
	endpoint.Secrets = append(endpoint.Secrets, newSecret)

	err = endpointRepo.UpdateSecrets(context.Background(), endpoint.UID, project.UID, endpoint.Secrets)
	require.NoError(t, err)

	err = endpointRepo.DeleteSecret(context.Background(), endpoint, endpoint.Secrets[0].UID, project.UID)
	require.NoError(t, err)

	deletedSecretEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	for _, secret := range deletedSecretEndpoint.Secrets {
		require.NotEqual(t, secret.UID, endpoint.Secrets[0].UID) // the deleted secret should not appear in a fetch
	}

	require.Equal(t, newSecret.UID, deletedSecretEndpoint.Secrets[0].UID)
	require.Equal(t, newSecret.Value, deletedSecretEndpoint.Secrets[0].Value)
	require.Empty(t, deletedSecretEndpoint.Secrets[0].ExpiresAt)
	require.Empty(t, deletedSecretEndpoint.Secrets[0].DeletedAt)
	require.NotEmpty(t, deletedSecretEndpoint.Secrets[0].CreatedAt)
	require.NotEmpty(t, deletedSecretEndpoint.Secrets[0].UpdatedAt)
}

func generateEndpoint(project *datastore.Project) *datastore.Endpoint {
	return &datastore.Endpoint{
		UID:                ulid.Make().String(),
		ProjectID:          project.UID,
		OwnerID:            ulid.Make().String(),
		TargetURL:          faker.New().Address().StreetAddress(),
		Title:              "Next application name",
		AdvancedSignatures: true,
		Description:        "testing",
		SlackWebhookURL:    "https:/gggggg",
		SupportEmail:       "ex@convoy.com",
		AppID:              "app1",
		HttpTimeout:        "30s",
		RateLimit:          300,
		Status:             datastore.ActiveEndpointStatus,
		RateLimitDuration:  "4s",
		Secrets: []datastore.Secret{
			{
				UID:       ulid.Make().String(),
				Value:     "kirer",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.APIKeyAuthentication,
			ApiKey: &datastore.ApiKey{
				HeaderValue: "4387rjejhgjfyuyu34",
				HeaderName:  "x-header",
			},
		},
	}
}

func seedEndpoint(t *testing.T, db database.Database) *datastore.Endpoint {
	project := seedProject(t, db)
	endpoint := generateEndpoint(project)

	err := NewEndpointRepo(db).CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	return endpoint
}
