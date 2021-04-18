package main

import (
	"log"

	"github.com/hookstack/hookstack/config"
	"github.com/spf13/cobra"
)

func main() {

	cmd := &cobra.Command{
		Use:   "hookstack",
		Short: "Opensource webhook management",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

			cfgPath, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}

			return config.LoadFromFile(cfgPath)
		},
	}

	var configFile string

	cmd.PersistentFlags().StringVar(&configFile, "config", "./hookstack.json", "Configuration file for Hookstack")

	cmd.AddCommand(addVersionCommand())
	cmd.AddCommand(addOrganisationCommnad())

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}

}
