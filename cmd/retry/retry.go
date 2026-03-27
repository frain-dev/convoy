package retry

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/worker/task"
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
				fmt.Fprintf(os.Stderr, "Error getting config: %v\n", err)
				os.Exit(1)
			}

			if len(cfg.Redis.BuildDsn()) == 0 {
				fmt.Fprintf(os.Stderr, "Queue type error: Command is available for redis queue only: %v\n", err)
				os.Exit(1)
			}

			statuses := []datastore.EventDeliveryStatus{datastore.EventDeliveryStatus(status)}
			task.RetryEventDeliveries(a.Logger, a.DB, a.Queue, statuses, timeInterval, eventId)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Status of event deliveries to requeue")
	cmd.Flags().StringVar(&timeInterval, "time", "", "Time interval")
	cmd.Flags().StringVar(&eventId, "eventid", "", "Requeue the informed eventId")
	return cmd
}
