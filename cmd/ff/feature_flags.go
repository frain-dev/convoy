package ff

import (
	"github.com/frain-dev/convoy/config"
	fflag2 "github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

func AddFeatureFlagsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature-flags",
		Short: "Print the list of feature flags",
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatal("Error fetching the config.")
			}

			f := fflag2.NewFFlag(&cfg)
			return f.ListFeatures()
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {},
	}

	return cmd
}
