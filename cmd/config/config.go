package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
)

func AddConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "config outputs your instances computed configuration",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting config: %v\n", err)
				os.Exit(1)
			}

			data, err := json.MarshalIndent(cfg, "", "    ")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error printing config: %v\n", err)
				os.Exit(1)
			}

			fmt.Println(string(data))
		},
	}

	return cmd
}
