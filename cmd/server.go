package main

import (
	"errors"

	"github.com/hookcamp/hookcamp/config"
	"github.com/hookcamp/hookcamp/server"
	"github.com/spf13/cobra"
)

func addServerCommand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "server",
		Aliases: []string{"serve", "s"},
		Short:   "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {

			cfg, err := config.Get()
			if err != nil {
				return err
			}

			if cfg.Server.HTTP.Port <= 0 {
				return errors.New("please provide the HTTP port in the hookcamp.json file")
			}

			srv := server.New(cfg, a.messageRepo, a.applicationRepo, a.orgRepo)
			return srv.ListenAndServe()
		},
	}

	return cmd
}
