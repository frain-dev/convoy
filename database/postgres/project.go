package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/frain-dev/convoy/database/hooks"
	"github.com/r3labs/diff/v3"

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
		id, search_policy, max_payload_read_size, 
		replay_attacks_prevention_enabled, ratelimit_count,
		ratelimit_duration, strategy_type,	strategy_duration,
		strategy_retry_count, signature_header, signature_versions,
		disable_endpoint, meta_events_enabled, meta_events_type,
		meta_events_event_type, meta_events_url, meta_events_secret,
		meta_events_pub_sub, ssl_enforce_secure_endpoints
	  )
	  VALUES
		(
		  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
		  $14, $15, $16, $17, $18, $19
		);
	`

	updateProjectConfiguration = `
	UPDATE convoy.project_configurations SET
		max_payload_read_size = $2,
		replay_attacks_prevention_enabled = $3,
		ratelimit_count = $4,
		ratelimit_duration = $5,
		strategy_type = $6,
		strategy_duration = $7,
		strategy_retry_count = $8,
		signature_header = $9,
		signature_versions = $10,
		disable_endpoint = $11,
		meta_events_enabled = $12,
		meta_events_type = $13,
		meta_events_event_type = $14,
		meta_events_url = $15,
		meta_events_secret = $16,
		meta_events_pub_sub = $17,
		search_policy = $18,
		ssl_enforce_secure_endpoints = $19,
		updated_at = NOW()
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
		c.search_policy AS "config.search_policy",
		c.max_payload_read_size AS "config.max_payload_read_size",
		c.multiple_endpoint_subscriptions AS "config.multiple_endpoint_subscriptions",
		c.replay_attacks_prevention_enabled AS "config.replay_attacks_prevention_enabled",
		c.ratelimit_count AS "config.ratelimit.count",
		c.ratelimit_duration AS "config.ratelimit.duration",
		c.strategy_type AS "config.strategy.type",
		c.strategy_duration AS "config.strategy.duration",
		c.strategy_retry_count AS "config.strategy.retry_count",
		c.signature_header AS "config.signature.header",
		c.signature_versions AS "config.signature.versions",
		c.disable_endpoint AS "config.disable_endpoint",
		c.ssl_enforce_secure_endpoints as "config.ssl.enforce_secure_endpoints",
		c.meta_events_enabled AS "config.meta_event.is_enabled",
		COALESCE(c.meta_events_type, '') AS "config.meta_event.type",
		c.meta_events_event_type AS "config.meta_event.event_type",
		COALESCE(c.meta_events_url, '') AS "config.meta_event.url",
		COALESCE(c.meta_events_secret, '') AS "config.meta_event.secret",
		c.meta_events_pub_sub AS "config.meta_event.pub_sub",
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
    c.search_policy AS "config.search_policy",
	c.max_payload_read_size AS "config.max_payload_read_size",
	c.multiple_endpoint_subscriptions AS "config.multiple_endpoint_subscriptions",
	c.replay_attacks_prevention_enabled AS "config.replay_attacks_prevention_enabled",
	c.ratelimit_count AS "config.ratelimit.count",
	c.ratelimit_duration AS "config.ratelimit.duration",
	c.strategy_type AS "config.strategy.type",
	c.strategy_duration AS "config.strategy.duration",
	c.ssl_enforce_secure_endpoints as "config.ssl.enforce_secure_endpoints",
	c.strategy_retry_count AS "config.strategy.retry_count",
	c.signature_header AS "config.signature.header",
	c.signature_versions AS "config.signature.versions",
	c.meta_events_enabled AS "config.meta_event.is_enabled",
	COALESCE(c.meta_events_type, '') AS "config.meta_event.type",
	c.meta_events_event_type AS "config.meta_event.event_type",
	COALESCE(c.meta_events_url, '') AS "config.meta_event.url",
	COALESCE(c.meta_events_secret, '') AS "config.meta_event.secret",
	c.meta_events_pub_sub AS "config.meta_event.pub_sub",
	p.created_at,
	p.updated_at,
	p.deleted_at
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
	updated_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteProject = `
	UPDATE convoy.projects SET
	deleted_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL;
	`

	deleteProjectEndpoints = `
	UPDATE convoy.endpoints SET
	deleted_at = NOW()
	WHERE project_id = $1 AND deleted_at IS NULL;
	`

	deleteProjectEvents = `
	UPDATE convoy.events
	SET deleted_at = NOW()
	WHERE project_id = $1 AND deleted_at IS NULL;
	`
	deleteProjectEndpointSubscriptions = `
	UPDATE convoy.subscriptions SET
	deleted_at = NOW()
	WHERE project_id = $1 AND deleted_at IS NULL;
	`

	projectStatistics = `
	SELECT
	(SELECT COUNT(*) FROM convoy.subscriptions WHERE project_id = $1 AND deleted_at IS NULL) AS total_subscriptions,
	(SELECT COUNT(*) FROM convoy.endpoints WHERE project_id = $1 AND deleted_at IS NULL) AS total_endpoints,
	(SELECT COUNT(*) FROM convoy.sources WHERE project_id = $1 AND deleted_at IS NULL) AS total_sources,
	(SELECT COUNT(*) FROM convoy.events WHERE project_id = $1 AND deleted_at IS NULL) AS messages_sent;
	`

	updateProjectEndpointStatus = `
	UPDATE convoy.endpoints SET status = ?, updated_at = NOW()
	WHERE project_id = ? AND status IN (?) AND deleted_at IS NULL RETURNING
	id, name, status, owner_id, url,
    description, http_timeout, rate_limit, rate_limit_duration,
    advanced_signatures, slack_webhook_url, support_email,
    app_id, project_id, secrets, created_at, updated_at,
    authentication_type AS "authentication.type",
    authentication_type_api_key_header_name AS "authentication.api_key.header_name",
    authentication_type_api_key_header_value AS "authentication.api_key.header_value";
	`

	getProjectsWithEventsInTheInterval = `
    SELECT p.id AS id, COUNT(e.id) AS events_count
    FROM convoy.projects p
    LEFT JOIN convoy.events e ON p.id = e.project_id
    WHERE e.created_at >= NOW() - MAKE_INTERVAL(hours := $1)
    AND p.deleted_at IS NULL
    GROUP BY p.id
    ORDER BY events_count DESC;
    `

	countProjects = `
	SELECT COUNT(*) AS count
	FROM convoy.projects
	WHERE deleted_at IS NULL`
)

