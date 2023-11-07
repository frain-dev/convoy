package newcloud

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
)

func (m *Migrator) RunProjectMigration() error {
	for _, org := range m.userOrgs {
		projects, err := m.loadOrgProjects(org.UID)
		if err != nil {
			return err
		}

		err = m.SaveProjects(context.Background(), projects)
		if err != nil {
			return fmt.Errorf("failed to save projects: %v", err)
		}

		m.projects = append(m.projects, projects...)

		return nil
	}

	return nil
}

const (
	saveProjects = `
	INSERT INTO convoy.projects (id, name, type, logo_url, organisation_id, project_configuration_id, created_at, updated_at, deleted_at)
	VALUES (:id, :name, :type, :logo_url, :organisation_id, :project_configuration_id, :created_at, :updated_at, :deleted_at)
	`

	saveProjectConfigurations = `
	INSERT INTO convoy.project_configurations (
		id, retention_policy_policy, max_payload_read_size,
		replay_attacks_prevention_enabled,
		retention_policy_enabled, ratelimit_count,
		ratelimit_duration, strategy_type,
		strategy_duration, strategy_retry_count,
		signature_header, signature_versions
	  )
	  VALUES
		(
		:id, :retention_policy_policy, :max_payload_read_size,
		:replay_attacks_prevention_enabled,
		:retention_policy_enabled, :ratelimit_count,
		:ratelimit_duration, :strategy_type,
		:strategy_duration, :strategy_retry_count,
		:signature_header, :signature_versions
		:created_at, :updated_at
        :disable_endpoint, :meta_events_enabled
        :meta_events_type,:meta_events_event_type
        :meta_events_url,:meta_events_secret
        :meta_events_pub_sub
		)
	`
)

func (m *Migrator) SaveProjects(ctx context.Context, projects []datastore.Project) error {
	prValues := make([]map[string]interface{}, 0, len(projects))
	cfgs := make([]map[string]interface{}, 0, len(projects))

	for _, project := range projects {
		project.ProjectConfigID = ulid.Make().String()

		prValues = append(prValues, map[string]interface{}{
			"id":                       project.UID,
			"name":                     project.Name,
			"type":                     project.Type,
			"logo_url":                 project.LogoURL,
			"organisation_id":          project.OrganisationID,
			"project_configuration_id": project.ProjectConfigID,
			"created_at":               project.CreatedAt,
			"updated_at":               project.UpdatedAt,
			"deleted_at":               project.DeletedAt,
		})

		rc := project.Config.GetRetentionPolicyConfig()
		rlc := project.Config.GetRateLimitConfig()
		sc := project.Config.GetStrategyConfig()
		sgc := project.Config.GetSignatureConfig()
		meta := project.Config.GetMetaEventConfig()

		cfgs = append(cfgs, map[string]interface{}{
			"id":                                project.ProjectConfigID,
			"retention_policy_policy":           rc.Policy,
			"max_payload_read_size":             project.Config.MaxIngestSize,
			"replay_attacks_prevention_enabled": project.Config.ReplayAttacks,
			"retention_policy_enabled":          project.Config.IsRetentionPolicyEnabled,
			"ratelimit_count":                   rlc.Count,
			"ratelimit_duration":                rlc.Duration,
			"strategy_type":                     sc.Type,
			"strategy_duration":                 sc.Duration,
			"strategy_retry_count":              sc.RetryCount,
			"signature_header":                  sgc.Header,
			"signature_versions":                sgc.Versions,
			"created_at":                        time.Now(),
			"updated_at":                        time.Now(),
			"disable_endpoint":                  project.Config.DisableEndpoint,
			"meta_events_enabled":               meta.IsEnabled,
			"meta_events_type":                  meta.Type,
			"meta_events_event_type":            meta.EventType,
			"meta_events_url":                   meta.URL,
			"meta_events_secret":                meta.Secret,
			"meta_events_pub_sub":               meta.PubSub,
		})
	}

	tx, err := m.newDB.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	_, err = tx.NamedExecContext(ctx, saveProjectConfigurations, cfgs)
	if err != nil {
		return err
	}

	_, err = tx.NamedExecContext(ctx, saveProjects, prValues)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func rollbackTx(tx *sqlx.Tx) {
	err := tx.Rollback()
	if err != nil && !errors.Is(err, sql.ErrTxDone) {
		log.WithError(err).Error("failed to rollback tx")
	}
}
