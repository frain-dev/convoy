package main

import (
	"context"
	"fmt"
	"os"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addUpgradeCommand(a *app) *cobra.Command {
	var oldVersion string
	var newVersion string

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Convoy Upgrader",
		Run: func(cmd *cobra.Command, args []string) {
			switch oldVersion {
			case "v0.4":
				if newVersion == "v0.5" {
					updateVersion4ToVersion5()
				}
				log.Error(fmt.Sprintf("%s is not a valid new version for v0.4", newVersion))
			default:
				log.Error(fmt.Sprintf("%s is not a valid old version", oldVersion))
			}

			os.Exit(0)
		},
	}

	cmd.Flags().StringVar(&oldVersion, "from-version", "", "old version")
	cmd.Flags().StringVar(&newVersion, "to-version", "", "new version")
	return cmd
}

func updateVersion4ToVersion5() {
	ctx := context.Background()

	cfg, err := config.Get()
	if err != nil {
		log.WithError(err).Fatalf("Error fetching the config.")
	}

	db, err := NewDB(cfg)
	if err != nil {
		log.WithError(err).Fatalf("Error connecting to the db.")
	}

	groups, err := db.GroupRepo().LoadGroups(ctx, &datastore.GroupFilter{})
	if err != nil {
		log.WithError(err).Fatalf("Error fetching the groups.")
	}

	for _, grp := range groups {
		group := *grp
		group.RateLimit = 5000
		group.RateLimitDuration = "1m"

		err = db.GroupRepo().UpdateGroup(ctx, &group)
		if err != nil {
			log.WithError(err).Fatalf("Error updating group details.")
		}
	}

	log.Info("Upgrade complete")
	os.Exit(0)
}
