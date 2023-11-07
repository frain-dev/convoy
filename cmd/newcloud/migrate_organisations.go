package newcloud

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
)

func (m *Migrator) RunOrgMigration() error {
	orgs, err := m.loadOrganisations(pagedResponse{})
	if err != nil {
		return err
	}

	// filter orgs owned by the user
	userOrgs := []datastore.Organisation{}
	for _, org := range orgs {
		if org.OwnerID == m.user.UID {
			userOrgs = append(userOrgs, org)
		}
	}

	err = m.SaveOrganisations(context.Background(), m.user, userOrgs)
	if err != nil {
		return fmt.Errorf("failed to save orgs: %v", err)
	}
	return nil
}

const (
	saveOrganizations = `
	INSERT INTO convoy.organisations (id, name, owner_id, custom_domain, assigned_domain, created_at, updated_at, deleted_at)
	VALUES (
	    :id, :name, :owner_id, :custom_domain, :assigned_domain, :created_at, :updated_at, :deleted_at
	)
	`

	saveOrgMembers = `
	INSERT INTO convoy.organisation_members (id, role_type, user_id, organisation_id, created_at, updated_at)
	VALUES (
	    :id, :role_type, :user_id, :organisation_id, :created_at, :updated_at
	)
	`
)

func (m *Migrator) SaveOrganisations(ctx context.Context, user *datastore.User, orgs []datastore.Organisation) error {
	values := make([]map[string]interface{}, 0, len(orgs))

	for _, org := range orgs {
		values = append(values, map[string]interface{}{
			"id":              org.UID,
			"name":            org.Name,
			"owner_id":        org.OwnerID,
			"custom_domain":   org.CustomDomain,
			"assigned_domain": org.AssignedDomain,
			"created_at":      org.CreatedAt,
			"updated_at":      org.UpdatedAt,
			"deleted_at":      org.DeletedAt,
		})
	}

	_, err := m.newDB.NamedExecContext(ctx, saveOrganizations, values)
	if err != nil {
		return fmt.Errorf("failed to save orgs: %v", err)
	}

	members := make([]map[string]interface{}, 0, len(orgs))
	for _, org := range orgs {
		members = append(values, map[string]interface{}{
			"id":              ulid.Make().String(),
			"role_type":       auth.RoleSuperUser.String(),
			"user_id":         user.UID,
			"organisation_id": org.UID,
			"created_at":      time.Now(),
			"updated_at":      time.Now(),
		})
	}

	_, err = m.newDB.NamedExecContext(ctx, saveOrgMembers, members)
	if err != nil {
		return fmt.Errorf("failed to save org memberss: %v", err)
	}

	return err
}
