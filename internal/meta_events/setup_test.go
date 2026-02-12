package meta_events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var testEnv *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		panic(err)
	}
	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Printf("failed to cleanup: %v\n", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (database.Database, context.Context) {
	t.Helper()

	ctx := context.Background()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

	db := postgres.NewFromConnection(conn)

	// Load config again and ensure it's set properly
	err = config.LoadConfig("")
	require.NoError(t, err)

	_, err = config.Get()
	require.NoError(t, err)

	// Initialize KeyManager
	km, err := keys.NewLocalKeyManager("test")
	require.NoError(t, err)

	if km.IsSet() {
		_, err = km.GetCurrentKeyFromCache()
		require.NoError(t, err)
	}

	err = keys.Set(km)
	require.NoError(t, err)

	return db, ctx
}

func createMetaEventService(t *testing.T, db database.Database) *Service {
	t.Helper()
	return New(log.NewLogger(os.Stdout), db)
}

func seedTestData(t *testing.T, db database.Database) (*datastore.User, *datastore.Organisation, *datastore.Project) {
	ctx := context.Background()
	logger := log.NewLogger(os.Stdout)

	// Create user
	userRepo := users.New(logger, db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
	}
	err := userRepo.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create organisation
	orgRepo := organisations.New(logger, db)
	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		Name:    "Test Org",
		OwnerID: user.UID,
	}
	err = orgRepo.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Create project
	projectRepo := projects.New(logger, db)
	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
	}
	err = projectRepo.CreateProject(ctx, project)
	require.NoError(t, err)

	return user, org, project
}

func seedMetaEvent(t *testing.T, db database.Database, project *datastore.Project) *datastore.MetaEvent {
	ctx := context.Background()
	service := createMetaEventService(t, db)

	metaEvent := &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		EventType: string(datastore.EndpointCreated),
		Metadata: &datastore.Metadata{
			Data:            json.RawMessage(`{"test": "data"}`),
			Raw:             `{"test": "data"}`,
			Strategy:        datastore.ExponentialStrategyProvider,
			NextSendTime:    time.Now().Add(time.Hour),
			NumTrials:       0,
			IntervalSeconds: 60,
			RetryLimit:      3,
		},
		Status: datastore.ScheduledEventStatus,
	}

	err := service.CreateMetaEvent(ctx, metaEvent)
	require.NoError(t, err)

	return metaEvent
}

func seedMultipleMetaEvents(t *testing.T, db database.Database, project *datastore.Project, count int) []*datastore.MetaEvent {
	ctx := context.Background()
	service := createMetaEventService(t, db)

	metaEvents := make([]*datastore.MetaEvent, count)
	eventTypes := []datastore.HookEventType{
		datastore.EndpointCreated,
		datastore.EndpointUpdated,
		datastore.EndpointDeleted,
		datastore.EventDeliverySuccess,
		datastore.EventDeliveryFailed,
	}

	for i := 0; i < count; i++ {
		metaEvent := &datastore.MetaEvent{
			UID:       ulid.Make().String(),
			ProjectID: project.UID,
			EventType: string(eventTypes[i%len(eventTypes)]),
			Metadata: &datastore.Metadata{
				Data:            json.RawMessage(fmt.Sprintf(`{"index": %d}`, i)),
				Raw:             fmt.Sprintf(`{"index": %d}`, i),
				Strategy:        datastore.ExponentialStrategyProvider,
				NextSendTime:    time.Now().Add(time.Hour * time.Duration(i+1)),
				NumTrials:       uint64(i),
				IntervalSeconds: 60,
				RetryLimit:      3,
			},
			Status: datastore.ScheduledEventStatus,
		}

		err := service.CreateMetaEvent(ctx, metaEvent)
		require.NoError(t, err)

		metaEvents[i] = metaEvent
	}

	return metaEvents
}
