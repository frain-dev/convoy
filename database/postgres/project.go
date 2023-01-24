package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/datastore"
)

var (
	ErrProjectNotCreated = errors.New("project could not be created")
	ErrProjectNotUpdated = errors.New("project could not be updated")
	ErrProjectNotDeleted = errors.New("project could not be deleted")
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

	cfgQuery := sq.Insert("convoy.project_configurations").
		Columns("retention_policy", "max_payload_read_size",
			"replay_attacks_prevention_enabled",
			"retention_policy_enabled", "ratelimit_count",
			"ratelimit_duration", "strategy_type",
			"strategy_duration", "strategy_retry_count",
			"signature_header", "signature_hash").
		Values(o.Config.RetentionPolicy,
			o.Config.MaxIngestSize,
			o.Config.ReplayAttacks,
			o.Config.IsRetentionPolicyEnabled,
			o.Config.RateLimitCount,
			o.Config.RateLimitDuration,
			o.Config.StrategyType,
			o.Config.StrategyDuration,
			o.Config.StrategyRetryCount,
			o.Config.SignatureHeader,
			o.Config.SignatureHash).
		Suffix(`RETURNING id`).
		PlaceholderFormat(sq.Dollar)

	cfgSql, cfgVals, err := cfgQuery.ToSql()
	if err != nil {
		return err
	}

	var id int
	err = tx.QueryRowContext(ctx, cfgSql, cfgVals...).Scan(&id)
	if err != nil {
		return err
	}

	query := sq.Insert("convoy.projects").
		Columns("name", "type", "logo_url", "organisation_id", "project_configuration_id").
		Values(o.Name, o.Type, o.LogoURL, o.OrganisationID, id).
		Suffix(`RETURNING id`).
		PlaceholderFormat(sq.Dollar)

	sql, vals, err := query.ToSql()
	if err != nil {
		return err
	}

	result, err := tx.ExecContext(ctx, sql, vals...)
	if err != nil {
		return err
	}

	nRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if nRows < 1 {
		return ErrProjectNotCreated
	}

	return tx.Commit()
}

func (p *projectRepo) LoadProjects(ctx context.Context, f *datastore.ProjectFilter) ([]*datastore.Project, error) {
	q := sq.Select("*").
		From("convoy.projects").
		Where("organisation_id = ?", f.OrgID).
		OrderBy("id").
		PlaceholderFormat(sq.Dollar)

	sql, vals, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := p.db.Queryx(sql, vals...)
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

	q := sq.Update("convoy.projects").
		Set("name", project.Name).
		Set("logo_url", project.LogoURL).
		Set("updated_at", time.Now()).
		Where("id = ?", project.UID).
		PlaceholderFormat(sq.Dollar)

	sql, vals, err := q.ToSql()
	if err != nil {
		return err
	}

	pRes, err := tx.Exec(sql, vals...)
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

	q = sq.Update("convoy.project_configurations").
		Set("retention_policy", project.Config.RetentionPolicy).
		Set("max_payload_read_size", project.Config.MaxIngestSize).
		Set("replay_attacks_prevention_enabled", project.Config.ReplayAttacks).
		Set("retention_policy_enabled", project.Config.IsRetentionPolicyEnabled).
		Set("ratelimit_count", project.Config.RateLimitCount).
		Set("ratelimit_duration", project.Config.RateLimitDuration).
		Set("strategy_type", project.Config.StrategyType).
		Set("strategy_duration", project.Config.StrategyDuration).
		Set("strategy_retry_count", project.Config.StrategyRetryCount).
		Set("signature_header", project.Config.SignatureHeader).
		Set("signature_hash", project.Config.SignatureHash).
		Set("updated_at", time.Now()).
		Where("id = ?", project.UID).
		PlaceholderFormat(sq.Dollar)

	sql, vals, err = q.ToSql()
	if err != nil {
		return err
	}

	cRes, err := tx.Exec(sql, vals...)
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

func (p *projectRepo) FetchProjectByID(ctx context.Context, id int) (*datastore.Project, error) {
	q := sq.Select(
		"p.name",
		"p.type",
		"p.retained_events",
		"p.created_at",
		"p.updated_at",
		"p.deleted_at",
		"p.organisation_id",
		`c.retention_policy as "config.retention_policy"`,
		`c.max_payload_read_size as "config.max_payload_read_size"`,
		`c.replay_attacks_prevention_enabled as "config.replay_attacks_prevention_enabled"`,
		`c.retention_policy_enabled as "config.retention_policy_enabled"`,
		`c.ratelimit_count as "config.ratelimit_count"`,
		`c.ratelimit_duration as "config.ratelimit_duration"`,
		`c.strategy_type as "config.strategy_type"`,
		`c.strategy_duration as "config.strategy_duration"`,
		`c.strategy_retry_count as "config.strategy_retry_count"`,
		`c.signature_header as "config.signature_header"`,
		`c.signature_hash as "config.signature_hash"`).
		From("convoy.projects as p").
		Join("convoy.project_configurations c ON p.project_configuration_id = c.id").
		Where("p.id = ?", id).
		PlaceholderFormat(sq.Dollar)

	sql, vals, err := q.ToSql()
	if err != nil {
		return nil, err
	}

	var project datastore.Project
	err = p.db.Get(&project, sql, vals...)
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

	q := sq.Update("convoy.projects").
		Set("deleted_at", time.Now()).
		Where("id = ?", id).
		PlaceholderFormat(sq.Dollar)

	sql, vals, err := q.ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(sql, vals...)
	if err != nil {
		return err
	}

	q = sq.Update("convoy.endpoints").
		Set("deleted_at", time.Now()).
		Where("project_id = ?", id).
		PlaceholderFormat(sq.Dollar)

	sql, vals, err = q.ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(sql, vals...)
	if err != nil {
		return err
	}

	q = sq.Update("convoy.events").
		Set("deleted_at", time.Now()).
		Where("project_id = ?", id).
		PlaceholderFormat(sq.Dollar)

	sql, vals, err = q.ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(sql, vals...)
	if err != nil {
		return err
	}

	q = sq.Update("convoy.subscriptions").
		Set("deleted_at", time.Now()).
		Where("project_id = ?", id).
		PlaceholderFormat(sq.Dollar)

	sql, vals, err = q.ToSql()
	if err != nil {
		return err
	}

	_, err = tx.Exec(sql, vals...)
	if err != nil {
		return err
	}

	return tx.Commit()
}
