package utils

import (
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

var (
	ErrCredentialEncryptionFeatureUnavailable = errors.New("credential encryption feature unavailable, please upgrade")
)

func AddInitEncryptionCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-encryption <encryption-key>",
		Short: "Initializes encryption for the specified table columns with the provided encryption key",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			encryptionKey := args[0]

			if encryptionKey == "" {
				return fmt.Errorf("encryption key cannot be empty")
			}

			log.Infof("Initializing encryption with the provided key...")

			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatal("Error fetching the config.")
			}

			if !a.Licenser.CredentialEncryption() {
				return ErrCredentialEncryptionFeatureUnavailable
			}

			km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser)
			if !km.IsSet() {
				return errors.New("missing required HCP vault configuration")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}
			defer db.Close()

			return keys.InitEncryption(db, km, encryptionKey)
		},
	}

	return cmd
}
