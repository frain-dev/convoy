package newcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/datastore"

	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/spf13/cobra"
)

var client = http.Client{Timeout: 15 * time.Second}

func AddMigrateCommand(a *cli.App) *cobra.Command {
	var oldBaseURL string
	var newBaseURL string
	var personalAccessKey string
	var newUserPassword string

	cmd := &cobra.Command{
		Use:   "newcloud",
		Short: "Convoy migrations",
	}

	cmd.AddCommand()
	return cmd
}

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

func (m *Migrator) loadUser() (*datastore.User, error) {
	url := fmt.Sprintf("%s/ui/users/random/profile", m.OldBaseURL)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	m.addHeader(r)

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %v", err)
	}

	user := &datastore.User{}
	err = readBody(resp.Body, user)
	if err != nil {
		return nil, fmt.Errorf("failed to read user body: %v", err)
	}

	return user, nil
}

func (m *Migrator) loadOrganisations(pageable pagedResponse) ([]datastore.Organisation, error) {
	url := fmt.Sprintf("%s/ui/organisations?perPage=%d&direction=next&next_page_cursor=%s", m.OldBaseURL, pageable.Pagination.PerPage, pageable.Pagination.NextPageCursor)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	m.addHeader(r)

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get orgs: %v", err)
	}

	orgs := []datastore.Organisation{}
	pg := pagedResponse{Content: &orgs}

	err = readBody(resp.Body, &pg)
	if err != nil {
		return nil, fmt.Errorf("failed to read orgs body: %v", err)
	}

	if pg.Pagination.HasNextPage {
		moreOrgs, err := m.loadOrganisations(pg)
		if err != nil {
			log.WithError(err).Errorf("failed to load next org page, next cursor is %s", pg.Pagination.NextPageCursor)
		}

		orgs = append(orgs, moreOrgs...)
	}

	return orgs, nil
}

func readBody(r io.ReadCloser, i interface{}) error {
	defer r.Close()
	return json.NewDecoder(r).Decode(i)
}

func (m *Migrator) loadOrgProjects(orgID string) ([]datastore.Project, error) {
	url := fmt.Sprintf("%s/ui/organisations/%s/projects", m.OldBaseURL, orgID)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	m.addHeader(r)

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get projects: %v", err)
	}

	projects := []datastore.Project{}

	err = readBody(resp.Body, &projects)
	if err != nil {
		return nil, fmt.Errorf("failed to read projects body: %v", err)
	}

	return projects, nil
}

func (m *Migrator) loadProjectEndpoints(orgID, projectID string, pageable pagedResponse) ([]datastore.Endpoint, error) {
	url := fmt.Sprintf("%s/ui/organisations/%s/projects/%s/endpoints?perPage=%d&direction=next&next_page_cursor=%s", m.OldBaseURL, orgID, projectID, pageable.Pagination.PerPage, pageable.Pagination.NextPageCursor)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	m.addHeader(r)

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints: %v", err)
	}

	endpoints := []datastore.Endpoint{}
	pg := pagedResponse{Content: &endpoints}

	err = readBody(resp.Body, &pg)
	if err != nil {
		return nil, fmt.Errorf("failed to read endpoints body: %v", err)
	}

	if pg.Pagination.HasNextPage {
		moreEndpoints, err := m.loadProjectEndpoints(orgID, projectID, pg)
		if err != nil {
			log.WithError(err).Errorf("failed to load next endpoints page, next cursor is %s", pg.Pagination.NextPageCursor)
		}

		endpoints = append(endpoints, moreEndpoints...)
	}

	return endpoints, nil
}

func (m *Migrator) addHeader(r *http.Request) {
	m.addHeader(r)
}

func (m *Migrator) loadProjectSources(orgID, projectID string, pageable pagedResponse) ([]datastore.Source, error) {
	url := fmt.Sprintf("%s/ui/organisations/%s/projects/%s/sources?perPage=%d&direction=next&next_page_cursor=%s", m.OldBaseURL, orgID, projectID, pageable.Pagination.PerPage, pageable.Pagination.NextPageCursor)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get sources: %v", err)
	}

	sources := []datastore.Source{}
	pg := pagedResponse{Content: &sources}

	err = readBody(resp.Body, &pg)
	if err != nil {
		return nil, fmt.Errorf("failed to read sources body: %v", err)
	}

	if pg.Pagination.HasNextPage {
		moreSources, err := m.loadProjectSources(orgID, projectID, pg)
		if err != nil {
			log.WithError(err).Errorf("failed to load next source page, next cursor is %s", pg.Pagination.NextPageCursor)
		}

		sources = append(sources, moreSources...)
	}

	return sources, nil
}

func (m *Migrator) loadProjectSubscriptions(orgID, projectID string, pageable pagedResponse) ([]datastore.Subscription, error) {
	url := fmt.Sprintf("%s/ui/organisations/%s/projects/%s/subscriptions?perPage=%d&direction=next&next_page_cursor=%s", m.OldBaseURL, orgID, projectID, pageable.Pagination.PerPage, pageable.Pagination.NextPageCursor)
	r, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	m.addHeader(r)

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriptions: %v", err)
	}

	subscriptions := []datastore.Subscription{}
	pg := pagedResponse{Content: &subscriptions}

	err = readBody(resp.Body, &pg)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscriptions body: %v", err)
	}

	if pg.Pagination.HasNextPage {
		moreSubscriptions, err := m.loadProjectSubscriptions(orgID, projectID, pg)
		if err != nil {
			log.WithError(err).Errorf("failed to load next subscriptions page, next cursor is %s", pg.Pagination.NextPageCursor)
		}

		subscriptions = append(subscriptions, moreSubscriptions...)
	}

	return subscriptions, nil
}
