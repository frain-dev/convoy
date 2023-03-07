package main

import (
	"os"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/ee"
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
	if err := cli.Execute(); err != nil {
		slog.Fatal(err)
	}
}
