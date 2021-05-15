package main

import (
	"context"
	"log"
	"time"

	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/datastore"
	"github.com/spf13/cobra"
)

func main() {

	app := &app{}

	cmd := &cobra.Command{
		Use:   "hookcamp",
		Short: "Opensource Webhooks as a service",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}

			err = config.LoadFromFile(cfgPath)
			if err != nil {
				return err
			}

			cfg, err := config.Get()
			if err != nil {
				return err
			}

			app.database, err = datastore.New(cfg)

			if err != nil {
				return err
			}

			return app.database.Migrate()
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return app.database.Close()
		},
	}

	var configFile string

	cmd.PersistentFlags().StringVar(&configFile, "config", "./hookcamp.json", "Configuration file for Hookcamp")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addOrganisationCommnad(app))
	cmd.AddCommand(addApplicationCommnand(app))

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

type app struct {
	database hookcamp.Datastore
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}
