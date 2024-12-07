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

func AddRotateKeyCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate-key <old-key> <new-key>",
		Short: "Rotates the encryption key by re-encrypting data with a new key",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			oldKey, newKey, err := validateAndGetKeys(args)
			if err != nil {
				return err
			}

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

			log.Infof("Starting key rotation...")

			// Ensure the old key matches the current key
			currentKey, err := km.GetCurrentKey()
			if err != nil {
				return err
			}
			if oldKey != currentKey {
				return fmt.Errorf("provided old key does not match the current encryption key")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}
			defer db.Close()

			return keys.RotateEncryptionKey(db, km, oldKey, newKey)
		},
	}
	return cmd
}

func validateAndGetKeys(args []string) (string, string, error) {
	oldKey := args[0]
	newKey := args[1]

	if oldKey == "" {
		return "", "", fmt.Errorf("old-key cannot be empty")
	}
	if newKey == "" {
		return "", "", fmt.Errorf("new-key cannot be empty")
	}
	return oldKey, newKey, nil
}
