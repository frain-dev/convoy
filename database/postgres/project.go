package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	createProject = `
	-- project.go:createProject
	INSERT INTO convoy.projects (name, type, logo_url, organisation_id, project_configuration_id)
	VALUES ($1, $2, $3, $4, $5) RETURNING id;
	`

	createProjectConfiguration = `
	-- project.go:createProjectConfiguration
	INSERT INTO convoy.project_configurations (
		retention_policy, max_payload_read_size, 
		replay_attacks_prevention_enabled, 
		retention_policy_enabled, ratelimit_count, 
		ratelimit_duration, strategy_type, 
		strategy_duration, strategy_retry_count, 
		signature_header, signature_hash
	  ) 
	  VALUES 
		(
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
		  $11
		) RETURNING id;
	`

	fetchProjectById = `
	-- project.go:fetchProjectById
	SELECT
		p.name,
		p.type,
		p.retained_events,
		p.created_at,
		p.updated_at,
		p.deleted_at,
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
		c.signature_hash as "config.signature_hash"
	FROM convoy.projects p
	LEFT JOIN convoy.project_configurations c
		ON p.project_configuration_id = c.id
	WHERE p.id = $1;
	`

	fetchProjectsPaginated = `
	-- project.go:fetchProjectsPaginated
	SELECT * FROM convoy.projects
	ORDER BY $3
	LIMIT $1
	OFFSET $2;
	`

	updateProjectById = `
	-- project.go:updateProjectById
	UPDATE convoy.projects SET
	name = $2,
	owner_id = $3,
	custom_domain = $4,
	assigned_domain = $5
	WHERE id = $1;
	`

	deleteProject = `
	-- project.go:deleteProject
	UPDATE convoy.projects SET 
	deleted_at = now()
	WHERE id = $1;
	`

	countProjects = `
	-- project.go:countProjects
	SELECT COUNT(id) FROM convoy.projects;
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

	var id int
	err = tx.QueryRowx(createProjectConfiguration,
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
		o.Config.SignatureHash,
	).Scan(&id)

	if err != nil {
		return err
	}

	proResult, err := tx.Exec(createProject, o.Name, o.Type, o.LogoURL, o.OrganisationID, id)
	if err != nil {
		return err
	}

	rowsAffected, err := proResult.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrOrganizationNotCreated
	}

	return tx.Commit()
}

func (p *projectRepo) LoadProjects(ctx context.Context, f *datastore.ProjectFilter) ([]*datastore.Project, error) {
	return nil, nil
}

func (p *projectRepo) UpdateProject(ctx context.Context, o *datastore.Project) error {
	return nil
}

func (p *projectRepo) FetchProjectByID(ctx context.Context, id int) (*datastore.Project, error) {
	var project datastore.Project
	err := p.db.Get(&project, fetchProjectById, id)
	fmt.Printf("QueryRowx: %+v\n", err)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

func (p *projectRepo) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	return nil
}

func (p *projectRepo) DeleteProject(ctx context.Context, uid string) error {
	return nil
}

func (p *projectRepo) FetchProjectsByIDs(ctx context.Context, ids []string) ([]datastore.Project, error) {
	return nil, nil
}

func (p *projectRepo) deleteEndpointEvents(ctx context.Context, endpoint_id string, update bson.M) error {
	return nil
}

func (p *projectRepo) deleteEndpoint(ctx context.Context, endpoint_id string, update bson.M) error {
	return nil
}

func (p *projectRepo) deleteEndpointSubscriptions(ctx context.Context, endpoint_id string, update bson.M) error {
	return nil
}
