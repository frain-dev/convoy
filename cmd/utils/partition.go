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
	cmd := &cobra.Command{
		Use:   "partition",
		Short: "partition tables",
		Long:  "partition tables that are deleted by convoy during retention, valid tables are events, event_deliveries, delivery_attempts and events_search",
		Args:  cobra.MaximumNArgs(1),
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

			eventsRepo := postgres.NewEventRepo(a.DB)
			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB)
			deliveryAttemptsRepo := postgres.NewDeliveryAttemptRepo(a.DB)

			// if the table name isn't supplied, then we will run all of them at the same time
			if len(args) == 0 {
				err = eventsRepo.PartitionEventsTable(cmd.Context())
				if err != nil {
					return err
				}

				err = eventsRepo.PartitionEventsSearchTable(cmd.Context())
				if err != nil {
					return err
				}

				err = eventDeliveryRepo.PartitionEventDeliveriesTable(cmd.Context())
				if err != nil {
					return err
				}

				err = deliveryAttemptsRepo.PartitionDeliveryAttemptsTable(cmd.Context())
				if err != nil {
					return err
				}
			} else {
				switch args[0] {
				case "events":
					err = eventsRepo.PartitionEventsTable(cmd.Context())
					if err != nil {
						return err
					}
				case "events_search":
					err = eventsRepo.PartitionEventsSearchTable(cmd.Context())
					if err != nil {
						return err
					}
				case "event_deliveries":
					err = eventDeliveryRepo.PartitionEventDeliveriesTable(cmd.Context())
					if err != nil {
						return err
					}
				case "delivery_attempts":
					err = deliveryAttemptsRepo.PartitionDeliveryAttemptsTable(cmd.Context())
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("unknown table %s", args[0])
				}
			}

			return nil
		},
	}

	return cmd
}

func AddUnPartitionCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unpartition",
		Short: "unpartitions tables",
		Long:  "unpartition tables that are deleted by convoy during retention, valid tables are events, event_deliveries, delivery_attempts and events_search",
		Args:  cobra.MaximumNArgs(1),
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

			eventsRepo := postgres.NewEventRepo(a.DB)
			eventDeliveryRepo := postgres.NewEventDeliveryRepo(a.DB)
			deliveryAttemptsRepo := postgres.NewDeliveryAttemptRepo(a.DB)

			// if the table name isn't supplied, then we will run all of them at the same time
			if len(args) == 0 {
				err = eventsRepo.UnPartitionEventsTable(cmd.Context())
				if err != nil {
					return err
				}

				err = eventsRepo.UnPartitionEventsSearchTable(cmd.Context())
				if err != nil {
					return err
				}

				err = eventDeliveryRepo.UnPartitionEventDeliveriesTable(cmd.Context())
				if err != nil {
					return err
				}

				err = deliveryAttemptsRepo.UnPartitionDeliveryAttemptsTable(cmd.Context())
				if err != nil {
					return err
				}
			} else {
				switch args[0] {
				case "events":
					err = eventsRepo.UnPartitionEventsTable(cmd.Context())
					if err != nil {
						return err
					}
				case "events_search":
					err = eventsRepo.UnPartitionEventsSearchTable(cmd.Context())
					if err != nil {
						return err
					}
				case "event_deliveries":
					err = eventDeliveryRepo.UnPartitionEventDeliveriesTable(cmd.Context())
					if err != nil {
						return err
					}
				case "delivery_attempts":
					err = deliveryAttemptsRepo.UnPartitionDeliveryAttemptsTable(cmd.Context())
					if err != nil {
						return err
					}
				default:
					return fmt.Errorf("unknown table %s", args[0])
				}
			}

			return nil
		},
	}

	return cmd
}
