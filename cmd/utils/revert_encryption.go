package utils

import (
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
		Use:   "revert-encryption",
		Short: "Reverts the encryption initialization for the specified table columns with the encryption key fetched from HCP Vaults",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {

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

			km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser, a.Cache)
			if !km.IsSet() {
				return ErrMissingHCPVaultConfig
			}

			currentKey, err := km.GetHCPSecretKey()
			if err != nil {
				return err
			}

			if currentKey == "" {
				return ErrEncryptionKeyCannotBeEmpty
			}

			log.Infof("Reverting encryption with the current encryption key...")

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.WithError(err).Error("Error connecting to database.")
				return err
			}
			defer db.Close()

			err = keys.RevertEncryption(a.Logger, db, currentKey, timeout)
			if err != nil {
				log.WithError(err).Error("Error reverting the encryption key.")
			}
			return err
		},
	}
	cmd.Flags().Int("timeout", 120, "Optional statement timeout in seconds (default: 120)")
	return cmd
}
