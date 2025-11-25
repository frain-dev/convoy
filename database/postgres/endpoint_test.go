//go:build integration
// +build integration

package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jaswdr/faker"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/constants"
	"github.com/frain-dev/convoy/pkg/log"
)

func Test_UpdateEndpoint(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runUpdateEndpointTest(t, db)
}

func runUpdateEndpointTest(t *testing.T, db database.Database) {

	endpointRepo := NewEndpointRepo(db)

	project := seedProject(t, db)
	endpoint := generateEndpoint(project)

	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	updatedEndpoint := &datastore.Endpoint{
		UID:                endpoint.UID,
		ProjectID:          endpoint.ProjectID,
		OwnerID:            "4304jj39h43h",
		Url:                "https//uere.ccm",
		Name:               "testing_endpoint_repo",
		Secrets:            endpoint.Secrets,
		AdvancedSignatures: true,
		AppID:              endpoint.AppID,
		Description:        "9897fdkhkhd",
		SlackWebhookURL:    "https:/899gfnnn",
		SupportEmail:       "ex@convoybbb.com",
		HttpTimeout:        88,
		RateLimit:          8898,
		Status:             datastore.ActiveEndpointStatus,
		RateLimitDuration:  10,
		ContentType:        constants.ContentTypeJSON, // Set expected content type
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

func Test_UpdateEndpoint_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	runUpdateEndpointTest(t, db)

	assertAndInitEncryption(t, db)

	runUpdateEndpointTest(t, db)
}

func assertAndInitEncryption(t *testing.T, db database.Database) {
	isEncrypted, err := checkEncryptionStatus(db)
	require.NoError(t, err)
	require.False(t, isEncrypted)

	km, err := keys.Get()
	require.NoError(t, err)
	err = keys.InitEncryption(log.FromContext(context.Background()), db, km, "test-key", 120)
	require.NoError(t, err)

	isEncrypted, err = checkEncryptionStatus(db)
	require.NoError(t, err)
	require.True(t, isEncrypted)
}

func Test_UpdateEndpointStatus(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runUpdateEndpointStatusTest(t, db)
}

func Test_UpdateEndpointStatus_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runUpdateEndpointStatusTest(t, db)

	assertAndInitEncryption(t, db)

	runUpdateEndpointStatusTest(t, db)
}

func runUpdateEndpointStatusTest(t *testing.T, db database.Database) {
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

	runDeleteEndpointTest(t, db)
}

func Test_DeleteEndpoint_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	runDeleteEndpointTest(t, db)
	assertAndInitEncryption(t, db)
	runDeleteEndpointTest(t, db)
}

func runDeleteEndpointTest(t *testing.T, db database.Database) {
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
	runCreateEndpointTest(t, db)
}

func runCreateEndpointTest(t *testing.T, db database.Database) {

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

func Test_CreateEndpoint_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	runCreateEndpointTest(t, db)

	assertAndInitEncryption(t, db)

	runCreateEndpointTest(t, db)
}

func Test_LoadEndpointsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runLoadEndpointsPagedTest(t, db)
}

func Test_LoadEndpointsPaged_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runLoadEndpointsPagedTest(t, db)
	assertAndInitEncryption(t, db)
	runLoadEndpointsPagedTest(t, db)
}
func runLoadEndpointsPagedTest(t *testing.T, db database.Database) {

	endpointRepo := NewEndpointRepo(db)
	eventRepo := NewEventRepo(db)

	project := seedProject(t, db)

	for i := 0; i < 7; i++ {
		endpoint := generateEndpoint(project)
		if i == 1 || i == 2 || i == 4 {
			endpoint.Name += " daniel"
		}

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
	runFindEndpointsByIDTest(t, db)
}
func Test_FindEndpointsByID_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runFindEndpointsByIDTest(t, db)
	assertAndInitEncryption(t, db)
	runFindEndpointsByIDTest(t, db)
}
func runFindEndpointsByIDTest(t *testing.T, db database.Database) {

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
	runFindEndpointsByAppIDTest(t, db)
}

func Test_FindEndpointsByAppID_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runFindEndpointsByAppIDTest(t, db)
	assertAndInitEncryption(t, db)
	runFindEndpointsByAppIDTest(t, db)
}

