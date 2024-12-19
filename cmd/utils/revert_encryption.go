package utils

import (
	"errors"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	fflag2 "github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

func AddRevertEncryptionCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revert-encryption <encryption-key>",
		Short: "Reverts the encryption initialization for the specified table columns with the provided encryption key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			encryptionKey := args[0]

			if encryptionKey == "" {
				return ErrEncryptionKeyCannotBeEmpty
			}
			timeout, err := cmd.Flags().GetInt("timeout")
			if err != nil {
				log.WithError(err).Errorln("failed to get timeout")
				return err
			}

			log.Infof("Reverting encryption with the provided key...")

			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Error("Error fetching the config.")
				return err
			}

			flag := fflag2.NewFFlag(cfg.EnableFeatureFlag)
			if !flag.CanAccessFeature(fflag2.CredentialEncryption) {
				return fflag2.ErrCredentialEncryptionNotEnabled
			}

			km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser)
			if !km.IsSet() {
				return ErrMissingHCPVaultConfig
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
					return ErrEncryptionKeyMismatch
				}
				// allow any key if downgraded
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.WithError(err).Error("Error connecting to database.")
				return err
			}
			defer db.Close()

			err = keys.RevertEncryption(a.Logger, db, encryptionKey, timeout)
			if err != nil {
				log.WithError(err).Error("Error reverting the encryption key.")
			}
			return err
		},
	}
	cmd.Flags().Int("timeout", 120, "Optional statement timeout in seconds (default: 120)")
	return cmd
}
