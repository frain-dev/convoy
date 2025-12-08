package utils

import (
	"context"
	"errors"
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
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/pkg/log"
)

var (
	ErrOrgIDRequired       = errors.New("org-id is required")
	ErrNoFlagsProvided     = errors.New("at least one --enable or --disable flag is required")
	ErrFeatureFlagNotFound = errors.New("feature flag not found in database")
	ErrOverrideNotAllowed  = errors.New("feature flag does not allow overrides")
)

func AddUpdateOrgFeatureFlagsCommand(a *cli.App) *cobra.Command {
	var enableFlags, disableFlags []string
	var orgID string

	cmd := &cobra.Command{
		Use:   "update-org-feature-flags",
		Short: "Update organization-level feature flags",
		RunE: func(cmd *cobra.Command, args []string) error {
			if orgID == "" {
				return ErrOrgIDRequired
			}

			if len(enableFlags) == 0 && len(disableFlags) == 0 {
				return ErrNoFlagsProvided
			}

			db := a.DB

			orgRepo := postgres.NewOrgRepo(db)
			org, err := orgRepo.FetchOrganisationByID(context.Background(), orgID)
			if err != nil {
				return fmt.Errorf("failed to fetch organisation: %w", err)
			}

			log.Infof("Updating feature flags for organisation: %s (%s)", org.Name, org.UID)

			errorList := processFeatureFlags(context.Background(), db, orgID, enableFlags, true)
			errorList = append(errorList, processFeatureFlags(context.Background(), db, orgID, disableFlags, false)...)

			if len(errorList) > 0 {
				log.Errorf("Encountered %d errors:", len(errorList))
				for _, errMsg := range errorList {
					log.Errorf("  - %s", errMsg)
				}
			}

			updated := len(enableFlags) + len(disableFlags) - len(errorList)
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

func processFeatureFlags(ctx context.Context, db database.Database, orgID string, flags []string, enabled bool) []string {
	var errorList []string
	for _, flag := range flags {
		flagKey := strings.ToLower(strings.TrimSpace(flag))
		if !isValidFeatureFlag(flagKey) {
			log.Warnf("Skipping invalid feature flag: %s", flag)
			continue
		}

		err := updateOrgFeatureFlag(ctx, db, orgID, flagKey, enabled)
		if err != nil {
			action := "enable"
			if !enabled {
				action = "disable"
			}
			errorList = append(errorList, fmt.Sprintf("Failed to %s %s: %v", action, flagKey, err))
			continue
		}

		action := "Enabled"
		if !enabled {
			action = "Disabled"
		}
		log.Infof("%s feature flag: %s", action, flagKey)
	}
	return errorList
}

func updateOrgFeatureFlag(ctx context.Context, db database.Database, orgID, flagKey string, enabled bool) error {
	featureFlag, err := postgres.FetchFeatureFlagByKey(ctx, db, flagKey)
	if err != nil {
		if errors.Is(err, postgres.ErrFeatureFlagNotFound) {
			return fmt.Errorf("%w: %s", ErrFeatureFlagNotFound, flagKey)
		}
		return err
	}

	flagKeyEnum := fflag.FeatureFlagKey(flagKey)
	if fflag.IsEarlyAdopterFeature(flagKeyEnum) {
		feature := &datastore.EarlyAdopterFeature{
			OrganisationID: orgID,
			FeatureKey:     flagKey,
			Enabled:        enabled,
		}
		if enabled {
			feature.EnabledAt = null.TimeFrom(time.Now())
		}
		return postgres.UpsertEarlyAdopterFeature(ctx, db, feature)
	}

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
