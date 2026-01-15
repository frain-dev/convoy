package projects

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

func TestUpdateProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	tests := []struct {
		name    string
		setup   func() *datastore.Project
		update  func(*datastore.Project)
		wantErr bool
		verify  func(*testing.T, *datastore.Project, *datastore.Project)
	}{
		{
			name: "should_update_project_name",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.Name = "Updated Project Name"
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.Equal(t, "Updated Project Name", updated.Name)
			},
		},
		{
			name: "should_update_logo_url",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.LogoURL = "https://example.com/new-logo.png"
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.Equal(t, "https://example.com/new-logo.png", updated.LogoURL)
			},
		},
		{
			name: "should_update_retained_events",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.RetainedEvents = 1000
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.Equal(t, 1000, updated.RetainedEvents)
			},
		},
		{
			name: "should_update_config_max_ingest_size",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.Config.MaxIngestSize = 10485760 // 10MB
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.Equal(t, uint64(10485760), updated.Config.MaxIngestSize)
			},
		},
		{
			name: "should_update_config_replay_attacks",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.Config.ReplayAttacks = true
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.True(t, updated.Config.ReplayAttacks)
			},
		},
		{
			name: "should_update_rate_limit_config",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.Config.RateLimit.Count = 10000
				p.Config.RateLimit.Duration = 120
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.Equal(t, 10000, updated.Config.RateLimit.Count)
				require.Equal(t, uint64(120), updated.Config.RateLimit.Duration)
			},
		},
		{
			name: "should_update_strategy_config",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.Config.Strategy.Type = datastore.ExponentialStrategyProvider
				p.Config.Strategy.Duration = 20
				p.Config.Strategy.RetryCount = 5
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.Equal(t, datastore.ExponentialStrategyProvider, updated.Config.Strategy.Type)
				require.Equal(t, uint64(20), updated.Config.Strategy.Duration)
				require.Equal(t, uint64(5), updated.Config.Strategy.RetryCount)
			},
		},
		{
			name: "should_update_signature_header",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.Config.Signature.Header = config.SignatureHeaderProvider("X-Custom-Signature")
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.Equal(t, config.SignatureHeaderProvider("X-Custom-Signature"), updated.Config.Signature.Header)
			},
		},
		{
			name: "should_update_ssl_config",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.Config.SSL.EnforceSecureEndpoints = true
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.True(t, updated.Config.SSL.EnforceSecureEndpoints)
			},
		},
		{
			name: "should_update_circuit_breaker_config",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			update: func(p *datastore.Project) {
				p.Config.CircuitBreaker.SampleRate = 200
				p.Config.CircuitBreaker.ErrorTimeout = 60
				p.Config.CircuitBreaker.FailureThreshold = 75
				p.Config.CircuitBreaker.SuccessThreshold = 20
			},
			wantErr: false,
			verify: func(t *testing.T, original, updated *datastore.Project) {
				require.Equal(t, uint64(200), updated.Config.CircuitBreaker.SampleRate)
				require.Equal(t, uint64(60), updated.Config.CircuitBreaker.ErrorTimeout)
				require.Equal(t, uint64(75), updated.Config.CircuitBreaker.FailureThreshold)
				require.Equal(t, uint64(20), updated.Config.CircuitBreaker.SuccessThreshold)
			},
		},
		{
			name: "should_fail_with_nil_project",
			setup: func() *datastore.Project {
				return nil
			},
			update:  func(p *datastore.Project) {},
			wantErr: true,
		},
		{
			name: "should_fail_with_non_existent_project",
			setup: func() *datastore.Project {
				p := seedProject(t, db, org)
				return p
			},
			update: func(p *datastore.Project) {
				p.UID = ulid.Make().String() // Change to non-existent ID
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := tt.setup()
			if original == nil {
				err := service.UpdateProject(ctx, nil)
				require.Error(t, err)
				return
			}

			// Store a copy of the original for comparison
			originalCopy, err := service.FetchProjectByID(ctx, original.UID)
			require.NoError(t, err)

			// Apply updates
			tt.update(original)

			// Perform update
			err = service.UpdateProject(ctx, original)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Fetch updated project
				updated, err := service.FetchProjectByID(ctx, original.UID)
				require.NoError(t, err)
				require.NotNil(t, updated)

				// Verify the update
				if tt.verify != nil {
					tt.verify(t, originalCopy, updated)
				}
			}
		})
	}
}

func TestUpdateProject_DisableEndpoint(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)

	// Create an endpoint for the project
	endpoint := seedEndpoint(t, db, project, datastore.InactiveEndpointStatus)

	// Update project to enable endpoints (DisableEndpoint = false)
	project.Config.DisableEndpoint = false
	err := service.UpdateProject(ctx, project)
	require.NoError(t, err)

	// Verify endpoint status was updated (this part may need endpoint repo to verify)
	// For now, we just verify the update operation succeeded
	updated, err := service.FetchProjectByID(ctx, project.UID)
	require.NoError(t, err)
	require.False(t, updated.Config.DisableEndpoint)

	// Clean up endpoint
	_ = endpoint
}

func TestUpdateProject_AllFieldsCombined(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)

	// Update multiple fields at once
	project.Name = "Fully Updated Project"
	project.LogoURL = "https://example.com/updated-logo.png"
	project.RetainedEvents = 2000
	project.Config.MaxIngestSize = 15728640 // 15MB
	project.Config.ReplayAttacks = true
	project.Config.DisableEndpoint = false
	project.Config.RateLimit.Count = 15000
	project.Config.Strategy.Type = datastore.ExponentialStrategyProvider
	project.Config.SSL.EnforceSecureEndpoints = true

	err := service.UpdateProject(ctx, project)
	require.NoError(t, err)

	// Fetch and verify all updates
	updated, err := service.FetchProjectByID(ctx, project.UID)
	require.NoError(t, err)

	require.Equal(t, "Fully Updated Project", updated.Name)
	require.Equal(t, "https://example.com/updated-logo.png", updated.LogoURL)
	require.Equal(t, 2000, updated.RetainedEvents)
	require.Equal(t, uint64(15728640), updated.Config.MaxIngestSize)
	require.True(t, updated.Config.ReplayAttacks)
	require.False(t, updated.Config.DisableEndpoint)
	require.Equal(t, 15000, updated.Config.RateLimit.Count)
	require.Equal(t, datastore.ExponentialStrategyProvider, updated.Config.Strategy.Type)
	require.True(t, updated.Config.SSL.EnforceSecureEndpoints)
}
