package utils

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisation_members"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/users"
)

func AddPromoteInstanceAdminCommand(a *cli.App) *cobra.Command {
	var email, orgID string

	cmd := &cobra.Command{
		Use:   "promote-instance-admin",
		Short: "Promote a user to instance admin for an organisation membership",
		Long: `Sets organisation_members.role_type to instance_admin for the given user.

Requires a matching organisation membership. If --org-id is omitted and the user belongs to exactly one organisation, that org is used. If they belong to more than one, you must set --org-id (the command errors with a list of organisation ids).

Run against the same database as the Convoy server (same config / env as other convoy utils commands).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			email = strings.TrimSpace(email)
			if email == "" {
				return fmt.Errorf("--email is required")
			}

			ctx := context.Background()
			userSvc := users.New(a.Logger, a.DB)
			orgMemberRepo := organisation_members.New(a.Logger, a.DB)

			u, err := userSvc.FindUserByEmail(ctx, email)
			if err != nil {
				if errors.Is(err, datastore.ErrUserNotFound) {
					return fmt.Errorf("no user found with email %q", email)
				}
				return fmt.Errorf("lookup user: %w", err)
			}

			resolvedOrgID, err := resolveOrgIDForPromotion(ctx, orgMemberRepo, u.UID, orgID)
			if err != nil {
				return err
			}

			member, err := orgMemberRepo.FetchOrganisationMemberByUserID(ctx, u.UID, resolvedOrgID)
			if err != nil {
				return fmt.Errorf("fetch organisation member: %w", err)
			}

			if member.Role.Type == auth.RoleInstanceAdmin {
				slog.Info("user is already instance admin for this organisation",
					"email", email,
					"user_id", u.UID,
					"org_id", resolvedOrgID,
				)
				return nil
			}

			prevRole := member.Role.Type
			member.Role = auth.Role{Type: auth.RoleInstanceAdmin}
			if err := orgMemberRepo.UpdateOrganisationMember(ctx, member); err != nil {
				return fmt.Errorf("update organisation member: %w", err)
			}

			slog.Info("promoted user to instance admin",
				"email", email,
				"user_id", u.UID,
				"org_id", resolvedOrgID,
				"previous_role", prevRole,
			)
			fmt.Printf("OK: %q is now instance_admin for organisation %s\n", email, resolvedOrgID)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "User email address")
	cmd.Flags().StringVar(&orgID, "org-id", "", "Organisation ID (required when the user has more than one membership)")

	return cmd
}

func resolveOrgIDForPromotion(
	ctx context.Context,
	orgMemberRepo datastore.OrganisationMemberRepository,
	userID string,
	explicitOrgID string,
) (string, error) {
	if strings.TrimSpace(explicitOrgID) != "" {
		return strings.TrimSpace(explicitOrgID), nil
	}

	orgs, err := loadAllUserOrganisations(ctx, orgMemberRepo, userID)
	if err != nil {
		return "", err
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("user has no organisation memberships")
	}
	if len(orgs) > 1 {
		var b strings.Builder
		b.WriteString("user belongs to multiple organisations: set --org-id to one of the following:\n")
		for _, o := range orgs {
			b.WriteString(fmt.Sprintf("  - %s (%s)\n", o.UID, o.Name))
		}
		return "", errors.New(strings.TrimSuffix(b.String(), "\n"))
	}
	return orgs[0].UID, nil
}

func loadAllUserOrganisations(
	ctx context.Context,
	orgMemberRepo datastore.OrganisationMemberRepository,
	userID string,
) ([]datastore.Organisation, error) {
	var all []datastore.Organisation
	cursor := ""
	for page := 0; page < 50; page++ {
		p := datastore.Pageable{
			PerPage:    100,
			Direction:  datastore.Next,
			Sort:       "",
			NextCursor: cursor,
		}
		p.SetCursors()

		orgs, pag, err := orgMemberRepo.LoadUserOrganisationsPaged(ctx, userID, p)
		if err != nil {
			return nil, fmt.Errorf("list user organisations: %w", err)
		}
		all = append(all, orgs...)
		if !pag.HasNextPage {
			return all, nil
		}
		cursor = pag.NextPageCursor
	}
	return nil, fmt.Errorf("user has too many organisation memberships; use --org-id")
}
