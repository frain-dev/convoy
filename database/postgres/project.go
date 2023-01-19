package postgres

import (
	"context"
	"database/sql"

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

	fetchProject = `
	-- project.go:fetchProject
	SELECT * FROM convoy.projects 
	WHERE $1 = $2;
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

func (p *projectRepo) FetchProjectByID(ctx context.Context, id string) (*datastore.Project, error) {
	return nil, nil
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