func runFindEndpointsByAppIDTest(t *testing.T, db database.Database) {
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
	runFindEndpointsByOwnerIDTest(t, db)
}

func Test_FindEndpointsByOwnerID_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runFindEndpointsByOwnerIDTest(t, db)
	assertAndInitEncryption(t, db)
	runFindEndpointsByOwnerIDTest(t, db)
}

func runFindEndpointsByOwnerIDTest(t *testing.T, db database.Database) {

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
	runFindEndpointByIDTest(t, db)
}

func Test_FindEndpointByID_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runFindEndpointByIDTest(t, db)
	assertAndInitEncryption(t, db)
	runFindEndpointByIDTest(t, db)
}

func runFindEndpointByIDTest(t *testing.T, db database.Database) {
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
	runUpdateSecretsTest(t, db)
}

func Test_UpdateSecrets_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runUpdateSecretsTest(t, db)
	assertAndInitEncryption(t, db)
	runUpdateSecretsTest(t, db)
}

func runUpdateSecretsTest(t *testing.T, db database.Database) {
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
	runDeleteSecretTest(t, db)
}

func Test_DeleteSecret_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	runDeleteSecretTest(t, db)
	assertAndInitEncryption(t, db)
	runDeleteSecretTest(t, db)
}

func runDeleteSecretTest(t *testing.T, db database.Database) {
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
		Url:                faker.New().Address().StreetAddress(),
		Name:               fmt.Sprintf("%s-%s", faker.New().Company().Name(), ulid.Make().String()),
		AdvancedSignatures: true,
		Description:        "testing",
		SlackWebhookURL:    "https:/gggggg",
		SupportEmail:       "ex@convoy.com",
		AppID:              "app1",
		HttpTimeout:        30,
		RateLimit:          300,
		Status:             datastore.ActiveEndpointStatus,
		RateLimitDuration:  10,
		ContentType:        constants.ContentTypeJSON, // Set default content type for tests
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

func Test_CreateEndpoint_WithMtls(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	runCreateEndpointWithMtlsTest(t, db)
}

func Test_CreateEndpoint_WithMtls_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	runCreateEndpointWithMtlsTest(t, db)

	assertAndInitEncryption(t, db)

	runCreateEndpointWithMtlsTest(t, db)
}

func runCreateEndpointWithMtlsTest(t *testing.T, db database.Database) {
	projectRepo := NewProjectRepo(db)
	endpointRepo := NewEndpointRepo(db)

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Project with mTLS",
		LogoURL:        "s3.com/logo",
		OrganisationID: seedOrg(t, db).UID,
		Type:           datastore.OutgoingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), project))

	// Create endpoint with mTLS certificate
	endpoint := &datastore.Endpoint{
		UID:                ulid.Make().String(),
		ProjectID:          project.UID,
		OwnerID:            ulid.Make().String(),
		Url:                "https://secure.example.com",
		Name:               "mTLS Test Endpoint",
		AdvancedSignatures: true,
		Description:        "endpoint with mTLS",
		HttpTimeout:        30,
		RateLimit:          300,
		Status:             datastore.ActiveEndpointStatus,
		RateLimitDuration:  10,
		Secrets: []datastore.Secret{
			{
				UID:       ulid.Make().String(),
				Value:     "secret-value",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		MtlsClientCert: &datastore.MtlsClientCert{
			ClientCert: "-----BEGIN CERTIFICATE-----\ntest-cert-data\n-----END CERTIFICATE-----",
			ClientKey:  "-----BEGIN PRIVATE KEY-----\ntest-key-data\n-----END PRIVATE KEY-----",
		},
	}

	// Create the endpoint
	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	// Fetch it back and verify mTLS configuration
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	// Verify mTLS certificate was stored and retrieved correctly
	require.NotNil(t, dbEndpoint.MtlsClientCert)
	require.Equal(t, "-----BEGIN CERTIFICATE-----\ntest-cert-data\n-----END CERTIFICATE-----", dbEndpoint.MtlsClientCert.ClientCert)
	require.Equal(t, "-----BEGIN PRIVATE KEY-----\ntest-key-data\n-----END PRIVATE KEY-----", dbEndpoint.MtlsClientCert.ClientKey)

	// Update the endpoint with different mTLS config
	endpoint.MtlsClientCert = &datastore.MtlsClientCert{
		ClientCert: "-----BEGIN CERTIFICATE-----\nupdated-cert\n-----END CERTIFICATE-----",
		ClientKey:  "-----BEGIN PRIVATE KEY-----\nupdated-key\n-----END PRIVATE KEY-----",
	}

	err = endpointRepo.UpdateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	// Verify update worked
	dbEndpoint, err = endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)
	require.NotNil(t, dbEndpoint.MtlsClientCert)
	require.Equal(t, "-----BEGIN CERTIFICATE-----\nupdated-cert\n-----END CERTIFICATE-----", dbEndpoint.MtlsClientCert.ClientCert)
	require.Equal(t, "-----BEGIN PRIVATE KEY-----\nupdated-key\n-----END PRIVATE KEY-----", dbEndpoint.MtlsClientCert.ClientKey)
}

