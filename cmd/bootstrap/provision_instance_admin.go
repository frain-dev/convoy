package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/spf13/cobra"
)

var (
	ErrRootRequired = errors.New("a root user is required")
)

func AddProvisionIACommand(a *cli.App) *cobra.Command {
	var firstName string
	var lastName string
	var format string
	var email string
	var token string

	cmd := &cobra.Command{
		Use:   "provision-instance-admin",
		Short: "creates a new instance admin user account",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		RunE: func(_ *cobra.Command, args []string) error {

			if token == "" {
				return fmt.Errorf("token required")
			}
			authUser, member, err := getInstanceAdminOrRoot(a, token)
			if err != nil {
				return err
			}

			if member.Role.Type != auth.RoleRoot {
				return fmt.Errorf("invalid role %+v", authUser.Role.Type)
			}

			if authUser == nil || member == nil {
				return ErrRootRequired
			}

			return runBootstrap(a, format, email, firstName, lastName, auth.RoleInstanceAdmin)
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email")
	cmd.Flags().StringVar(&firstName, "first-name", "instance-admin", "Email")
	cmd.Flags().StringVar(&lastName, "last-name", "admin", "Email")
	cmd.Flags().StringVar(&format, "format", "json", "Output Format")
	cmd.Flags().StringVar(&token, "token", "", "Root Personal Access Token")

	return cmd
}

func getInstanceAdminOrRoot(a *cli.App, token string) (*auth.AuthenticatedUser, *datastore.OrganisationMember, error) {
	err := initialize(a)
	if err != nil {
		return nil, nil, err
	}

	rc, err := realm_chain.Get()
	if err != nil {
		return nil, nil, err
	}
	authUser, err := rc.Authenticate(context.Background(), &auth.Credential{
		Type:   auth.CredentialTypeAPIKey,
		APIKey: token,
		Token:  token,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("authorization failed %w", err)
	}

	user, ok := authUser.Metadata.(*datastore.User)
	if !ok {
		return nil, nil, fmt.Errorf("authorization failed %w", err)
	}

	orgMemberRepo := postgres.NewOrgMemberRepo(a.DB)
	m, err := orgMemberRepo.FetchAnyInstanceAdminOrRootByUserID(context.Background(), user.UID)
	if err != nil {
		if errors.Is(err, datastore.ErrOrgMemberNotFound) {
			return nil, nil, fmt.Errorf("root user not found %w", err)
		}
		return nil, nil, err
	}
	return authUser, m, nil
}

func initialize(a *cli.App) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	km := keys.NewHCPVaultKeyManagerFromConfig(cfg.HCPVault, a.Licenser, a.Cache)
	if km.IsSet() {
		if _, err := km.GetCurrentKeyFromCache(); err != nil {
			if !errors.Is(err, keys.ErrCredentialEncryptionFeatureUnavailable) {
				return err
			}
			km.Unset()
		}
	}
	if err := keys.Set(km); err != nil {
		return err
	}

	apiKeyRepo := postgres.NewAPIKeyRepo(a.DB)
	userRepo := postgres.NewUserRepo(a.DB)
	portalLinkRepo := postgres.NewPortalLinkRepo(a.DB)

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, a.Cache)
	if err != nil {
		a.Logger.WithError(err).Fatal("failed to initialize realm chain")
	}

	return nil
}
