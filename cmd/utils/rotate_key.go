package utils

import (
	"errors"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	fflag2 "github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
)

var (
	ErrOldKeyCannotBeEmpty = errors.New("old-key cannot be empty")
	ErrNewKeyCannotBeEmpty = errors.New("new-key cannot be empty")
)

func AddRotateKeyCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate-key <old-key> <new-key>",
		Short: "Rotates the encryption key by re-encrypting data with a new key",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldKey, newKey, err := validateAndGetKeys(args)
			if err != nil {
				return err
			}
			timeout, err := cmd.Flags().GetInt("timeout")
			if err != nil {
				slog.Error("failed to get timeout", "error", err)
				return err
			}

			cfg, err := config.Get()
			if err != nil {
				slog.Error("Error fetching the config.", "error", err)
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

			slog.Info("Starting key rotation...")

			// Ensure the old key matches the current key
			currentKey, err := km.GetCurrentKey()
			if err != nil {
				return err
			}
			if oldKey != currentKey {
				return ErrOldEncryptionKeyMismatch
			}

			db, err := postgres.NewDB(cfg, a.Logger)
			if err != nil {
				slog.Error("Error connecting to database.", "error", err)
				return err
			}
			defer db.Close()

			err = keys.RotateEncryptionKey(a.Logger, db, km, oldKey, newKey, timeout)
			if err != nil {
				slog.Error("Error rotating key.", "error", err)
			}
			return err
		},
	}
	cmd.Flags().Int("timeout", 120, "Optional statement timeout in seconds (default: 120)")
	return cmd
}

func validateAndGetKeys(args []string) (string, string, error) {
	oldKey := args[0]
	newKey := args[1]

	if oldKey == "" {
		return "", "", ErrOldKeyCannotBeEmpty
	}
	if newKey == "" {
		return "", "", ErrNewKeyCannotBeEmpty
	}
	return oldKey, newKey, nil
}