func Test_CreateEndpoint_WithOAuth2_Encrypted(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	runCreateEndpointWithOAuth2SharedSecretTest(t, db)
	runCreateEndpointWithOAuth2ClientAssertionECTest(t, db)
	runCreateEndpointWithOAuth2ClientAssertionRSATest(t, db)

	assertAndInitEncryption(t, db)

	runCreateEndpointWithOAuth2SharedSecretTest(t, db)
	runCreateEndpointWithOAuth2ClientAssertionECTest(t, db)
	runCreateEndpointWithOAuth2ClientAssertionRSATest(t, db)
}

func runCreateEndpointWithOAuth2SharedSecretTest(t *testing.T, db database.Database) {
	projectRepo := NewProjectRepo(db)
	endpointRepo := NewEndpointRepo(db)

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Project with OAuth2 Shared Secret",
		LogoURL:        "s3.com/logo",
		OrganisationID: seedOrg(t, db).UID,
		Type:           datastore.OutgoingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), project))

	// Create endpoint with OAuth2 shared secret
	endpoint := &datastore.Endpoint{
		UID:                ulid.Make().String(),
		ProjectID:          project.UID,
		OwnerID:            ulid.Make().String(),
		Url:                "https://oauth.example.com",
		Name:               "OAuth2 Shared Secret Test Endpoint",
		AdvancedSignatures: true,
		Description:        "endpoint with OAuth2 shared secret",
		HttpTimeout:        30,
		RateLimit:          300,
		Status:             datastore.ActiveEndpointStatus,
		RateLimitDuration:  10,
		Secrets: []datastore.Secret{
			{
				UID:       ulid.Make().String(),
				Value:     "secret-value",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                "https://oauth.example.com/token",
				ClientID:           "test-client-id",
				ClientSecret:       "test-client-secret-12345",
				GrantType:          "client_credentials",
				Scope:              "read write",
				AuthenticationType: datastore.SharedSecretAuth,
			},
		},
	}

	// Create the endpoint
	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	// Fetch it back and verify OAuth2 configuration
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	// Verify OAuth2 config was stored and retrieved correctly
	require.NotNil(t, dbEndpoint.Authentication)
	require.Equal(t, datastore.OAuth2Authentication, dbEndpoint.Authentication.Type)
	require.NotNil(t, dbEndpoint.Authentication.OAuth2)
	require.Equal(t, "https://oauth.example.com/token", dbEndpoint.Authentication.OAuth2.URL)
	require.Equal(t, "test-client-id", dbEndpoint.Authentication.OAuth2.ClientID)
	require.Equal(t, "test-client-secret-12345", dbEndpoint.Authentication.OAuth2.ClientSecret)
	require.Equal(t, "client_credentials", dbEndpoint.Authentication.OAuth2.GrantType)
	require.Equal(t, "read write", dbEndpoint.Authentication.OAuth2.Scope)
	require.Equal(t, datastore.SharedSecretAuth, dbEndpoint.Authentication.OAuth2.AuthenticationType)

	// Update the endpoint with different OAuth2 config
	endpoint.Authentication.OAuth2.ClientSecret = "updated-client-secret-67890"
	endpoint.Authentication.OAuth2.Scope = "read write admin"

	err = endpointRepo.UpdateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	// Verify update worked
	dbEndpoint, err = endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)
	require.NotNil(t, dbEndpoint.Authentication.OAuth2)
	require.Equal(t, "updated-client-secret-67890", dbEndpoint.Authentication.OAuth2.ClientSecret)
	require.Equal(t, "read write admin", dbEndpoint.Authentication.OAuth2.Scope)
}

