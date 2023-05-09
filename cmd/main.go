package main

import (
	"os"
	_ "time/tzdata"

	configCmd "github.com/frain-dev/convoy/cmd/config"
	"github.com/frain-dev/convoy/cmd/hooks"
	"github.com/frain-dev/convoy/cmd/ingest"
	"github.com/frain-dev/convoy/cmd/migrate"
	"github.com/frain-dev/convoy/cmd/retry"
	"github.com/frain-dev/convoy/cmd/scheduler"
	"github.com/frain-dev/convoy/cmd/server"
	"github.com/frain-dev/convoy/cmd/stream"
	"github.com/frain-dev/convoy/cmd/version"
	"github.com/frain-dev/convoy/cmd/worker"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/sirupsen/logrus"

	"github.com/frain-dev/convoy/internal/pkg/cli"

	"github.com/frain-dev/convoy"
)

func main() {
	slog := logrus.New()
	slog.Out = os.Stdout

	err := os.Setenv("TZ", "") // Use UTC by default :)
	if err != nil {
		slog.Fatal("failed to set env - ", err)
	}

	app := &cli.App{}
	app.Version = convoy.GetVersionFromFS(convoy.F)
	db := &postgres.Postgres{}

	c := cli.NewCli(app, db)

	var dbPort int
	var dbDsn string
	var dbType string
	var dbHost string
	var dbScheme string
	var dbUsername string
	var dbPassword string
	var dbDatabase string

	var queuePort int
	var queueDsn string
	var queueHost string
	var queueType string
	var queueScheme string
	var queueUsername string
	var queuePassword string
	var queueDatabase string

	var configFile string

	c.Flags().StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")

	// db config
	c.Flags().StringVar(&dbDsn, "db-dsn", "", "Database dsn")
	c.Flags().StringVar(&dbHost, "db-host", "localhost", "Database Host")
	c.Flags().StringVar(&dbType, "db-type", "postgres", "Database provider")
	c.Flags().StringVar(&dbScheme, "db-scheme", "postgres", "Database Scheme")
	c.Flags().StringVar(&dbUsername, "db-username", "postgres", "Database Username")
	c.Flags().StringVar(&dbPassword, "db-password", "postgres", "Database Password")
	c.Flags().StringVar(&dbDatabase, "db-database", "convoy", "Database database")
	c.Flags().IntVar(&dbPort, "db-port", 6379, "Database Port")

	// redis config
	c.Flags().StringVar(&queueDsn, "queue-dsn", "", "Queue dsn")
	c.Flags().StringVar(&queueHost, "queue-host", "localhost", "Queue Host")
	c.Flags().StringVar(&queueType, "queue-type", "redis", "Queue provider")
	c.Flags().StringVar(&queueScheme, "queue-scheme", "redis", "Queue Scheme")
	c.Flags().StringVar(&queueUsername, "queue-username", "", "Queue Username")
	c.Flags().StringVar(&queuePassword, "queue-password", "", "Queue Password")
	c.Flags().StringVar(&queueDatabase, "queue-database", "0", "Queue database")
	c.Flags().IntVar(&queuePort, "queue-port", 5432, "Queue Port")

	c.PersistentPreRunE(hooks.PreRun(app, db))
	c.PersistentPostRunE(hooks.PostRun(app, db))

	c.AddCommand(version.AddVersionCommand())
	c.AddCommand(server.AddServerCommand(app))
	c.AddCommand(worker.AddWorkerCommand(app))
	c.AddCommand(retry.AddRetryCommand(app))
	c.AddCommand(scheduler.AddSchedulerCommand(app))
	c.AddCommand(migrate.AddMigrateCommand(app))
	c.AddCommand(configCmd.AddConfigCommand(app))
	c.AddCommand(stream.AddStreamCommand(app))
	c.AddCommand(ingest.AddIngestCommand(app))

	if err := c.Execute(); err != nil {
		slog.Fatal(err)
	}
}
