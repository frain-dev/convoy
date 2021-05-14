package main

import (
	"log"

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
			return err
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			return app.database.Close()
		},
	}

	var configFile string

	cmd.PersistentFlags().StringVar(&configFile, "config", "./hookcamp.json", "Configuration file for Hookcamp")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addOrganisationCommnad(app))

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

type app struct {
	database hookcamp.Datastore
}
