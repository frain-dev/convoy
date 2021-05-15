package main

import (
	"context"
	"log"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/datastore"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

func main() {
	os.Setenv("TZ", "") // Use UTC by default :)

	app := &app{}

	var db *gorm.DB

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

			db, err = datastore.New(cfg)
			if err != nil {
				return err
			}

			app.orgRepo = datastore.NewOrganisationRepo(db)
			app.applicationRepo = datastore.NewApplicationRepo(db)
			app.endpointRepo = datastore.NewEndpointRepository(db)

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if db == nil {
				return nil
			}

			inner, err := db.DB()
			if err != nil {
				return err
			}

			return inner.Close()
		},
	}

	var configFile string

	cmd.PersistentFlags().StringVar(&configFile, "config", "./hookcamp.json", "Configuration file for Hookcamp")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addOrganisationCommnad(app))
	cmd.AddCommand(addApplicationCommnand(app))
	cmd.AddCommand(addEndpointCommand(app))

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

type app struct {
	orgRepo         hookcamp.OrganisationRepository
	applicationRepo hookcamp.ApplicationRepository
	endpointRepo    hookcamp.EndpointRepository
}

func getCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Second*1)
}