func runCreateEndpointWithOAuth2ClientAssertionECTest(t *testing.T, db database.Database) {
	projectRepo := NewProjectRepo(db)
	endpointRepo := NewEndpointRepo(db)

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Project with OAuth2 Client Assertion EC",
		LogoURL:        "s3.com/logo",
		OrganisationID: seedOrg(t, db).UID,
		Type:           datastore.OutgoingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), project))

	// Create endpoint with OAuth2 client assertion (EC key)
	endpoint := &datastore.Endpoint{
		UID:                ulid.Make().String(),
		ProjectID:          project.UID,
		OwnerID:            ulid.Make().String(),
		Url:                "https://oauth.example.com",
		Name:               "OAuth2 Client Assertion EC Test Endpoint",
		AdvancedSignatures: true,
		Description:        "endpoint with OAuth2 client assertion EC",
		HttpTimeout:        30,
		RateLimit:          300,
		Status:             datastore.ActiveEndpointStatus,
		RateLimitDuration:  10,
		Secrets: []datastore.Secret{
			{
				UID:       ulid.Make().String(),
				Value:     "secret-value",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                "https://oauth.example.com/token",
				ClientID:           "test-client-id",
				GrantType:          "client_credentials",
				Scope:              "read write",
				AuthenticationType: datastore.ClientAssertionAuth,
				SigningAlgorithm:   "ES256",
				Issuer:             "test-client-id",
				Subject:            "test-client-id",
				SigningKey: &datastore.OAuth2SigningKey{
					Kty: "EC",
					Crv: "P-256",
					X:   "test-x-coordinate-base64url",
					Y:   "test-y-coordinate-base64url",
					D:   "test-private-key-base64url",
					Kid: "test-ec-key-id",
				},
			},
		},
	}

	// Create the endpoint
	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	// Fetch it back and verify OAuth2 configuration
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	// Verify OAuth2 config was stored and retrieved correctly
	require.NotNil(t, dbEndpoint.Authentication)
	require.Equal(t, datastore.OAuth2Authentication, dbEndpoint.Authentication.Type)
	require.NotNil(t, dbEndpoint.Authentication.OAuth2)
	require.Equal(t, "https://oauth.example.com/token", dbEndpoint.Authentication.OAuth2.URL)
	require.Equal(t, "test-client-id", dbEndpoint.Authentication.OAuth2.ClientID)
	require.Equal(t, datastore.ClientAssertionAuth, dbEndpoint.Authentication.OAuth2.AuthenticationType)
	require.Equal(t, "ES256", dbEndpoint.Authentication.OAuth2.SigningAlgorithm)
	require.Equal(t, "test-client-id", dbEndpoint.Authentication.OAuth2.Issuer)
	require.Equal(t, "test-client-id", dbEndpoint.Authentication.OAuth2.Subject)

	require.NotNil(t, dbEndpoint.Authentication.OAuth2.SigningKey)
	require.Equal(t, "EC", dbEndpoint.Authentication.OAuth2.SigningKey.Kty)
	require.Equal(t, "P-256", dbEndpoint.Authentication.OAuth2.SigningKey.Crv)
	require.Equal(t, "test-x-coordinate-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.X)
	require.Equal(t, "test-y-coordinate-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.Y)
	require.Equal(t, "test-private-key-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.D)
	require.Equal(t, "test-ec-key-id", dbEndpoint.Authentication.OAuth2.SigningKey.Kid)

	// Verify RSA fields are empty for EC keys
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.N)
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.E)
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.P)
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.Q)
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.Dp)
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.Dq)
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.Qi)
}

