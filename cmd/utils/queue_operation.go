package utils

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/queue/drain"
)

// Queue commands use the same controller and durable idempotency contract as
// the dashboard. Status never starts consumers or changes an operation.
func AddQueueOperationCommand(app *cli.App) *cobra.Command {
	var store, purpose, key, configurationRevision, operationID, action string
	var revision int64
	var resumePaused bool
	command := &cobra.Command{Use: "queue-operation [status|begin|command|history]", Short: "Review and control durable queue maintenance", Args: cobra.ExactArgs(1)}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		if app.Broker == nil || app.Broker.Drain == nil {
			return errors.New("queue drain is not configured")
		}
		if app.Licenser == nil || !app.Licenser.AsynqMonitoring() {
			return errors.New("queue monitoring entitlement is required")
		}
		controller := app.Broker.Drain
		switch store {
		case "active":
		case "previous":
			if app.Broker.Source == nil {
				return errors.New("no previous queue is configured")
			}
			controller = app.Broker.Source.Drain
		default:
			return errors.New("store must be active or previous")
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
		defer cancel()
		ctx = drain.WithActor(ctx, "cli")
		var result interface{}
		var err error
		switch args[0] {
		case "status":
			result, err = controller.Status(ctx)
		case "begin":
			result, err = controller.BeginWithOptions(ctx, drain.Purpose(purpose), key, configurationRevision, resumePaused)
		case "command":
			result, err = controller.Command(ctx, operationID, revision, action)
		case "history":
			result, err = controller.Repository.History(ctx, controller.Target.Scope, operationID)
		default:
			return errors.New("expected status, begin, command or history")
		}
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(result)
	}
	flags := command.Flags()
	flags.StringVar(&store, "store", "active", "Configured store: active or previous")
	flags.StringVar(&purpose, "purpose", "", "Operation purpose: quiesce, drain or drain_previous")
	flags.StringVar(&key, "idempotency-key", "", "Unique request key; reuse when retrying the same begin")
	flags.StringVar(&configurationRevision, "configuration-revision", "", "Configuration revision returned by status")
	flags.StringVar(&operationID, "operation-id", "", "Durable operation ID")
	flags.Int64Var(&revision, "revision", 0, "Expected operation revision returned by status")
	flags.StringVar(&action, "action", "", "Command: stop, resume_drain or resume_traffic")
	flags.BoolVar(&resumePaused, "resume-paused", false, "Temporarily consume paused queues, restoring pause on stop or completion")
	return command
}
