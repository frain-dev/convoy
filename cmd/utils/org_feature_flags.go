package utils

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/pkg/log"
)

func AddUpdateOrgFeatureFlagsCommand(a *cli.App) *cobra.Command {
	var enableFlags, disableFlags []string
	var orgID string

	cmd := &cobra.Command{
		Use:   "update-org-feature-flags",
		Short: "Update organization-level feature flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			if orgID == "" {
				return fmt.Errorf("org-id is required")
			}

			if len(enableFlags) == 0 && len(disableFlags) == 0 {
				return fmt.Errorf("at least one --enable or --disable flag is required")
			}

			db := a.DB

			orgRepo := postgres.NewOrgRepo(db)
			org, err := orgRepo.FetchOrganisationByID(context.Background(), orgID)
			if err != nil {
				return fmt.Errorf("failed to fetch organisation: %w", err)
			}

			log.Infof("Updating feature flags for organisation: %s (%s)", org.Name, org.UID)

			updated := 0
			errors := []string{}

			for _, flag := range enableFlags {
				flagKey := strings.ToLower(strings.TrimSpace(flag))
				if !isValidFeatureFlag(flagKey) {
					log.Warnf("Skipping invalid feature flag: %s", flag)
					continue
				}

				err := updateOrgFeatureFlag(context.Background(), db, orgID, flagKey, true)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Failed to enable %s: %v", flagKey, err))
					continue
				}

				log.Infof("Enabled feature flag: %s", flagKey)
				updated++
			}

			for _, flag := range disableFlags {
				flagKey := strings.ToLower(strings.TrimSpace(flag))
				if !isValidFeatureFlag(flagKey) {
					log.Warnf("Skipping invalid feature flag: %s", flag)
					continue
				}

				err := updateOrgFeatureFlag(context.Background(), db, orgID, flagKey, false)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Failed to disable %s: %v", flagKey, err))
					continue
				}

				log.Infof("Disabled feature flag: %s", flagKey)
				updated++
			}

			if len(errors) > 0 {
				log.Errorf("Encountered %d errors:", len(errors))
				for _, errMsg := range errors {
					log.Errorf("  - %s", errMsg)
				}
			}

			if updated > 0 {
				log.Infof("Successfully updated %d feature flag(s)", updated)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&orgID, "org-id", "", "Organization ID")
	cmd.Flags().StringSliceVar(&enableFlags, "enable", []string{}, "Feature flags to enable (comma-separated)")
	cmd.Flags().StringSliceVar(&disableFlags, "disable", []string{}, "Feature flags to disable (comma-separated)")

	return cmd
}

func updateOrgFeatureFlag(ctx context.Context, db database.Database, orgID, flagKey string, enabled bool) error {
	// Fetch feature flag from database
	featureFlag, err := postgres.FetchFeatureFlagByKey(ctx, db, flagKey)
	if err != nil {
		if err == postgres.ErrFeatureFlagNotFound {
			return fmt.Errorf("feature flag '%s' not found in database", flagKey)
		}
		return err
	}

	// Check if allow_override is true
	if !featureFlag.AllowOverride {
		return fmt.Errorf("feature flag '%s' does not allow overrides (allow_override=false)", flagKey)
	}

	// Create or update override
	override := &datastore.FeatureFlagOverride{
		FeatureFlagID: featureFlag.UID,
		OwnerType:     "organisation",
		OwnerID:       orgID,
		Enabled:       enabled,
		EnabledBy:     null.String{}, // CLI updates don't track user
	}

	if enabled {
		override.EnabledAt = null.TimeFrom(time.Now())
	}

	return postgres.UpsertFeatureFlagOverride(ctx, db, override)
}

func isValidFeatureFlag(flagKey string) bool {
	validFlags := []string{
		"circuit-breaker",
		"mtls",
		"oauth-token-exchange",
		"ip-rules",
		"retention-policy",
		"full-text-search",
	}

	return slices.Contains(validFlags, flagKey)
}
