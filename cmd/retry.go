package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/datastore"

	"github.com/spf13/cobra"
)

func addRetryCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry",
		Short: "Get info about queue",
	}

	cmd.AddCommand(getQueueLength(a))
	return cmd
}

//Get queue length, number of entries in the stream
func requeeEventDeliveriedByStatus(a *app) *cobra.Command {
	var status string
	var timeInterval string

	cmd := &cobra.Command{
		Use:   "retry",
		Short: "retry event deliveries with a particular status in a timeframe",
		RunE: func(cmd *cobra.Command, args []string) error {

			ctx := context.Background()
			d, err := time.ParseDuration(timeInterval)
			if err != nil {
				log.WithError(err).Fatal("failed to parse time duration")
			}

			now := time.Now()
			then := now.Add(-d)

			s := datastore.EventDeliveryStatus(status)
			searchParams := datastore.SearchParams{
				CreatedAtStart: int64(primitive.NewDateTimeFromTime(then)),
				CreatedAtEnd:   int64(primitive.NewDateTimeFromTime(now)),
			}

			pageable := datastore.Pageable{
				Page:    0,
				PerPage: 1000,
				Sort:    -1,
			}

			processedDeliveries := []datastore.EventDelivery{}

			count := 0

			for {
				deliveries, paginationData, err := a.eventDeliveryRepo.LoadEventDeliveriesPaged(ctx, "", "", "", []datastore.EventDeliveryStatus{s}, searchParams, pageable)
				if err != nil {
					log.WithError(err).Fatalf("succesfully requeued %d event deliveries, encountered error fetching page %d", count, pageable.Page)
				}

				processedDeliveries = append(processedDeliveries, deliveries...)

				count += len(processedDeliveries)
				pageable.Page++
				return nil
			}

		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Log time interval")
	cmd.Flags().StringVar(&timeInterval, "time", "", " time interval")
	return cmd
}
