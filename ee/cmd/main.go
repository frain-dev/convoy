package main

import (
	"os"

	"github.com/frain-dev/convoy"
	configCmd "github.com/frain-dev/convoy/cmd/config"
	"github.com/frain-dev/convoy/cmd/hooks"
	"github.com/frain-dev/convoy/cmd/ingest"
	"github.com/frain-dev/convoy/cmd/migrate"
	"github.com/frain-dev/convoy/cmd/retry"
	"github.com/frain-dev/convoy/cmd/scheduler"
	"github.com/frain-dev/convoy/cmd/server"
	"github.com/frain-dev/convoy/cmd/version"
	"github.com/frain-dev/convoy/cmd/worker"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/ee"
	"github.com/frain-dev/convoy/ee/cmd/domain"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/sirupsen/logrus"
)

func main() {
	slog := logrus.New()
	slog.Out = os.Stdout

	err := os.Setenv("TZ", "") // Use UTC by default :)
	if err != nil {
		slog.Fatal("failed to set env - ", err)
	}

	app := &cli.App{}
	app.Version = convoy.GetVersionFromFS(ee.F)
	db := &postgres.Postgres{}

	cli := cli.NewCli(app, db)

	var redisDsn string
	var dbDsn string
	var queue string
	var configFile string

	cli.Flags().StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")
	cli.Flags().StringVar(&queue, "queue", "", "Queue provider (\"redis\")")
	cli.Flags().StringVar(&dbDsn, "db", "", "Postgres database dsn")
	cli.Flags().StringVar(&redisDsn, "redis", "", "Redis dsn")

	cli.PersistentPreRunE(hooks.PreRun(app, db))
	cli.PersistentPostRunE(hooks.PostRun(app, db))

	// Add Sub Commands
	cli.AddCommand(version.AddVersionCommand())
	cli.AddCommand(server.AddServerCommand(app))
	cli.AddCommand(worker.AddWorkerCommand(app))
	cli.AddCommand(retry.AddRetryCommand(app))
	cli.AddCommand(scheduler.AddSchedulerCommand(app))
	cli.AddCommand(migrate.AddMigrateCommand(app))
	cli.AddCommand(configCmd.AddConfigCommand(app))
	cli.AddCommand(domain.AddDomainCommand(app))
	cli.AddCommand(ingest.AddIngestCommand(app))

	if err := cli.Execute(); err != nil {
		slog.Fatal(err)
	}
}
