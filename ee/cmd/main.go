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
	"github.com/frain-dev/convoy/cmd/stream"
	"github.com/frain-dev/convoy/cmd/version"
	"github.com/frain-dev/convoy/cmd/worker"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/ee"
	"github.com/frain-dev/convoy/ee/cmd/domain"
	"github.com/frain-dev/convoy/ee/cmd/server"
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

	c := cli.NewCli(app)

	var dbPort int
	var dbType string
	var dbHost string
	var dbScheme string
	var dbUsername string
	var dbPassword string
	var dbDatabase string

	var redisPort int
	var redisHost string
	var redisType string
	var redisScheme string
	var redisUsername string
	var redisPassword string
	var redisDatabase string

	var configFile string

	c.Flags().StringVar(&configFile, "config", "./convoy.json", "Configuration file for convoy")

	// db config
	c.Flags().StringVar(&dbHost, "db-host", "", "Database Host")
	c.Flags().StringVar(&dbType, "db-type", "", "Database provider")
	c.Flags().StringVar(&dbScheme, "db-scheme", "", "Database Scheme")
	c.Flags().StringVar(&dbUsername, "db-username", "", "Database Username")
	c.Flags().StringVar(&dbPassword, "db-password", "", "Database Password")
	c.Flags().StringVar(&dbDatabase, "db-database", "", "Database Database")
	c.Flags().StringVar(&dbDatabase, "db-options", "", "Database Options")
	c.Flags().IntVar(&dbPort, "db-port", 0, "Database Port")

	// redis config
	c.Flags().StringVar(&redisHost, "redis-host", "", "Redis Host")
	c.Flags().StringVar(&redisType, "redis-type", "", "Redis provider")
	c.Flags().StringVar(&redisScheme, "redis-scheme", "", "Redis Scheme")
	c.Flags().StringVar(&redisUsername, "redis-username", "", "Redis Username")
	c.Flags().StringVar(&redisPassword, "redis-password", "", "Redis Password")
	c.Flags().StringVar(&redisDatabase, "redis-database", "", "Redis database")
	c.Flags().IntVar(&redisPort, "redis-port", 0, "Redis Port")

	c.PersistentPreRunE(hooks.PreRun(app, db))
	c.PersistentPostRunE(hooks.PostRun(app, db))

	// Add Sub Commands
	c.AddCommand(version.AddVersionCommand())
	c.AddCommand(server.AddServerCommand(app))
	c.AddCommand(worker.AddWorkerCommand(app))
	c.AddCommand(retry.AddRetryCommand(app))
	c.AddCommand(scheduler.AddSchedulerCommand(app))
	c.AddCommand(migrate.AddMigrateCommand(app))
	c.AddCommand(configCmd.AddConfigCommand(app))
	c.AddCommand(domain.AddDomainCommand(app))
	c.AddCommand(ingest.AddIngestCommand(app))
	c.AddCommand(stream.AddStreamCommand(app))

	if err := c.Execute(); err != nil {
		slog.Fatal(err)
	}
}
