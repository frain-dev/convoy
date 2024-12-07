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

func AddRevertEncryptionCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revert-encryption <encryption-key>",
		Short: "Reverts the encryption initialization for the specified table columns with the provided encryption key",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			encryptionKey := args[0]

			if encryptionKey == "" {
				return fmt.Errorf("encryption key cannot be empty")
			}

			log.Infof("Reverting encryption with the provided key...")

			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatal("Error fetching the config.")
			}

			km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser)
			if !km.IsSet() {
				return errors.New("missing required HCP vault configuration")
			}

			// Ensure the encryption key matches the current key
			currentKey, err := km.GetCurrentKey()
			if err != nil {
				if !errors.Is(err, keys.ErrCredentialEncryptionFeatureUnavailable) {
					return err
				}
			}
			if encryptionKey != currentKey {
				if !errors.Is(err, keys.ErrCredentialEncryptionFeatureUnavailable) {
					return fmt.Errorf("provided encryption key does not match the current encryption key")
				}
				// allow any key if downgraded
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}
			defer db.Close()

			return keys.RevertEncryption(db, km, encryptionKey)
		},
	}

	return cmd
}
