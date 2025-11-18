//go:build integration
// +build integration

package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/cmd/utils"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
)

func TestCircuitBreakersUpdate_Integration(t *testing.T) {
	app := buildServer()
	ctx := context.Background()

	userRepo := postgres.NewUserRepo(app.A.DB)
	user := &datastore.User{UID: "cli-user-1", Email: "cli-user-1@test.local"}
	_ = userRepo.CreateUser(ctx, user)

	orgRepo := postgres.NewOrgRepo(app.A.DB)
	org := &datastore.Organisation{UID: "cli-org-1", Name: "CLI Org 1", OwnerID: user.UID}
	_ = orgRepo.CreateOrganisation(ctx, org)

	projectRepo := postgres.NewProjectRepo(app.A.DB)
	pc := datastore.DefaultProjectConfig
	pc.CircuitBreaker = &datastore.CircuitBreakerConfiguration{
		SampleRate:                  30,
		ErrorTimeout:                30,
		FailureThreshold:            70,
		SuccessThreshold:            5,
		ObservabilityWindow:         5,
		MinimumRequestCount:         10,
		ConsecutiveFailureThreshold: 10,
	}
	project := &datastore.Project{UID: "cli-proj-1", Name: "CLI Proj 1", OrganisationID: org.UID, Config: &pc}
	_ = projectRepo.CreateProject(ctx, project)

	// Seed breaker in Redis for this project (via application Redis)
	store := cb.NewRedisStore(app.A.Redis, clock.NewRealClock())
	breaker := cb.CircuitBreaker{Key: "cb-endpoint-1", TenantId: project.UID}
	require.NoError(t, store.SetMany(ctx, map[string]cb.CircuitBreaker{"breaker:cb-endpoint-1": breaker}, time.Minute))

	// Execute update via CLI command using same app deps
	cmd := utils.AddCircuitBreakersUpdateCommand(&cli.App{
		DB:     app.A.DB,
		Redis:  app.A.Redis,
		Logger: app.A.Logger,
	})
	require.NoError(t, cmd.Flags().Set("failure_threshold", "55"))
	require.NoError(t, cmd.Flags().Set("success_threshold", "6"))
	require.NoError(t, cmd.Flags().Set("minimum_request_count", "11"))
	require.NoError(t, cmd.Flags().Set("observability_window", "7"))
	require.NoError(t, cmd.Flags().Set("consecutive_failure_threshold", "4"))

	err := cmd.RunE(cmd, []string{project.UID})
	require.NoError(t, err)

	// Ensure breaker state was cleared from Redis to pick new config
	got, getErr := app.A.Redis.Get(ctx, "breaker:cb-endpoint-1").Result()
	require.Error(t, getErr)
	_ = got

	// Verify project updated
	updated, err := projectRepo.FetchProjectByID(ctx, project.UID)
	require.NoError(t, err)
	require.NotNil(t, updated.Config)
	require.NotNil(t, updated.Config.CircuitBreaker)
	require.Equal(t, uint64(55), updated.Config.CircuitBreaker.FailureThreshold)
	require.Equal(t, uint64(6), updated.Config.CircuitBreaker.SuccessThreshold)
	require.Equal(t, uint64(11), updated.Config.CircuitBreaker.MinimumRequestCount)
	require.Equal(t, uint64(7), updated.Config.CircuitBreaker.ObservabilityWindow)
	require.Equal(t, uint64(4), updated.Config.CircuitBreaker.ConsecutiveFailureThreshold)
}

