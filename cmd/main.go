package main

import (
	"errors"
	"log"

	"github.com/hookstack/hookstack"
	"github.com/hookstack/hookstack/config"
	"github.com/spf13/cobra"
)

func main() {

	app := &hookstack.App{}

	cmd := &cobra.Command{
		Use:   "hookstack",
		Short: "Opensource webhook management",
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

			if cfg.Organisation.FetchMode != config.FileSystemOrganisationFetchMode {
				return errors.New("unsupported fetch mode")
			}

			app.OrgLoader = hookstack.NewFileOrganisationLoader(cfg.Organisation.FilePath)
			return nil
		},
	}

	var configFile string

	cmd.PersistentFlags().StringVar(&configFile, "config", "./hookstack.json", "Configuration file for Hookstack")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addOrganisationCommnad(app))

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}

}
