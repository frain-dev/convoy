package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
)

var (
	ErrProjectConfigNotCreated = errors.New("project config could not be created")
	ErrProjectConfigNotUpdated = errors.New("project config could not be updated")
	ErrProjectNotCreated       = errors.New("project could not be created")
	ErrProjectNotUpdated       = errors.New("project could not be updated")
)

const (
	createProject = `
	INSERT INTO convoy.projects (id, name, type, logo_url, organisation_id, project_configuration_id)
	VALUES ($1, $2, $3, $4, $5, $6) RETURNING id;
	`

	createProjectConfiguration = `
	INSERT INTO convoy.project_configurations (
		id, retention_policy_policy, max_payload_read_size,
		replay_attacks_prevention_enabled,
		retention_policy_enabled, ratelimit_count,
		ratelimit_duration, strategy_type,
		strategy_duration, strategy_retry_count,
		signature_header, signature_versions, disable_endpoint,
		meta_events_enabled, meta_events_type, meta_events_event_type,
		meta_events_url, meta_events_secret, meta_events_pub_sub
	  )
	  VALUES
		(
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		  $14, $15, $16, $17, $18, $19
		);
	`

	updateProjectConfiguration = `
	UPDATE convoy.project_configurations SET
		retention_policy_policy = $2,
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
		disable_endpoint = $13,
		meta_events_enabled = $14,
		meta_events_type = $15,
		meta_events_event_type = $16,
		meta_events_url = $17,
		meta_events_secret = $18,
		meta_events_pub_sub = $19,
		updated_at = now()
	WHERE id = $1 AND deleted_at IS NULL;
	`
	fetchProjectById = `
	SELECT
		p.id,
		p.name,
		p.type,
		p.retained_events,
		p.logo_url,
		p.organisation_id,
		p.project_configuration_id,
		c.retention_policy_policy as "config.retention_policy.policy",
		c.max_payload_read_size as "config.max_payload_read_size",
		c.replay_attacks_prevention_enabled as "config.replay_attacks_prevention_enabled",
		c.retention_policy_enabled as "config.retention_policy_enabled",
		c.ratelimit_count as "config.ratelimit.count",
		c.ratelimit_duration as "config.ratelimit.duration",
		c.strategy_type as "config.strategy.type",
		c.strategy_duration as "config.strategy.duration",
		c.strategy_retry_count as "config.strategy.retry_count",
		c.signature_header as "config.signature.header",
		c.signature_versions as "config.signature.versions",
		c.disable_endpoint as "config.disable_endpoint",
		c.meta_events_enabled as "config.meta_event.is_enabled",
		COALESCE(c.meta_events_type, '') as "config.meta_event.type",
		c.meta_events_event_type as "config.meta_event.event_type",
		COALESCE(c.meta_events_url, '') as "config.meta_event.url",
		COALESCE(c.meta_events_secret, '') as "config.meta_event.secret",
		c.meta_events_pub_sub as "config.meta_event.pub_sub",
		p.created_at,
		p.updated_at,
		p.deleted_at
	FROM convoy.projects p
	LEFT JOIN convoy.project_configurations c
	ON p.project_configuration_id = c.id
	WHERE p.id = $1 AND p.deleted_at IS NULL;
	`

	fetchProjects = `
  SELECT
	p.id,
	p.name,
	p.type,
	p.retained_events,
	p.logo_url,
	p.organisation_id,
	p.project_configuration_id,
	c.retention_policy_policy as "config.retention_policy.policy",
	c.max_payload_read_size as "config.max_payload_read_size",
	c.replay_attacks_prevention_enabled as "config.replay_attacks_prevention_enabled",
	c.retention_policy_enabled as "config.retention_policy_enabled",
	c.ratelimit_count as "config.ratelimit.count",
	c.ratelimit_duration as "config.ratelimit.duration",
	c.strategy_type as "config.strategy.type",
	c.strategy_duration as "config.strategy.duration",
	c.strategy_retry_count as "config.strategy.retry_count",
	c.signature_header as "config.signature.header",
	c.signature_versions as "config.signature.versions",
	c.meta_events_enabled as "config.meta_event.is_enabled",
	COALESCE(c.meta_events_type, '') as "config.meta_event.type",
	c.meta_events_event_type as "config.meta_event.event_type",
	COALESCE(c.meta_events_url, '') as "config.meta_event.url",
	COALESCE(c.meta_events_secret, '') as "config.meta_event.secret",
	c.meta_events_pub_sub as "config.meta_event.pub_sub",
	p.created_at,
	p.updated_at,
	p.deleted_at,
	(SELECT count(*) from convoy.events where project_id = p.id AND deleted_at IS NULL) AS "statistics.messages_sent",
	(SELECT count(*) from convoy.endpoints where project_id = p.id AND deleted_at IS NULL) AS "statistics.total_endpoints"
  FROM convoy.projects p
  LEFT JOIN convoy.project_configurations c
  ON p.project_configuration_id = c.id
  WHERE (p.organisation_id = $1 OR $1 = '') AND p.deleted_at IS NULL ORDER BY p.id;
 `

	updateProjectById = `
	UPDATE convoy.projects SET
	name = $2,
	logo_url = $3,
	retained_events = $4,
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

	projectStatistics = `
	SELECT
	(SELECT count(*) FROM convoy.subscriptions WHERE project_id = $1 AND deleted_at IS NULL) AS total_subscriptions,
	(SELECT count(*) FROM convoy.endpoints WHERE project_id = $1 AND deleted_at IS NULL) AS total_endpoints,
	(SELECT count(*) FROM convoy.sources WHERE project_id = $1 AND deleted_at IS NULL) AS total_sources,
	(SELECT count(*) FROM convoy.events WHERE project_id = $1 AND deleted_at IS NULL) AS messages_sent;
	`

	updateProjectEndpointStatus = `
	UPDATE convoy.endpoints SET status = ?, updated_at = now()
	WHERE project_id = ? AND status IN (?) AND deleted_at IS NULL;
	`
)

type projectRepo struct {
	db *sqlx.DB
}

func NewProjectRepo(db database.Database) datastore.ProjectRepository {
	return &projectRepo{db: db.GetDB()}
}

func (p *projectRepo) CreateProject(ctx context.Context, project *datastore.Project) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	rc := project.Config.GetRetentionPolicyConfig()
	rlc := project.Config.GetRateLimitConfig()
	sc := project.Config.GetStrategyConfig()
	sgc := project.Config.GetSignatureConfig()
	me := project.Config.GetMetaEventConfig()

	configID := ulid.Make().String()
	result, err := tx.ExecContext(ctx, createProjectConfiguration,
		configID,
		rc.Policy,
		project.Config.MaxIngestSize,
		project.Config.ReplayAttacks,
		project.Config.IsRetentionPolicyEnabled,
		rlc.Count,
		rlc.Duration,
		sc.Type,
		sc.Duration,
		sc.RetryCount,
		sgc.Header,
		sgc.Versions,
		project.Config.DisableEndpoint,
		me.IsEnabled,
		me.Type,
		me.EventType,
		me.URL,
		me.Secret,
		me.PubSub,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectConfigNotCreated
	}

	project.ProjectConfigID = configID
	proResult, err := tx.ExecContext(ctx, createProject, project.UID, project.Name, project.Type, project.LogoURL, project.OrganisationID, project.ProjectConfigID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return datastore.ErrDuplicateProjectName
		}
		return err
	}

	rowsAffected, err = proResult.RowsAffected()
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

	projects := make([]*datastore.Project, 0)
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
	defer rollbackTx(tx)

	pRes, err := tx.ExecContext(ctx, updateProjectById, project.UID, project.Name, project.LogoURL, project.RetainedEvents)
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

	me := project.Config.GetMetaEventConfig()
	cRes, err := tx.ExecContext(ctx, updateProjectConfiguration,
		project.ProjectConfigID,
		project.Config.RetentionPolicy.Policy,
		project.Config.MaxIngestSize,
		project.Config.ReplayAttacks,
		project.Config.IsRetentionPolicyEnabled,
		project.Config.RateLimit.Count,
		project.Config.RateLimit.Duration,
		project.Config.Strategy.Type,
		project.Config.Strategy.Duration,
		project.Config.Strategy.RetryCount,
		project.Config.Signature.Header,
		project.Config.Signature.Versions,
		project.Config.DisableEndpoint,
		me.IsEnabled,
		me.Type,
		me.EventType,
		me.URL,
		me.Secret,
		me.PubSub,
	)
	if err != nil {
		return err
	}

	rowsAffected, err = cRes.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectConfigNotUpdated
	}

	if !project.Config.DisableEndpoint {
		status := []datastore.EndpointStatus{datastore.InactiveEndpointStatus, datastore.PendingEndpointStatus}
		query, args, err := sqlx.In(updateProjectEndpointStatus, datastore.ActiveEndpointStatus, project.UID, status)
		if err != nil {
			return err
		}

		query = p.db.Rebind(query)
		_, err = tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p *projectRepo) FetchProjectByID(ctx context.Context, id string) (*datastore.Project, error) {
	var project datastore.Project
	err := p.db.GetContext(ctx, &project, fetchProjectById, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrProjectNotFound
		}
		return nil, err
	}

	return &project, nil
}

func (p *projectRepo) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	var stats datastore.ProjectStatistics
	err := p.db.GetContext(ctx, &stats, projectStatistics, project.UID)
	if err != nil {
		return err
	}

	project.Statistics = &stats
	return nil
}

func (p *projectRepo) DeleteProject(ctx context.Context, id string) error {
	tx, err := p.db.BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

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
