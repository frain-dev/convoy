package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/datastore"
)

var (
	ErrProjectNotCreated              = errors.New("project could not be created")
	ErrProjectNotUpdated              = errors.New("project could not be updated")
	ErrProjectNotDeleted              = errors.New("project could not be deleted")
	ErrProjectEventsNotDeleted        = errors.New("project events could not be deleted")
	ErrProjectEndpointsNotDeleted     = errors.New("project endpoints could not be deleted")
	ErrProjectSubscriptionsNotDeleted = errors.New("project subscriptions could not be deleted")
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

	fetchProjects = `
	-- project.go:fetchProjects
	SELECT * FROM convoy.projects
	WHERE organisation_id = $1
	ORDER BY id;
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

	getProjectEndpoints = `
	-- project.go:updateProjectById
	SELECT id FROM convoy.endpoints WHERE project_id = $1
	`

	deleteProject = `
	-- project.go:deleteProject
	UPDATE convoy.projects SET 
	deleted_at = now()
	WHERE id = $1;
	`

	deleteProjectEndpoints = `
	-- project.go:deleteProjectEndpoints
	UPDATE convoy.endpoints SET 
	deleted_at = now()
	WHERE project_id = $1;
	`

	deleteProjectEvents = `
	-- project.go:deleteProjectEvents
	UPDATE convoy.events 
	SET deleted_at = now() 
	WHERE project_id = $1;
	`
	deleteProjectEndpointSubscriptions = `
	-- project.go:deleteProjectEndpointSubscriptions
	UPDATE convoy.subscriptions SET 
	deleted_at = now()
	WHERE project_id = $1;
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
		return ErrProjectNotCreated
	}

	return tx.Commit()
}

func (p *projectRepo) LoadProjects(ctx context.Context, f *datastore.ProjectFilter) ([]*datastore.Project, error) {
	rows, err := p.db.Queryx(fetchProjects, f.OrgID)
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

	return projects, nil
}

func (p *projectRepo) UpdateProject(ctx context.Context, o *datastore.Project) error {
	result, err := p.db.Exec(updateProjectById, o)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectNotUpdated
	}

	return nil
}

func (p *projectRepo) FetchProjectByID(ctx context.Context, id int) (*datastore.Project, error) {
	var project datastore.Project
	err := p.db.Get(&project, fetchProjectById, id)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

func (p *projectRepo) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	return nil
}

func (p *projectRepo) DeleteProject(ctx context.Context, id string) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	_, err = tx.Exec(deleteProject, id)
	if err != nil {
		return err
	}

	// var ids []int
	// err = tx.Select(&ids, getProjectEndpoints, uid)
	// if err != nil {
	// 	return err
	// }

	// fmt.Println(ids)

	_, err = tx.Exec(deleteProjectEndpoints, id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(deleteProjectEvents, id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(deleteProjectEndpointSubscriptions, id)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (p *projectRepo) FetchProjectsByIDs(ctx context.Context, ids []string) ([]datastore.Project, error) {
	return nil, nil
}
