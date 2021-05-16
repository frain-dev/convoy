package main

import (
	"fmt"

	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/datastore"
	"github.com/spf13/cobra"
)

func addMigrationCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Update the database to the latest versions",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			db, err := datastore.New(cfg)
			if err != nil {
				return err
			}

			err = db.AutoMigrate(&hookcamp.Organisation{}, &hookcamp.Application{},
				&hookcamp.Endpoint{}, &hookcamp.Message{})
			if err != nil {
				return fmt.Errorf("could not run migration... %w", err)
			}

			fmt.Println("Migrations was successful")
			return nil
		},
	}

	return cmd
}