type projectRepo struct {
	db   database.Database
	hook *hooks.Hook
}

func NewProjectRepo(db database.Database) datastore.ProjectRepository {
	return &projectRepo{db: db, hook: db.GetHook()}
}

func (p *projectRepo) CountProjects(ctx context.Context) (int64, error) {
	var projCount int64
	err := p.db.GetReadDB().GetContext(ctx, &projCount, countProjects)
	if err != nil {
		return 0, err
	}

	return projCount, nil
}

func (p *projectRepo) CreateProject(ctx context.Context, project *datastore.Project) error {
	tx, err := p.db.GetDB().BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	rlc := project.Config.GetRateLimitConfig()
	sc := project.Config.GetStrategyConfig()
	sgc := project.Config.GetSignatureConfig()
	me := project.Config.GetMetaEventConfig()

	configID := ulid.Make().String()
	result, err := tx.ExecContext(ctx, createProjectConfiguration,
		configID,
		project.Config.SearchPolicy,
		project.Config.MaxIngestSize,
		project.Config.ReplayAttacks,
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
		project.Config.SSL.EnforceSecureEndpoints,
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

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (p *projectRepo) LoadProjects(ctx context.Context, f *datastore.ProjectFilter) ([]*datastore.Project, error) {
	rows, err := p.db.GetReadDB().QueryxContext(ctx, fetchProjects, f.OrgID)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

	projects := make([]*datastore.Project, 0)
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

func (p *projectRepo) UpdateProject(ctx context.Context, project *datastore.Project) error {
	pro, err := p.FetchProjectByID(ctx, project.UID)
	if err != nil {
		return err
	}

	changelog, err := diff.Diff(pro, project)
	if err != nil {
		return err
	}

	tx, err := p.db.GetDB().BeginTxx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer rollbackTx(tx)

	pRes, err := tx.ExecContext(ctx, updateProjectById, project.UID, project.Name, project.LogoURL, project.RetainedEvents)
	if err != nil {
		return fmt.Errorf("update project err: %v", err)
	}

	rowsAffected, err := pRes.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectNotUpdated
	}

	rlc := project.Config.GetRateLimitConfig()
	sc := project.Config.GetStrategyConfig()
	sgc := project.Config.GetSignatureConfig()
	ssl := project.Config.GetSSLConfig()
	me := project.Config.GetMetaEventConfig()

	cRes, err := tx.ExecContext(ctx, updateProjectConfiguration,
		project.ProjectConfigID,
		project.Config.MaxIngestSize,
		project.Config.ReplayAttacks,
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
		project.Config.SearchPolicy,
		ssl.EnforceSecureEndpoints,
	)
	if err != nil {
		return fmt.Errorf("update project config err: %v", err)
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

		query = p.db.GetDB().Rebind(query)
		rows, err := p.db.GetDB().QueryxContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer closeWithError(rows)

		for rows.Next() {
			var endpoint datastore.Endpoint
			err = rows.StructScan(&endpoint)
			if err != nil {
				return err
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	go p.hook.Fire(datastore.ProjectUpdated, project, changelog)
	return nil
}

func (p *projectRepo) FetchProjectByID(ctx context.Context, id string) (*datastore.Project, error) {
	var project datastore.Project
	err := p.db.GetDB().GetContext(ctx, &project, fetchProjectById, id)
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
	err := p.db.GetReadDB().GetContext(ctx, &stats, projectStatistics, project.UID)
	if err != nil {
		return err
	}

	project.Statistics = &stats
	return nil
}

func (p *projectRepo) DeleteProject(ctx context.Context, id string) error {
	tx, err := p.db.GetDB().BeginTxx(ctx, &sql.TxOptions{})
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

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (p *projectRepo) GetProjectsWithEventsInTheInterval(ctx context.Context, interval int) ([]datastore.ProjectEvents, error) {
	var projects []datastore.ProjectEvents
	rows, err := p.db.GetReadDB().QueryxContext(ctx, getProjectsWithEventsInTheInterval, interval)
	if err != nil {
		return nil, err
	}
	defer closeWithError(rows)

	for rows.Next() {
		var proj datastore.ProjectEvents

		err = rows.StructScan(&proj)
		if err != nil {
			return nil, err
		}

		projects = append(projects, proj)
	}

	return projects, nil
}
