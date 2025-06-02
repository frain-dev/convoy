package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/auth"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/services"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"

	"github.com/spf13/cobra"
)

var (
	ErrInstanceAdminRequired = errors.New("an instance admin is required")
)

func AddBootstrapCommand(a *cli.App) *cobra.Command {
	var firstName string
	var lastName string
	var format string
	var email string
	var token string

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "bootstrap creates a new user account",
		Annotations: map[string]string{
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {

			orgMemberRepo := postgres.NewOrgMemberRepo(a.DB)

			count, err := orgMemberRepo.CountOrganisationAdminUsers(context.Background())
			if err != nil {
				return fmt.Errorf("failed to count org admins: %w", err)
			}

			if count > 0 {
				// org admin exists
				if token == "" {
					return fmt.Errorf("an access token required to proceed")
				}
				authUser, member, err := getInstanceAdmin(a, token)
				if err != nil {
					log.WithError(err).Warn("failed to get instance admin")
					return fmt.Errorf("failed to get instance admin: %w", err)
				}
				if authUser == nil || member == nil {
					return ErrInstanceAdminRequired
				}

				if member.Role.Type != auth.RoleInstanceAdmin {
					return fmt.Errorf("invalid role %+v", authUser.Role.Type)
				}
			}

			return runBootstrap(a, format, email, firstName, lastName, auth.RoleOrganisationAdmin)
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email")
	cmd.Flags().StringVar(&firstName, "first-name", "admin", "Email")
	cmd.Flags().StringVar(&lastName, "last-name", "admin", "Email")
	cmd.Flags().StringVar(&format, "format", "json", "Output Format")
	cmd.Flags().StringVar(&token, "token", "", "Instance Admin Personal Access Token")

	return cmd
}

func runBootstrap(a *cli.App, format string, email string, firstName string, lastName string, roleType auth.RoleType) error {
	ok, err := a.Licenser.CreateUser(context.Background())
	if err != nil {
		return err
	}

	if !ok {
		return services.ErrUserLimit
	}

	if format != "json" && format != "human" {
		return errors.New("unsupported output format")
	}

	if util.IsStringEmpty(email) {
		return errors.New("email is required")
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

	if roleType == auth.RoleInstanceAdmin {
		user.EmailVerified = true
	}

	userRepo := postgres.NewUserRepo(a.DB)
	err = userRepo.CreateUser(context.Background(), user)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateEmail) {
			// user already exists
			log.WithError(err).Error("bootstrap failed: user already exists")
			return nil
		}

		return err
	}
	orgRepo := postgres.NewOrgRepo(a.DB)
	orgMemberRepo := postgres.NewOrgMemberRepo(a.DB)
	co := services.CreateOrganisationService{
		OrgRepo:       orgRepo,
		OrgMemberRepo: orgMemberRepo,
		NewOrg:        &models.Organisation{Name: "Default Organisation"},
		User:          user,
		Licenser:      a.Licenser,
		RoleType:      roleType,
	}

	_, err = co.Run(context.Background())
	if err != nil {
		return err
	}

	type JsonUser struct {
		FirstName string `json:"first_name,omitempty"`
		LastName  string `json:"last_name,omitempty"`
		Email     string `json:"email,omitempty"`
		Password  string `json:"password,omitempty"`
	}

	jsUser := &JsonUser{
		Email:     user.Email,
		Password:  p.Plaintext,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(jsUser, "", "    ")
		if err != nil {
			log.Fatalf("Error printing config: %v\n", err)
		}

		fmt.Println(string(data))
	case "human":
		fmt.Printf("Email: %s\n", jsUser.Email)
		fmt.Printf("Password: %s\n", jsUser.Password)
		fmt.Printf("First Name: %s\n", jsUser.FirstName)
		fmt.Printf("Last Name: %s\n", jsUser.LastName)
	default:
		return errors.New("unsupported output format")
	}

	if roleType == auth.RoleInstanceAdmin {
		org := &datastore.Organisation{
			UID:       ulid.Make().String(),
			OwnerID:   user.UID,
			Name:      "Instance Admin Org",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err = orgRepo.CreateOrganisation(context.Background(), org)
		if err != nil {
			return err
		}

		member := &datastore.OrganisationMember{
			UID:            ulid.Make().String(),
			OrganisationID: org.UID,
			UserID:         user.UID,
			Role:           auth.Role{Type: auth.RoleInstanceAdmin},
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		err = orgMemberRepo.CreateOrganisationMember(context.Background(), member)
		if err != nil {
			return err
		}

		a.Logger.Infof("Created instance admin user with username: %s and password: %s", user.Email, p.Plaintext)
	}

	return nil
}
