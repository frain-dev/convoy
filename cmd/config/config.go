package config

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
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
				log.Fatalf("Error getting config: %v\n", err)
			}

			data, err := json.MarshalIndent(cfg, "", "    ")
			if err != nil {
				log.Fatalf("Error printing config: %v\n", err)
			}

			fmt.Println(string(data))
		},
	}

	return cmd
}