func TestCircuitBreakersUpdate_EdgeCases(t *testing.T) {
	app := buildServer()
	ctx := context.Background()
	now := time.Now().UnixNano()

	userRepo := postgres.NewUserRepo(app.A.DB)
	orgRepo := postgres.NewOrgRepo(app.A.DB)
	projectRepo := postgres.NewProjectRepo(app.A.DB)

	// base seed (unique ids)
	user := &datastore.User{UID: fmt.Sprintf("cli-user-%d", now), Email: fmt.Sprintf("cli-user-%d@test.local", now)}
	_ = userRepo.CreateUser(ctx, user)
	org := &datastore.Organisation{UID: fmt.Sprintf("cli-org-%d", now), Name: "CLI Org EC", OwnerID: user.UID}
	_ = orgRepo.CreateOrganisation(ctx, org)

	t.Run("no flags -> no changes", func(t *testing.T) {
		pc := datastore.DefaultProjectConfig
		pc.CircuitBreaker = &datastore.CircuitBreakerConfiguration{
			SampleRate:                  30,
			ErrorTimeout:                30,
			FailureThreshold:            70,
			SuccessThreshold:            5,
			ObservabilityWindow:         5,
			MinimumRequestCount:         10,
			ConsecutiveFailureThreshold: 10,
		}
		project := &datastore.Project{UID: fmt.Sprintf("cli-proj-%d-a", now), Name: "CLI Proj EC A", OrganisationID: org.UID, Config: &pc}
		_ = projectRepo.CreateProject(ctx, project)

		// Seed breaker
		store := cb.NewRedisStore(app.A.Redis, clock.NewRealClock())
		breaker := cb.CircuitBreaker{Key: "ec-endpoint-a", TenantId: project.UID}
		require.NoError(t, store.SetMany(ctx, map[string]cb.CircuitBreaker{"breaker:ec-endpoint-a": breaker}, time.Minute))

		// Run update without flags
		cmd := utils.AddCircuitBreakersUpdateCommand(&cli.App{DB: app.A.DB, Redis: app.A.Redis, Logger: app.A.Logger})
		err := cmd.RunE(cmd, []string{project.UID})
		require.NoError(t, err)

		updated, err := projectRepo.FetchProjectByID(ctx, project.UID)
		require.NoError(t, err)
		require.Equal(t, uint64(70), updated.Config.CircuitBreaker.FailureThreshold)
		require.Equal(t, uint64(5), updated.Config.CircuitBreaker.SuccessThreshold)
		require.Equal(t, uint64(10), updated.Config.CircuitBreaker.MinimumRequestCount)
		require.Equal(t, uint64(5), updated.Config.CircuitBreaker.ObservabilityWindow)
		require.Equal(t, uint64(10), updated.Config.CircuitBreaker.ConsecutiveFailureThreshold)
	})

	t.Run("with project id", func(t *testing.T) {
		pc := datastore.DefaultProjectConfig
		pc.CircuitBreaker = &datastore.DefaultCircuitBreakerConfiguration
		project := &datastore.Project{UID: fmt.Sprintf("cli-proj-%d-b", now), Name: "CLI Proj EC B", OrganisationID: org.UID, Config: &pc}
		_ = projectRepo.CreateProject(ctx, project)

		store := cb.NewRedisStore(app.A.Redis, clock.NewRealClock())
		breaker := cb.CircuitBreaker{Key: "ec-endpoint-b", TenantId: project.UID}
		require.NoError(t, store.SetMany(ctx, map[string]cb.CircuitBreaker{"breaker:ec-endpoint-b": breaker}, time.Minute))

		cmd := utils.AddCircuitBreakersUpdateCommand(&cli.App{DB: app.A.DB, Redis: app.A.Redis, Logger: app.A.Logger})
		_ = cmd.Flags().Set("failure_threshold", "51")
		err := cmd.RunE(cmd, []string{project.UID})
		require.NoError(t, err)

		updated, err := projectRepo.FetchProjectByID(ctx, project.UID)
		require.NoError(t, err)
		require.Equal(t, uint64(51), updated.Config.CircuitBreaker.FailureThreshold)
	})

	t.Run("initialize missing cb config", func(t *testing.T) {
		pc := datastore.DefaultProjectConfig
		pc.CircuitBreaker = nil // simulate missing
		project := &datastore.Project{UID: fmt.Sprintf("cli-proj-%d-c", now), Name: "CLI Proj EC C", OrganisationID: org.UID, Config: &pc}
		_ = projectRepo.CreateProject(ctx, project)

		store := cb.NewRedisStore(app.A.Redis, clock.NewRealClock())
		breaker := cb.CircuitBreaker{Key: "ec-endpoint-c", TenantId: project.UID}
		require.NoError(t, store.SetMany(ctx, map[string]cb.CircuitBreaker{"breaker:ec-endpoint-c": breaker}, time.Minute))

		cmd := utils.AddCircuitBreakersUpdateCommand(&cli.App{DB: app.A.DB, Redis: app.A.Redis, Logger: app.A.Logger})
		_ = cmd.Flags().Set("success_threshold", "9")
		err := cmd.RunE(cmd, []string{project.UID})
		require.NoError(t, err)

		updated, err := projectRepo.FetchProjectByID(ctx, project.UID)
		require.NoError(t, err)
		require.NotNil(t, updated.Config.CircuitBreaker)
		require.Equal(t, uint64(9), updated.Config.CircuitBreaker.SuccessThreshold)
	})

	t.Run("out of range thresholds", func(t *testing.T) {
		// seed project + breaker
		pc := datastore.DefaultProjectConfig
		project := &datastore.Project{UID: fmt.Sprintf("cli-proj-%d-d", now), Name: "CLI Proj EC D", OrganisationID: org.UID, Config: &pc}
		_ = projectRepo.CreateProject(ctx, project)
		store := cb.NewRedisStore(app.A.Redis, clock.NewRealClock())
		breaker := cb.CircuitBreaker{Key: "ec-endpoint-d", TenantId: project.UID}
		require.NoError(t, store.SetMany(ctx, map[string]cb.CircuitBreaker{"breaker:ec-endpoint-d": breaker}, time.Minute))

		// failure_threshold > 100 should error
		cmd := utils.AddCircuitBreakersUpdateCommand(&cli.App{DB: app.A.DB, Redis: app.A.Redis, Logger: app.A.Logger})
		_ = cmd.Flags().Set("failure_threshold", "101")
		err := cmd.RunE(cmd, []string{project.UID})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failure_threshold")

		// success_threshold > 100 should error
		cmd2 := utils.AddCircuitBreakersUpdateCommand(&cli.App{DB: app.A.DB, Redis: app.A.Redis, Logger: app.A.Logger})
		_ = cmd2.Flags().Set("success_threshold", "1000")
		err = cmd2.RunE(cmd2, []string{project.UID})
		require.Error(t, err)
		require.Contains(t, err.Error(), "success_threshold")

		// observability_window == 0 should error
		cmd3 := utils.AddCircuitBreakersUpdateCommand(&cli.App{DB: app.A.DB, Redis: app.A.Redis, Logger: app.A.Logger})
		_ = cmd3.Flags().Set("observability_window", "0")
		err = cmd3.RunE(cmd3, []string{project.UID})
		require.Error(t, err)
		require.Contains(t, err.Error(), "observability_window")
	})
}
