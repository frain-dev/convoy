package retry

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/worker/task"

	"github.com/spf13/cobra"
)

func AddRetryCommand(a *cli.App) *cobra.Command {
	var status string
	var timeInterval string
	var eventId string

	cmd := &cobra.Command{
		Use:   "retry",
		Short: "retry event deliveries with a particular status in a timeframe",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.Fatalf("Error getting config: %v", err)
			}

			if len(cfg.Redis.BuildDsn()) == 0 {
				log.WithError(err).Fatal("Queue type error: Command is available for redis queue only.")
			}

			statuses := []datastore.EventDeliveryStatus{datastore.EventDeliveryStatus(status)}
			task.RetryEventDeliveries(a.DB, a.Cache, a.Queue, statuses, timeInterval, eventId)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Status of event deliveries to requeue")
	cmd.Flags().StringVar(&timeInterval, "time", "", "Time interval")
	cmd.Flags().StringVar(&eventId, "eventid", "", "Requeue the informed eventId")
	return cmd
}
