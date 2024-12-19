package utils

import (
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/spf13/cobra"
)

func AddPartitionCommand(a *cli.App) *cobra.Command {
	var table string

	cmd := &cobra.Command{
		Use:   "partition",
		Short: "runs partition commands",
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				a.Logger.WithError(err).Fatal("Failed to load configuration")
			}

			featureFlag := fflag.NewFFlag(cfg.EnableFeatureFlag)
			if !featureFlag.CanAccessFeature(fflag.RetentionPolicy) {
				return fmt.Errorf("partitioning is only avaliable when the retention policy fflag is enabled")
			}

			if !a.Licenser.RetentionPolicy() {
				return fmt.Errorf("partitioning is only avaliable with a license key")
			}

			eventsRepo := postgres.NewEventRepo(a.DB, nil)
			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB, nil)
			deliveryAttemptsRepo := postgres.NewDeliveryAttemptRepo(a.DB)

			if table == "" {
				return fmt.Errorf("table name is required")
			}

			switch table {
			case "events":
				err := eventsRepo.PartitionEventsTable(cmd.Context())
				if err != nil {
					return err
				}
			case "event-deliveries":
				err := eventDeliveryRepo.PartitionEventDeliveriesTable(cmd.Context())
				if err != nil {
					return err
				}
			case "delivery-attempts":
				err := deliveryAttemptsRepo.PartitionDeliveryAttemptsTable(cmd.Context())
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown table %s", table)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&table, "table", "t", "", "table name")

	return cmd
}

func AddUnPartitionCommand(a *cli.App) *cobra.Command {
	var table string

	cmd := &cobra.Command{
		Use:   "unpartition",
		Short: "runs partition commands",
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				a.Logger.WithError(err).Fatal("Failed to load configuration")
			}

			featureFlag := fflag.NewFFlag(cfg.EnableFeatureFlag)
			if !featureFlag.CanAccessFeature(fflag.RetentionPolicy) {
				return fmt.Errorf("partitioning is only avaliable when the retention policy fflag is enabled")
			}

			if !a.Licenser.RetentionPolicy() {
				return fmt.Errorf("partitioning is only avaliable with a license key")
			}

			eventsRepo := postgres.NewEventRepo(a.DB, nil)
			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB, nil)
			deliveryAttemptsRepo := postgres.NewDeliveryAttemptRepo(a.DB)

			if table == "" {
				return fmt.Errorf("table name is required")
			}

			switch table {
			case "events":
				err := eventsRepo.UnPartitionEventsTable(cmd.Context())
				if err != nil {
					return err
				}
			case "event-deliveries":
				err := eventDeliveryRepo.UnPartitionEventDeliveriesTable(cmd.Context())
				if err != nil {
					return err
				}
			case "delivery-attempts":
				err := deliveryAttemptsRepo.UnPartitionDeliveryAttemptsTable(cmd.Context())
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown table %s", table)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&table, "table", "t", "", "table name")

	return cmd
}
