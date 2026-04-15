package utils

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	fflag2 "github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
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
		Use:   "init-encryption",
		Short: "Initializes encryption for the specified table columns with the encryption key fetched from HCP Vault",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout, err := cmd.Flags().GetInt("timeout")
			if err != nil {
				a.Logger.Error("failed to get timeout", "error", err)
				return err
			}

			cfg, err := config.Get()
			if err != nil {
				a.Logger.Error("Error fetching the config.", "error", err)
				return err
			}

			flag := fflag2.NewFFlag(cfg.EnableFeatureFlag)
			if !flag.CanAccessFeature(fflag2.CredentialEncryption) {
				return fflag2.ErrCredentialEncryptionNotEnabled
			}

			if !a.Licenser.CredentialEncryption() {
				return ErrCredentialEncryptionFeatureUnavailable
			}

			km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser, a.Cache)
			if !km.IsSet() {
				return ErrMissingHCPVaultConfig
			}

			currentKey, err := km.GetCurrentKey()
			if err != nil {
				return err
			}

			if currentKey == "" {
				return ErrEncryptionKeyCannotBeEmpty
			}

			a.Logger.Info("Initializing encryption with the current encryption key...")

			db, err := postgres.NewDB(cfg, a.Logger)
			if err != nil {
				a.Logger.Error("Error connecting to database.", "error", err)
				return err
			}
			defer db.Close()

			err = keys.InitEncryption(a.Logger, db, km, currentKey, timeout)
			if err != nil {
				a.Logger.Error("Error initializing encryption key.", "error", err)
			}
			return err
		},
	}
	cmd.Flags().Int("timeout", 120, "Optional statement timeout in seconds (default: 120)")
	return cmd
}
