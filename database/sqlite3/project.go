package sqlite3

import (
	"context"
	"strings"

	"github.com/dchest/uniuri"
	"github.com/jmoiron/sqlx"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/bson"
)

func isDuplicateNameIndex(err error) bool {
	return strings.Contains(err.Error(), "name")
}

const (
	createProjectQuery = `
	-- models/user.go:Create
	INSERT INTO users (uid, name, logo_url, organisation_id) 
	VALUES ($1, $2, $3, $4);
	`
)

type projectRepo struct {
	db *sqlx.DB
}

func NewProjectRepo(db *sqlx.DB) datastore.ProjectRepository {
	return &projectRepo{db: db}
}

func (p *projectRepo) CreateProject(ctx context.Context, o *datastore.Project) error {
	_, err := p.db.Exec(createProjectQuery, uniuri.NewLen(uniuri.UUIDLen), o.Name, o.Type, o.Config)
	if err != nil {
		return err
	}
	return nil
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
