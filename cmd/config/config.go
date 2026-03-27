package config

import (
	"encoding/json"
	"fmt"

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
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return fmt.Errorf("error getting config: %w", err)
			}

			data, err := json.MarshalIndent(cfg, "", "    ")
			if err != nil {
				return fmt.Errorf("error printing config: %w", err)
			}

			fmt.Println(string(data))
			return nil
		},
	}

	return cmd
}
