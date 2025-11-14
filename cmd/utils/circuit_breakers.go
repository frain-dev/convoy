package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/spf13/cobra"
)

func AddCircuitBreakersCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "circuit-breakers",
		Short: "manage circuit breakers",
		Long:  "manage circuit breakers for endpoints",
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "false",
		},
	}

	cmd.AddCommand(AddCircuitBreakersGetCommand(a))
	cmd.AddCommand(AddCircuitBreakersUpdateCommand(a))

	return cmd
}

func AddCircuitBreakersGetCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [endpoint-id]",
		Short: "get circuit breaker information",
		Long:  "get detailed information about a specific circuit breaker",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			breakerID := args[0]

			// Remove the "breaker:" prefix if present
			breakerID = strings.TrimPrefix(breakerID, "breaker:")

			// Create circuit breaker manager with config provider
			cbManager, err := cb.NewCircuitBreakerManager(
				cb.ConfigProviderOption(func(projectID string) *cb.CircuitBreakerConfig {
					// For get command, we don't have projectID yet, so use defaults
					// The actual project config will be fetched when needed
					return &cb.CircuitBreakerConfig{
						SampleRate:                  datastore.DefaultCircuitBreakerConfiguration.SampleRate,
						BreakerTimeout:              datastore.DefaultCircuitBreakerConfiguration.ErrorTimeout,
						FailureThreshold:            datastore.DefaultCircuitBreakerConfiguration.FailureThreshold,
						SuccessThreshold:            datastore.DefaultCircuitBreakerConfiguration.SuccessThreshold,
						ObservabilityWindow:         datastore.DefaultCircuitBreakerConfiguration.ObservabilityWindow,
						MinimumRequestCount:         datastore.DefaultCircuitBreakerConfiguration.MinimumRequestCount,
						ConsecutiveFailureThreshold: datastore.DefaultCircuitBreakerConfiguration.ConsecutiveFailureThreshold,
					}
				}),
				cb.StoreOption(cb.NewRedisStore(a.Redis, clock.NewRealClock())),
				cb.ClockOption(clock.NewRealClock()),
				cb.LoggerOption(log.NewLogger(os.Stdout)),
			)
			if err != nil {
				return fmt.Errorf("failed to create circuit breaker manager: %v", err)
			}

			// Get circuit breaker
			breaker, err := cbManager.GetCircuitBreakerWithError(context.Background(), breakerID)
			if err != nil {
				return fmt.Errorf("failed to get circuit breaker: %v", err)
			}

			if breaker == nil {
				return fmt.Errorf("circuit breaker not found")
			}

			// Format output
			output := map[string]interface{}{
				"key":                  fmt.Sprintf("breaker:%s", breakerID),
				"tenant_id":            breaker.TenantId,
				"state":                breaker.State,
				"requests":             breaker.Requests,
				"failure_rate":         breaker.FailureRate,
				"success_rate":         breaker.SuccessRate,
				"will_reset_at":        breaker.WillResetAt,
				"total_failures":       breaker.TotalFailures,
				"total_successes":      breaker.TotalSuccesses,
				"consecutive_failures": breaker.ConsecutiveFailures,
				"notifications_sent":   breaker.NotificationsSent,
			}

			// Print as JSON
			jsonOutput, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %v", err)
			}

			fmt.Println(string(jsonOutput))
			return nil
		},
	}

	return cmd
}