func runCreateEndpointWithOAuth2ClientAssertionRSATest(t *testing.T, db database.Database) {
	projectRepo := NewProjectRepo(db)
	endpointRepo := NewEndpointRepo(db)

	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Project with OAuth2 Client Assertion RSA",
		LogoURL:        "s3.com/logo",
		OrganisationID: seedOrg(t, db).UID,
		Type:           datastore.OutgoingProject,
		Config:         &datastore.DefaultProjectConfig,
	}

	require.NoError(t, projectRepo.CreateProject(context.Background(), project))

	// Create endpoint with OAuth2 client assertion (RSA key)
	endpoint := &datastore.Endpoint{
		UID:                ulid.Make().String(),
		ProjectID:          project.UID,
		OwnerID:            ulid.Make().String(),
		Url:                "https://oauth.example.com",
		Name:               "OAuth2 Client Assertion RSA Test Endpoint",
		AdvancedSignatures: true,
		Description:        "endpoint with OAuth2 client assertion RSA",
		HttpTimeout:        30,
		RateLimit:          300,
		Status:             datastore.ActiveEndpointStatus,
		RateLimitDuration:  10,
		Secrets: []datastore.Secret{
			{
				UID:       ulid.Make().String(),
				Value:     "secret-value",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		Authentication: &datastore.EndpointAuthentication{
			Type: datastore.OAuth2Authentication,
			OAuth2: &datastore.OAuth2{
				URL:                "https://oauth.example.com/token",
				ClientID:           "test-client-id",
				GrantType:          "client_credentials",
				Scope:              "read write",
				AuthenticationType: datastore.ClientAssertionAuth,
				SigningAlgorithm:   "RS256",
				Issuer:             "test-client-id",
				Subject:            "test-client-id",
				SigningKey: &datastore.OAuth2SigningKey{
					Kty: "RSA",
					N:   "test-rsa-n-base64url",
					E:   "test-rsa-e-base64url",
					D:   "test-rsa-d-base64url",
					P:   "test-rsa-p-base64url",
					Q:   "test-rsa-q-base64url",
					Dp:  "test-rsa-dp-base64url",
					Dq:  "test-rsa-dq-base64url",
					Qi:  "test-rsa-qi-base64url",
					Kid: "test-rsa-key-id",
				},
			},
		},
	}

	// Create the endpoint
	err := endpointRepo.CreateEndpoint(context.Background(), endpoint, project.UID)
	require.NoError(t, err)

	// Fetch it back and verify OAuth2 configuration
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, project.UID)
	require.NoError(t, err)

	// Verify OAuth2 config was stored and retrieved correctly
	require.NotNil(t, dbEndpoint.Authentication)
	require.Equal(t, datastore.OAuth2Authentication, dbEndpoint.Authentication.Type)
	require.NotNil(t, dbEndpoint.Authentication.OAuth2)
	require.Equal(t, "https://oauth.example.com/token", dbEndpoint.Authentication.OAuth2.URL)
	require.Equal(t, "test-client-id", dbEndpoint.Authentication.OAuth2.ClientID)
	require.Equal(t, datastore.ClientAssertionAuth, dbEndpoint.Authentication.OAuth2.AuthenticationType)
	require.Equal(t, "RS256", dbEndpoint.Authentication.OAuth2.SigningAlgorithm)
	require.Equal(t, "test-client-id", dbEndpoint.Authentication.OAuth2.Issuer)
	require.Equal(t, "test-client-id", dbEndpoint.Authentication.OAuth2.Subject)

	require.NotNil(t, dbEndpoint.Authentication.OAuth2.SigningKey)
	require.Equal(t, "RSA", dbEndpoint.Authentication.OAuth2.SigningKey.Kty)
	require.Equal(t, "test-rsa-n-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.N)
	require.Equal(t, "test-rsa-e-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.E)
	require.Equal(t, "test-rsa-d-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.D)
	require.Equal(t, "test-rsa-p-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.P)
	require.Equal(t, "test-rsa-q-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.Q)
	require.Equal(t, "test-rsa-dp-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.Dp)
	require.Equal(t, "test-rsa-dq-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.Dq)
	require.Equal(t, "test-rsa-qi-base64url", dbEndpoint.Authentication.OAuth2.SigningKey.Qi)
	require.Equal(t, "test-rsa-key-id", dbEndpoint.Authentication.OAuth2.SigningKey.Kid)

	// Verify EC fields are empty for RSA keys
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.Crv)
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.X)
	require.Empty(t, dbEndpoint.Authentication.OAuth2.SigningKey.Y)
}
