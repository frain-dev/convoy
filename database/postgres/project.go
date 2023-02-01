package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/datastore"
)

var (
	ErrProjectNotCreated = errors.New("project could not be created")
	ErrProjectNotUpdated = errors.New("project could not be updated")
	ErrProjectNotDeleted = errors.New("project could not be deleted")
)

const (
	createProject = `
	INSERT INTO convoy.projects (id, name, type, logo_url, organisation_id, project_configuration_id)
	VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;
	`

	createProjectConfiguration = `
	INSERT INTO convoy.project_configurations (
		id, retention_policy, max_payload_read_size,
		replay_attacks_prevention_enabled,
		retention_policy_enabled, ratelimit_count,
		ratelimit_duration, strategy_type,
		strategy_duration, strategy_retry_count,
		signature_header, signature_versions
	  )
	  VALUES
		(
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		) RETURNING id;
	`

	updateProjectConfiguration = `
	UPDATE convoy.project_configurations SET
		retention_policy = $2,
		max_payload_read_size = $3,
		replay_attacks_prevention_enabled = $4,
		retention_policy_enabled = $5,
		ratelimit_count = $6,
		ratelimit_duration = $7,
		strategy_type = $8,
		strategy_duration = $9,
		strategy_retry_count = $10,
		signature_header = $11,
		signature_versions = $12,
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	fetchProjectById = `
	SELECT
		p.id,
		p.name,
		p.type,
		p.retained_events,
		p.organisation_id,
		c.retention_policy as "config.retention_policy",
		c.max_payload_read_size as "config.max_payload_read_size",
		c.replay_attacks_prevention_enabled as "config.replay_attacks_prevention_enabled",
		c.retention_policy_enabled as "config.retention_policy_enabled",
		c.ratelimit_count as "config.ratelimit_count",
		c.ratelimit_duration as "config.ratelimit_duration",
		c.strategy_type as "config.strategy_type",
		c.strategy_duration as "config.strategy_duration",
		c.strategy_retry_count as "config.strategy_retry_count",
		c.signature_header as "config.signature_header",
		c.signature_versions as "config.signature_versions",
		p.created_at,
		p.updated_at,
		p.deleted_at
	FROM convoy.projects p
	LEFT JOIN convoy.project_configurations c
		ON p.project_configuration_id = c.id
	WHERE p.id = $1 AND p.deleted_at IS NULL;
	`

	fetchProjects = `
	SELECT * FROM convoy.projects
	WHERE organisation_id = $1
	ORDER BY id
	WHERE deleted_at IS NULL;
	`

	updateProjectById = `
	UPDATE convoy.projects SET
	name = $2,
	logo_url = $3,
	updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteProject = `
	UPDATE convoy.projects SET
	deleted_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteProjectEndpoints = `
	UPDATE convoy.endpoints SET
	deleted_at = now()
	WHERE project_id = $1 AND deleted_at IS NULL;
	`

	deleteProjectEvents = `
	UPDATE convoy.events
	SET deleted_at = now()
	WHERE project_id = $1 AND deleted_at IS NULL;
	`
	deleteProjectEndpointSubscriptions = `
	UPDATE convoy.subscriptions SET
	deleted_at = now()
	WHERE project_id = $1 AND deleted_at IS NULL;
	`
)

type projectRepo struct {
	db *sqlx.DB
}

func NewProjectRepo(db *sqlx.DB) datastore.ProjectRepository {
	return &projectRepo{db: db}
}

func (p *projectRepo) CreateProject(ctx context.Context, o *datastore.Project) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	signatureVersions, err := json.Marshal(o.Config.SignatureVersions)
	if err != nil {
		return err
	}

	var config_id string
	err = tx.QueryRowxContext(ctx, createProjectConfiguration,
		ulid.Make().String(),
		o.Config.RetentionPolicy,
		o.Config.MaxIngestSize,
		o.Config.ReplayAttacks,
		o.Config.IsRetentionPolicyEnabled,
		o.Config.RateLimitCount,
		o.Config.RateLimitDuration,
		o.Config.StrategyType,
		o.Config.StrategyDuration,
		o.Config.StrategyRetryCount,
		o.Config.SignatureHeader,
		signatureVersions,
	).Scan(&config_id)

	if err != nil {
		return err
	}

	o.UID = ulid.Make().String()
	proResult, err := tx.ExecContext(ctx, createProject, o.UID, o.Name, o.Type, o.LogoURL, o.OrganisationID, config_id)
	if err != nil {
		return err
	}

	rowsAffected, err := proResult.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectNotCreated
	}

	return tx.Commit()
}

func (p *projectRepo) LoadProjects(ctx context.Context, f *datastore.ProjectFilter) ([]*datastore.Project, error) {
	rows, err := p.db.QueryxContext(ctx, fetchProjects, f.OrgID)
	if err != nil {
		return nil, err
	}

	var projects []*datastore.Project
	for rows.Next() {
		var proj datastore.Project

		err = rows.StructScan(&proj)
		if err != nil {
			return nil, err
		}

		projects = append(projects, &proj)
	}

	return projects, rows.Close()
}

func (p *projectRepo) UpdateProject(ctx context.Context, project *datastore.Project) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	pRes, err := tx.ExecContext(ctx, updateProjectById, project.UID, project.Name, project.LogoURL)
	if err != nil {
		return err
	}

	rowsAffected, err := pRes.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectNotUpdated
	}

	signatureVersions, err := json.Marshal(project.Config.SignatureVersions)
	if err != nil {
		return err
	}

	cRes, err := tx.ExecContext(ctx, updateProjectConfiguration,
		project.ProjectConfigID,
		project.Config.RetentionPolicy,
		project.Config.MaxIngestSize,
		project.Config.ReplayAttacks,
		project.Config.IsRetentionPolicyEnabled,
		project.Config.RateLimitCount,
		project.Config.RateLimitDuration,
		project.Config.StrategyType,
		project.Config.StrategyDuration,
		project.Config.StrategyRetryCount,
		project.Config.SignatureHeader,
		signatureVersions,
	)
	if err != nil {
		return err
	}

	rowsAffected, err = cRes.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectNotUpdated
	}

	return tx.Commit()
}

func (p *projectRepo) FetchProjectByID(ctx context.Context, id string) (*datastore.Project, error) {
	var project datastore.Project
	err := p.db.GetContext(ctx, &project, fetchProjectById, id)
	if err != nil {
		return nil, err
	}

	var signatureVersions []datastore.SignatureVersion
	err = json.Unmarshal(project.Config.Versions, &signatureVersions)
	if err != nil {
		return nil, err
	}

	project.Config.SignatureVersions = signatureVersions

	return &project, nil
}

func (p *projectRepo) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	// TODO (raymond): add implementation
	return nil
}

func (p *projectRepo) DeleteProject(ctx context.Context, id string) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteProject, id)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteProjectEndpoints, id)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteProjectEvents, id)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, deleteProjectEndpointSubscriptions, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}
