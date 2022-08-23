package main

import (
	"github.com/frain-dev/convoy/cmd/migrations"
	"github.com/frain-dev/convoy/config"
	"github.com/golang-migrate/migrate/v4"
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addMigrateCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convoy migrations",
	}

	cmd.AddCommand(addUpSubCommand())
	cmd.AddCommand(addDownSubCommand())

	return cmd
}

func addUpSubCommand() *cobra.Command {
	var n int

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Apply migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := createMigrate()
			if err != nil {
				log.WithError(err).Fatal("Failed to create migrate instance")
				return err
			}

			if n > 0 {
				return m.Steps(n)
			}

			return m.Up()
		},
	}

	cmd.Flags().IntVar(&n, "n", 0, "number of steps")
	return cmd
}

func addDownSubCommand() *cobra.Command {
	var n int

	cmd := &cobra.Command{
		Use:   "down",
		Short: "Rollback migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := createMigrate()
			if err != nil {
				log.WithError(err).Fatal("Failed to create migrate instance")
				return err
			}

			if n > 0 {
				return m.Steps(-n)
			}

			return m.Down()
		},
	}

	cmd.Flags().IntVar(&n, "n", 0, "number of steps")
	return cmd
}

func createMigrate() (*migrate.Migrate, error) {
	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Error("Failed to load config instance")
		return nil, err
	}

	s := bindata.Resource(migrations.AssetNames(),
		func(name string) ([]byte, error) {
			return migrations.Asset(name)
		})

	d, err := bindata.WithInstance(s)
	if err != nil {
		log.WithError(err).Error("Failed to load migrations data")
		return nil, err
	}

	m, err := migrate.NewWithSourceInstance("go-bindata", d, cfg.Database.Dsn)
	if err != nil {
		log.WithError(err).Error("Failed to create migration instance")
		return nil, err
	}

	return m, nil
}
