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

var (
	ErrCredentialEncryptionFeatureUnavailable = errors.New("credential encryption feature unavailable, please upgrade")
	ErrEncryptionKeyCannotBeEmpty             = errors.New("encryption key cannot be empty")
	ErrMissingHCPVaultConfig                  = errors.New("missing required HCP vault configuration")
	ErrEncryptionKeyMismatch                  = errors.New("provided encryption key does not match the current encryption key")
	ErrOldEncryptionKeyMismatch               = errors.New("provided old key does not match the current encryption key")
)

func AddInitEncryptionCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-encryption <encryption-key>",
		Short: "Initializes encryption for the specified table columns with the provided encryption key",
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

			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Error("Error fetching the config.")
				return err
			}

			flag := fflag2.NewFFlag(cfg.EnableFeatureFlag)
			if !flag.CanAccessFeature(fflag2.CredentialEncryption) {
				return fflag2.ErrCredentialEncryptionNotEnabled
			}

			if !a.Licenser.CredentialEncryption() {
				return ErrCredentialEncryptionFeatureUnavailable
			}

			km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser)
			if !km.IsSet() {
				return ErrMissingHCPVaultConfig
			}

			log.Infof("Initializing encryption with the provided key...")

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.WithError(err).Error("Error connecting to database.")
				return err
			}
			defer db.Close()

			err = keys.InitEncryption(a.Logger, db, km, encryptionKey, timeout)
			if err != nil {
				log.WithError(err).Error("Error initializing encryption key.")
			}
			return err
		},
	}
	cmd.Flags().Int("timeout", 120, "Optional statement timeout in seconds (default: 120)")
	return cmd
}