func AddCircuitBreakersUpdateCommand(a *cli.App) *cobra.Command {
	var (
		failureThreshold            uint64
		successThreshold            uint64
		minimumRequestCount         uint64
		observabilityWindow         uint64
		consecutiveFailureThreshold uint64
	)

	cmd := &cobra.Command{
		Use:   "update [project-id]",
		Short: "update circuit breaker configuration",
		Long:  "update circuit breaker configuration for a specific project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectID := args[0]

			// Validate flag ranges before any work
			if cmd.Flags().Changed("failure_threshold") {
				if failureThreshold > 100 {
					return fmt.Errorf("failure_threshold must be between 0 and 100")
				}
			}
			if cmd.Flags().Changed("success_threshold") {
				if successThreshold > 100 {
					return fmt.Errorf("success_threshold must be between 0 and 100")
				}
			}
			if cmd.Flags().Changed("observability_window") {
				if observabilityWindow == 0 {
					return fmt.Errorf("observability_window must be greater than 0")
				}
			}

			// Get current project configuration
			projectRepo := postgres.NewProjectRepo(a.DB)
			project, err := projectRepo.FetchProjectByID(context.Background(), projectID)
			if err != nil {
				return fmt.Errorf("failed to fetch project: %v", err)
			}

			// Initialize project config if it doesn't exist
			if project.Config == nil {
				project.Config = &datastore.ProjectConfig{}
			}

			// Initialize circuit breaker configuration if it doesn't exist
			if project.Config.CircuitBreaker == nil {
				project.Config.CircuitBreaker = &datastore.DefaultCircuitBreakerConfiguration
			}

			// Update circuit breaker configuration if flags are provided
			updated := false
			if cmd.Flags().Changed("failure_threshold") {
				project.Config.CircuitBreaker.FailureThreshold = failureThreshold
				updated = true
			}
			if cmd.Flags().Changed("success_threshold") {
				project.Config.CircuitBreaker.SuccessThreshold = successThreshold
				updated = true
			}
			if cmd.Flags().Changed("minimum_request_count") {
				project.Config.CircuitBreaker.MinimumRequestCount = minimumRequestCount
				updated = true
			}
			if cmd.Flags().Changed("observability_window") {
				project.Config.CircuitBreaker.ObservabilityWindow = observabilityWindow
				updated = true
			}
			if cmd.Flags().Changed("consecutive_failure_threshold") {
				project.Config.CircuitBreaker.ConsecutiveFailureThreshold = consecutiveFailureThreshold
				updated = true
			}

			if updated {
				// Validate updated configuration using circuit breaker validation rules
				validateCfg := cb.CircuitBreakerConfig{
					SampleRate:                  project.Config.CircuitBreaker.SampleRate,
					BreakerTimeout:              project.Config.CircuitBreaker.ErrorTimeout,
					FailureThreshold:            project.Config.CircuitBreaker.FailureThreshold,
					SuccessThreshold:            project.Config.CircuitBreaker.SuccessThreshold,
					ObservabilityWindow:         project.Config.CircuitBreaker.ObservabilityWindow,
					MinimumRequestCount:         project.Config.CircuitBreaker.MinimumRequestCount,
					ConsecutiveFailureThreshold: project.Config.CircuitBreaker.ConsecutiveFailureThreshold,
				}
				if err := validateCfg.Validate(); err != nil {
					return fmt.Errorf("invalid circuit breaker configuration: %v", err)
				}

				// Update project configuration
				err = projectRepo.UpdateProject(context.Background(), project)
				if err != nil {
					return fmt.Errorf("failed to update project configuration: %v", err)
				}

				// Reset all circuit breakers for this project in Redis so new config takes immediate effect
				// Get all breaker keys and filter by project ID
				store := cb.NewRedisStore(a.Redis, clock.NewRealClock())
				keys, err := store.Keys(context.Background(), "breaker:")
				if err == nil {
					for _, key := range keys {
						// Get breaker to check TenantId
						breakerData, err := store.GetOne(context.Background(), key)
						if err == nil {
							breaker, err := cb.NewCircuitBreakerFromStore([]byte(breakerData), log.NewLogger(os.Stdout))
							if err == nil && breaker.TenantId == projectID {
								// Ignore errors here; it's best-effort
								_ = a.Redis.Del(context.Background(), key).Err()
							}
						}
					}
				}

				fmt.Println("Circuit breaker configuration updated successfully")
			} else {
				fmt.Println("No changes made to circuit breaker configuration")
			}

			// Display current configuration
			fmt.Println("\nCurrent circuit breaker configuration:")
			fmt.Printf("sample_rate                   | %d -- cannot be changed\n", project.Config.CircuitBreaker.SampleRate)
			fmt.Printf("error_timeout                 | %d -- cannot be changed\n", project.Config.CircuitBreaker.ErrorTimeout)
			fmt.Printf("failure_threshold             | %d\n", project.Config.CircuitBreaker.FailureThreshold)
			fmt.Printf("success_threshold             | %d\n", project.Config.CircuitBreaker.SuccessThreshold)
			fmt.Printf("observability_window          | %d\n", project.Config.CircuitBreaker.ObservabilityWindow)
			fmt.Printf("minimum_request_count         | %d\n", project.Config.CircuitBreaker.MinimumRequestCount)
			fmt.Printf("consecutive_failure_threshold | %d\n", project.Config.CircuitBreaker.ConsecutiveFailureThreshold)

			return nil
		},
	}

	// Add flags for configurable parameters
	cmd.Flags().Uint64Var(&failureThreshold, "failure_threshold", 0, "failure threshold percentage")
	cmd.Flags().Uint64Var(&successThreshold, "success_threshold", 0, "success threshold percentage")
	cmd.Flags().Uint64Var(&minimumRequestCount, "minimum_request_count", 0, "minimum request count")
	cmd.Flags().Uint64Var(&observabilityWindow, "observability_window", 0, "observability window in minutes")
	cmd.Flags().Uint64Var(&consecutiveFailureThreshold, "consecutive_failure_threshold", 0, "consecutive failure threshold")

	return cmd
}
