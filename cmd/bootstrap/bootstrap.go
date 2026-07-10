package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/users"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
)

type bootstrapOutput struct {
	FirstName       string `json:"first_name,omitempty"`
	LastName        string `json:"last_name,omitempty"`
	Email           string `json:"email,omitempty"`
	Password        string `json:"password,omitempty"`
	UserID          string `json:"user_id,omitempty"`
	OrganisationID  string `json:"organisation_id,omitempty"`
	APIKey          string `json:"api_key,omitempty"`
	APIKeyID        string `json:"api_key_id,omitempty"`
	APIKeyExpiresAt string `json:"api_key_expires_at,omitempty"`
}

// printBootstrapOutput writes the seeded credentials to stdout. Secrets
// (password, api_key) are emitted here only, never through the logger.
func printBootstrapOutput(format string, out *bootstrapOutput, logger log.Logger) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(out, "", "    ")
		if err != nil {
			logger.Error("Error printing config", "error", err)
			return err
		}

		fmt.Println(string(data))
	case "human":
		fmt.Printf("Email: %s\n", out.Email)
		fmt.Printf("Password: %s\n", out.Password)
		fmt.Printf("First Name: %s\n", out.FirstName)
		fmt.Printf("Last Name: %s\n", out.LastName)
		fmt.Printf("User ID: %s\n", out.UserID)
		fmt.Printf("Organisation ID: %s\n", out.OrganisationID)
		if out.APIKey != "" {
			fmt.Printf("API Key: %s\n", out.APIKey)
			fmt.Printf("API Key ID: %s\n", out.APIKeyID)
			fmt.Printf("API Key Expires At: %s\n", out.APIKeyExpiresAt)
		}
	default:
		return errors.New("unsupported output format")
	}

	return nil
}

func AddBootstrapCommand(a *cli.App) *cobra.Command {
	var firstName string
	var lastName string
	var format string
	var email string
	var withAPIKey bool
	var apiKeyName string
	var apiKeyExpiration int

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "bootstrap creates a new user account",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "json" && format != "human" {
				return errors.New("unsupported output format")
			}

			if util.IsStringEmpty(email) {
				return errors.New("email is required")
			}

			// Validate the API key flags up front, before any license check or
			// record creation, so a bad flag combination fails fast and cannot
			// leave a half-seeded user/organisation.
			if !withAPIKey && (cmd.Flags().Changed("api-key-name") || cmd.Flags().Changed("api-key-expiration")) {
				return errors.New("--api-key-name and --api-key-expiration require --with-api-key")
			}

			// An explicit expiration must be positive; when unset (0) the key
			// service applies its own default (24h).
			if cmd.Flags().Changed("api-key-expiration") && apiKeyExpiration <= 0 {
				return errors.New("--api-key-expiration must be greater than 0 (days)")
			}

			ok, err := a.Licenser.CheckUserLimit(context.Background())
			if err != nil {
				return err
			}

			if !ok {
				return services.ErrUserLimit
			}

			password, err := util.GenerateSecret()
			if err != nil {
				return err
			}

			p := datastore.Password{Plaintext: password}
			err = p.GenerateHash()
			if err != nil {
				return err
			}

			user := &datastore.User{
				UID:       ulid.Make().String(),
				FirstName: firstName,
				LastName:  lastName,
				Email:     email,
				Password:  string(p.Hash),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			userRepo := users.New(a.Logger, a.DB)
			err = userRepo.CreateUser(context.Background(), user)
			if err != nil {
				if errors.Is(err, datastore.ErrDuplicateEmail) {
					// user already exists
					a.Logger.Error("bootstrap failed: user already exists", "error", err)
					return nil
				}

				return err
			}

			co := services.NewCreateOrganisationService(
				organisations.New(a.Logger, a.DB),
				organisation_members.New(a.Logger, a.DB),
				&datastore.OrganisationRequest{Name: "Default Organisation"},
				user,
				a.Licenser,
				"",
				a.Logger,
			)

			org, err := co.Run(context.Background())
			if err != nil {
				return err
			}

			// The user and organisation are now persisted. The generated
			// password lives only in memory, and a rerun with the same email
			// exits on duplicate email without re-emitting it, so any later
			// failure must still print these credentials or the account is
			// stranded.
			out := &bootstrapOutput{
				Email:          user.Email,
				Password:       p.Plaintext,
				FirstName:      user.FirstName,
				LastName:       user.LastName,
				UserID:         user.UID,
				OrganisationID: org.UID,
			}

			// Optionally mint a personal API key for the bootstrapped user so
			// the instance can be seeded fully via the API with no login step.
			// The key is a durable, org-admin credential; it is printed to
			// stdout only (never logged) and is revocable via api_key_id.
			if withAPIKey {
				cpk := &services.CreatePersonalAPIKeyService{
					ProjectRepo: projects.New(a.Logger, a.DB),
					UserRepo:    userRepo,
					APIKeyRepo:  api_keys.New(a.Logger, a.DB),
					User:        user,
					NewApiKey:   &models.PersonalAPIKey{Name: apiKeyName, Expiration: apiKeyExpiration},
					Logger:      a.Logger,
				}

				apiKey, keyString, keyErr := cpk.Run(context.Background())
				if keyErr != nil {
					// Fail closed on the exit code, but still emit the user
					// credentials first so the already-created admin account is
					// recoverable via login even though the key mint failed.
					a.Logger.Error("bootstrap: api key minting failed after user/org creation", "error", keyErr)
					if printErr := printBootstrapOutput(format, out, a.Logger); printErr != nil {
						return printErr
					}
					return keyErr
				}

				out.APIKey = keyString
				out.APIKeyID = apiKey.UID
				if apiKey.ExpiresAt.Valid {
					out.APIKeyExpiresAt = apiKey.ExpiresAt.Time.UTC().Format(time.RFC3339)
				}
			}

			return printBootstrapOutput(format, out, a.Logger)
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email")
	cmd.Flags().StringVar(&firstName, "first-name", "admin", "First name")
	cmd.Flags().StringVar(&lastName, "last-name", "admin", "Last name")
	cmd.Flags().StringVar(&format, "format", "json", "Output Format")
	cmd.Flags().BoolVar(&withAPIKey, "with-api-key", false, "Also mint and print a personal API key for the user")
	cmd.Flags().StringVar(&apiKeyName, "api-key-name", "bootstrap-key", "Name for the minted API key (requires --with-api-key)")
	cmd.Flags().IntVar(&apiKeyExpiration, "api-key-expiration", 0, "API key lifetime in days; 0 uses the default (24h). Rerun on an existing user will not re-mint (requires --with-api-key)")

	return cmd
}
