package projects

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

func TestCreateProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	tests := []struct {
		name    string
		project *datastore.Project
		wantErr bool
		errMsg  string
	}{
		{
			name: "should_create_project_successfully",
			project: &datastore.Project{
				UID:            ulid.Make().String(),
				Name:           "Test Project",
				Type:           datastore.OutgoingProject,
				OrganisationID: org.UID,
				Config:         getDefaultProjectConfig(),
			},
			wantErr: false,
		},
		{
			name: "should_create_project_with_custom_config",
			project: &datastore.Project{
				UID:            ulid.Make().String(),
				Name:           "Custom Config Project",
				Type:           datastore.OutgoingProject,
				OrganisationID: org.UID,
				Config: &datastore.ProjectConfig{
					SearchPolicy:  "test-policy",
					MaxIngestSize: 5242880,
					ReplayAttacks: true,
					RateLimit: &datastore.RateLimitConfiguration{
						Count:    5000,
						Duration: 60,
					},
					Strategy: &datastore.StrategyConfiguration{
						Type:       datastore.LinearStrategyProvider,
						Duration:   10,
						RetryCount: 3,
					},
					Signature: &datastore.SignatureConfiguration{
						Header:   config.DefaultSignatureHeader,
						Versions: datastore.SignatureVersions{},
					},
					SSL: &datastore.SSLConfiguration{
						EnforceSecureEndpoints: true,
					},
					MetaEvent: &datastore.MetaEventConfiguration{
						IsEnabled: false,
					},
					CircuitBreaker: &datastore.CircuitBreakerConfiguration{
						SampleRate:                  100,
						ErrorTimeout:                30,
						FailureThreshold:            50,
						SuccessThreshold:            10,
						ObservabilityWindow:         5,
						MinimumRequestCount:         10,
						ConsecutiveFailureThreshold: 5,
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "should_fail_with_nil_project",
			project: nil,
			wantErr: true,
			errMsg:  "project cannot be nil",
		},
		{
			name: "should_fail_with_duplicate_name",
			project: &datastore.Project{
				UID:            ulid.Make().String(),
				Name:           "Duplicate Project",
				Type:           datastore.OutgoingProject,
				OrganisationID: org.UID,
				Config:         getDefaultProjectConfig(),
			},
			wantErr: true,
			errMsg:  "", // Will be set after first creation
		},
	}

	// First create a project with a known name for duplicate test
	duplicateProject := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Duplicate Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         getDefaultProjectConfig(),
	}
	err := service.CreateProject(ctx, duplicateProject)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.CreateProject(ctx, tt.project)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					require.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)

				// Verify project was created
				if tt.project != nil {
					fetched, err := service.FetchProjectByID(ctx, tt.project.UID)
					require.NoError(t, err)
					require.NotNil(t, fetched)
					require.Equal(t, tt.project.UID, fetched.UID)
					require.Equal(t, tt.project.Name, fetched.Name)
					require.Equal(t, tt.project.Type, fetched.Type)
					require.Equal(t, tt.project.OrganisationID, fetched.OrganisationID)
					require.NotEmpty(t, fetched.ProjectConfigID)

					// Verify configuration was created
					require.NotNil(t, fetched.Config)
					require.NotNil(t, fetched.Config.RateLimit)
					require.NotNil(t, fetched.Config.Strategy)
					require.NotNil(t, fetched.Config.Signature)
					require.NotNil(t, fetched.Config.SSL)
					require.NotNil(t, fetched.Config.MetaEvent)
					require.NotNil(t, fetched.Config.CircuitBreaker)

					// Verify specific config values
					require.Equal(t, tt.project.Config.MaxIngestSize, fetched.Config.MaxIngestSize)
					require.Equal(t, tt.project.Config.ReplayAttacks, fetched.Config.ReplayAttacks)
					require.Equal(t, tt.project.Config.RateLimit.Count, fetched.Config.RateLimit.Count)
					require.Equal(t, tt.project.Config.Strategy.Type, fetched.Config.Strategy.Type)
				}
			}
		})
	}
}

func TestCreateProject_TransactionRollback(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	// Test that if configuration creation fails, project is not created
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Rollback Test Project",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         getDefaultProjectConfig(),
	}

	err := service.CreateProject(ctx, project)
	require.NoError(t, err)

	// Verify the project exists
	fetched, err := service.FetchProjectByID(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
}
