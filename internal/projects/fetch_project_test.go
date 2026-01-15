package projects

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestFetchProjectByID(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	// Create a test project
	project := seedProject(t, db, org)

	tests := []struct {
		name    string
		id      string
		want    *datastore.Project
		wantErr bool
		errType error
	}{
		{
			name:    "should_fetch_existing_project",
			id:      project.UID,
			want:    project,
			wantErr: false,
		},
		{
			name:    "should_return_error_for_non_existent_project",
			id:      ulid.Make().String(),
			want:    nil,
			wantErr: true,
			errType: datastore.ErrProjectNotFound,
		},
		{
			name:    "should_return_error_for_empty_id",
			id:      "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.FetchProjectByID(ctx, tt.id)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != nil {
					require.ErrorIs(t, err, tt.errType)
				}
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				require.Equal(t, tt.want.UID, got.UID)
				require.Equal(t, tt.want.Name, got.Name)
				require.Equal(t, tt.want.Type, got.Type)
				require.Equal(t, tt.want.OrganisationID, got.OrganisationID)

				// Verify nested config is properly loaded
				require.NotNil(t, got.Config)
				require.NotNil(t, got.Config.RateLimit)
				require.NotNil(t, got.Config.Strategy)
				require.NotNil(t, got.Config.Signature)
				require.NotNil(t, got.Config.SSL)
				require.NotNil(t, got.Config.MetaEvent)
				require.NotNil(t, got.Config.CircuitBreaker)
			}
		})
	}
}

func TestFetchProjectByID_ConfigMapping(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	// Create a project with specific config values
	project := seedProjectWithCustomConfig(t, db, org)

	// Fetch and verify all config fields are correctly mapped
	fetched, err := service.FetchProjectByID(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)

	// Verify rate limit config
	require.Equal(t, project.Config.RateLimit.Count, fetched.Config.RateLimit.Count)
	require.Equal(t, project.Config.RateLimit.Duration, fetched.Config.RateLimit.Duration)

	// Verify strategy config
	require.Equal(t, project.Config.Strategy.Type, fetched.Config.Strategy.Type)
	require.Equal(t, project.Config.Strategy.Duration, fetched.Config.Strategy.Duration)
	require.Equal(t, project.Config.Strategy.RetryCount, fetched.Config.Strategy.RetryCount)

	// Verify signature config
	require.Equal(t, project.Config.Signature.Header, fetched.Config.Signature.Header)

	// Verify SSL config
	require.Equal(t, project.Config.SSL.EnforceSecureEndpoints, fetched.Config.SSL.EnforceSecureEndpoints)

	// Verify meta event config
	require.Equal(t, project.Config.MetaEvent.IsEnabled, fetched.Config.MetaEvent.IsEnabled)

	// Verify circuit breaker config
	require.Equal(t, project.Config.CircuitBreaker.SampleRate, fetched.Config.CircuitBreaker.SampleRate)
	require.Equal(t, project.Config.CircuitBreaker.ErrorTimeout, fetched.Config.CircuitBreaker.ErrorTimeout)
	require.Equal(t, project.Config.CircuitBreaker.FailureThreshold, fetched.Config.CircuitBreaker.FailureThreshold)
	require.Equal(t, project.Config.CircuitBreaker.SuccessThreshold, fetched.Config.CircuitBreaker.SuccessThreshold)
	require.Equal(t, project.Config.CircuitBreaker.ObservabilityWindow, fetched.Config.CircuitBreaker.ObservabilityWindow)
	require.Equal(t, project.Config.CircuitBreaker.MinimumRequestCount, fetched.Config.CircuitBreaker.MinimumRequestCount)
	require.Equal(t, project.Config.CircuitBreaker.ConsecutiveFailureThreshold, fetched.Config.CircuitBreaker.ConsecutiveFailureThreshold)
}
