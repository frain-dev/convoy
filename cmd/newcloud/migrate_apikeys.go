package newcloud

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/auth"

	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/database/postgres"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

func (m *Migrator) RunAPIKeyMigration() error {
	apiKeyRepo := postgres.NewAPIKeyRepo(m, ncache.NewNoopCache())
	pageable := &datastore.Pageable{
		PerPage:    1000,
		Direction:  "next",
		NextCursor: "",
	}

	// migrate project api keys
	for _, p := range m.projects {
		keys, err := m.loadAPIKeys(apiKeyRepo, p.UID, "", pageable)
		if err != nil {
			return err
		}

		err = m.SaveAPIKeys(context.Background(), keys)
		if err != nil {
			return fmt.Errorf("failed to save project keys: %v", err)
		}
		return nil
	}

	// migrate user api keys
	userKeys, err := m.loadAPIKeys(apiKeyRepo, "", m.user.UID, pageable)
	if err != nil {
		return err
	}

	err = m.SaveAPIKeys(context.Background(), userKeys)
	if err != nil {
		return fmt.Errorf("failed to save user keys: %v", err)
	}

	return nil
}

const (
	saveAPIKeys = `
    INSERT INTO convoy.api_keys (id,name,key_type,mask_id,role_type,role_project,role_endpoint,hash,salt,user_id,expires_at,created_at,updated_at, deleted_at)
    VALUES (
        :id, :name, :key_type, :mask_id, :role_type, :role_project,
        :role_endpoint, :hash, :salt, :user_id, :expires_at,
        :created_at, :updated_at, :deleted_at
    )
    `
)

func (a *Migrator) SaveAPIKeys(ctx context.Context, keys []datastore.APIKey) error {
	values := make([]map[string]interface{}, 0, len(keys))

	for _, key := range keys {
		var (
			userID     *string
			endpointID *string
			projectID  *string
			roleType   *auth.RoleType
		)

		if !util.IsStringEmpty(key.UserID) {
			userID = &key.UserID
		}

		if !util.IsStringEmpty(key.Role.Endpoint) {
			endpointID = &key.Role.Endpoint
		}

		if !util.IsStringEmpty(key.Role.Project) {
			projectID = &key.Role.Project
		}

		if !util.IsStringEmpty(string(key.Role.Type)) {
			roleType = &key.Role.Type
		}

		values = append(values, map[string]interface{}{
			"id":            key.UID,
			"name":          key.Name,
			"key_type":      key.Type,
			"mask_id":       key.MaskID,
			"role_type":     roleType,
			"role_project":  projectID,
			"role_endpoint": endpointID,
			"hash":          key.Hash,
			"salt":          key.Salt,
			"user_id":       userID,
			"expires_at":    key.ExpiresAt,
			"created_at":    key.CreatedAt,
			"updated_at":    key.UpdatedAt,
			"deleted_at":    key.DeletedAt,
		})
	}

	_, err := a.newDB.NamedExecContext(ctx, saveAPIKeys, values)
	return err
}
