package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
	"github.com/r3labs/diff/v3"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/projects/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// Service implements the ProjectRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier
	db     *pgxpool.Pool
	hook   *hooks.Hook
}

// Ensure Service implements datastore.ProjectRepository at compile time
var _ datastore.ProjectRepository = (*Service)(nil)

// New creates a new Project Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
		hook:   db.GetHook(),
	}
}

// pubSubToJSON converts PubSubConfig to JSON bytes
func pubSubToJSON(config *datastore.PubSubConfig) []byte {
	if config == nil {
		return []byte("{}")
	}
	data, _ := json.Marshal(config)
	return data
}

// pgTextToSlice converts pgtype.Text to []string
func pgTextToSlice(t pgtype.Text) []string {
	if !t.Valid || t.String == "" {
		return []string{}
	}
	return strings.Split(t.String, ",")
}

// sliceToPgText converts []string to pgtype.Text (comma-separated)
func sliceToPgText(arr []string) pgtype.Text {
	if len(arr) == 0 {
		return pgtype.Text{String: "", Valid: false}
	}
	return pgtype.Text{String: strings.Join(arr, ","), Valid: true}
}

func jsonToSignatureVersions(data []byte) datastore.SignatureVersions {
	if len(data) == 0 {
		return getDefaultSignatureVersions()
	}
	var result []datastore.SignatureVersion
	err := json.Unmarshal(data, &result)
	if err != nil {
		return getDefaultSignatureVersions()
	}
	return result
}

// getDefaultSignatureVersions returns default signature configuration
func getDefaultSignatureVersions() datastore.SignatureVersions {
	return []datastore.SignatureVersion{
		{
			UID:      ulid.Make().String(),
			Hash:     "SHA256",
			Encoding: datastore.HexEncoding,
		},
	}
}

// jsonBytesToPubSub converts JSON bytes to PubSubConfig
func jsonBytesToPubSub(data []byte) *datastore.PubSubConfig {
	if len(data) == 0 {
		return &datastore.PubSubConfig{}
	}
	var result datastore.PubSubConfig
	err := json.Unmarshal(data, &result)
	if err != nil {
		return &datastore.PubSubConfig{}
	}
	return &result
}

// signatureVersionsToJSON converts SignatureVersions to JSON bytes
func signatureVersionsToJSON(versions datastore.SignatureVersions) []byte {
	if len(versions) == 0 {
		return []byte("[]")
	}
	data, err := json.Marshal(versions)
	if err != nil {
		return []byte("[]")
	}
	return data
}

// projectConfigToCreateParams converts ProjectConfig to CreateProjectConfigurationParams
func projectConfigToCreateParams(id string, config *datastore.ProjectConfig) repo.CreateProjectConfigurationParams {
	rlc := config.GetRateLimitConfig()
	sc := config.GetStrategyConfig()
	sgc := config.GetSignatureConfig()
	me := config.GetMetaEventConfig()
	cb := config.GetCircuitBreakerConfig()
	ssl := config.GetSSLConfig()

	return repo.CreateProjectConfigurationParams{
		ID:                             id,
		SearchPolicy:                   common.StringToPgText(config.SearchPolicy),
		MaxPayloadReadSize:             int32(config.MaxIngestSize),
		ReplayAttacksPreventionEnabled: config.ReplayAttacks,
		RatelimitCount:                 int32(rlc.Count),
		RatelimitDuration:              int32(rlc.Duration),
		StrategyType:                   string(sc.Type),
		StrategyDuration:               int32(sc.Duration),
		StrategyRetryCount:             int32(sc.RetryCount),
		SignatureHeader:                string(sgc.Header),
		SignatureVersions:              signatureVersionsToJSON(sgc.Versions),
		DisableEndpoint:                config.DisableEndpoint,
		MetaEventsEnabled:              me.IsEnabled,
		MetaEventsType:                 common.StringToPgText(string(me.Type)),
		MetaEventsEventType:            sliceToPgText(me.EventType),
		MetaEventsUrl:                  common.StringToPgText(me.URL),
		MetaEventsSecret:               common.StringToPgText(me.Secret),
		MetaEventsPubSub:               pubSubToJSON(me.PubSub),
		SslEnforceSecureEndpoints:      common.BoolToPgBool(ssl.EnforceSecureEndpoints),
		CbSampleRate:                   int32(cb.SampleRate),
		CbErrorTimeout:                 int32(cb.ErrorTimeout),
		CbFailureThreshold:             int32(cb.FailureThreshold),
		CbSuccessThreshold:             int32(cb.SuccessThreshold),
		CbObservabilityWindow:          int32(cb.ObservabilityWindow),
		CbMinimumRequestCount:          int32(cb.MinimumRequestCount),
		CbConsecutiveFailureThreshold:  int32(cb.ConsecutiveFailureThreshold),
	}
}

// projectConfigToUpdateParams converts ProjectConfig to UpdateProjectConfigurationParams
func projectConfigToUpdateParams(id string, config *datastore.ProjectConfig) repo.UpdateProjectConfigurationParams {
	rlc := config.GetRateLimitConfig()
	sc := config.GetStrategyConfig()
	sgc := config.GetSignatureConfig()
	me := config.GetMetaEventConfig()
	cb := config.GetCircuitBreakerConfig()
	ssl := config.GetSSLConfig()

	return repo.UpdateProjectConfigurationParams{
		ID:                             id,
		MaxPayloadReadSize:             int32(config.MaxIngestSize),
		ReplayAttacksPreventionEnabled: config.ReplayAttacks,
		RatelimitCount:                 int32(rlc.Count),
		RatelimitDuration:              int32(rlc.Duration),
		StrategyType:                   string(sc.Type),
		StrategyDuration:               int32(sc.Duration),
		StrategyRetryCount:             int32(sc.RetryCount),
		SignatureHeader:                string(sgc.Header),
		SignatureVersions:              signatureVersionsToJSON(sgc.Versions),
		DisableEndpoint:                config.DisableEndpoint,
		MetaEventsEnabled:              me.IsEnabled,
		MetaEventsType:                 common.StringToPgText(string(me.Type)),
		MetaEventsEventType:            sliceToPgText(me.EventType),
		MetaEventsUrl:                  common.StringToPgText(me.URL),
		MetaEventsSecret:               common.StringToPgText(me.Secret),
		MetaEventsPubSub:               pubSubToJSON(me.PubSub),
		SearchPolicy:                   common.StringToPgText(config.SearchPolicy),
		SslEnforceSecureEndpoints:      common.BoolToPgBool(ssl.EnforceSecureEndpoints),
		CbSampleRate:                   int32(cb.SampleRate),
		CbErrorTimeout:                 int32(cb.ErrorTimeout),
		CbFailureThreshold:             int32(cb.FailureThreshold),
		CbSuccessThreshold:             int32(cb.SuccessThreshold),
		CbObservabilityWindow:          int32(cb.ObservabilityWindow),
		CbMinimumRequestCount:          int32(cb.MinimumRequestCount),
		CbConsecutiveFailureThreshold:  int32(cb.ConsecutiveFailureThreshold),
	}
}

// rowToProject converts a FetchProjectByIDRow to datastore.Project
func rowToProject(row interface{}) (*datastore.Project, error) {
	var (
		id, name, projectType, orgID, configID string
		logoUrl                                pgtype.Text
		retainedEvents                         pgtype.Int4
		createdAt, updatedAt, deletedAt        pgtype.Timestamptz
		// Config fields - some are strings, some are pgtype.Text per the generated code
		searchPolicy                                    pgtype.Text
		strategyType, signatureHeader                   pgtype.Text
		signatureVersions                               []byte
		metaEventsType, metaEventsUrl, metaEventsSecret string // These are strings with COALESCE in SQL
		maxPayloadReadSize                              pgtype.Int4
		multipleEndpointSubscriptions                   pgtype.Bool
		replayAttacks                                   pgtype.Bool
		ratelimitCount                                  pgtype.Int4
		ratelimitDuration                               pgtype.Int4
		strategyDuration                                pgtype.Int4
		strategyRetryCount                              pgtype.Int4
		disableEndpoint                                 pgtype.Bool
		sslEnforceSecureEndpoints                       pgtype.Bool
		metaEventsEnabled                               pgtype.Bool
		metaEventsEventType                             pgtype.Text
		metaEventsPubSub                                []byte
		cbSampleRate                                    pgtype.Int4
		cbErrorTimeout                                  pgtype.Int4
		cbFailureThreshold                              pgtype.Int4
		cbSuccessThreshold                              pgtype.Int4
		cbObservabilityWindow                           pgtype.Int4
		cbMinimumRequestCount                           pgtype.Int4
		cbConsecutiveFailureThreshold                   pgtype.Int4
	)

	switch r := row.(type) {
	case repo.FetchProjectByIDRow:
		id, name, projectType = r.ID, r.Name, r.Type
		retainedEvents = r.RetainedEvents
		logoUrl, orgID, configID = r.LogoUrl, r.OrganisationID, r.ProjectConfigurationID
		createdAt, updatedAt, deletedAt = r.CreatedAt, r.UpdatedAt, r.DeletedAt
		// Config
		searchPolicy = r.ConfigSearchPolicy
		maxPayloadReadSize = r.ConfigMaxPayloadReadSize
		multipleEndpointSubscriptions = r.ConfigMultipleEndpointSubscriptions
		replayAttacks = r.ConfigReplayAttacksPreventionEnabled
		ratelimitCount = r.ConfigRatelimitCount
		ratelimitDuration = r.ConfigRatelimitDuration
		strategyType = r.ConfigStrategyType
		strategyDuration = r.ConfigStrategyDuration
		strategyRetryCount = r.ConfigStrategyRetryCount
		signatureHeader = r.ConfigSignatureHeader
		signatureVersions = r.ConfigSignatureVersions
		disableEndpoint = r.ConfigDisableEndpoint
		sslEnforceSecureEndpoints = r.ConfigSslEnforceSecureEndpoints
		metaEventsEnabled = r.ConfigMetaEventsEnabled
		metaEventsType = r.ConfigMetaEventsType
		metaEventsEventType = r.ConfigMetaEventsEventType
		metaEventsUrl = r.ConfigMetaEventsUrl
		metaEventsSecret = r.ConfigMetaEventsSecret
		metaEventsPubSub = r.ConfigMetaEventsPubSub
		cbSampleRate = r.ConfigCbSampleRate
		cbErrorTimeout = r.ConfigCbErrorTimeout
		cbFailureThreshold = r.ConfigCbFailureThreshold
		cbSuccessThreshold = r.ConfigCbSuccessThreshold
		cbObservabilityWindow = r.ConfigCbObservabilityWindow
		cbMinimumRequestCount = r.ConfigCbMinimumRequestCount
		cbConsecutiveFailureThreshold = r.ConfigCbConsecutiveFailureThreshold
	case repo.FetchProjectsRow:
		id, name, projectType = r.ID, r.Name, r.Type
		retainedEvents = r.RetainedEvents
		logoUrl, orgID, configID = r.LogoUrl, r.OrganisationID, r.ProjectConfigurationID
		createdAt, updatedAt, deletedAt = r.CreatedAt, r.UpdatedAt, r.DeletedAt
		// Config
		searchPolicy = r.ConfigSearchPolicy
		maxPayloadReadSize = r.ConfigMaxPayloadReadSize
		multipleEndpointSubscriptions = r.ConfigMultipleEndpointSubscriptions
		replayAttacks = r.ConfigReplayAttacksPreventionEnabled
		ratelimitCount = r.ConfigRatelimitCount
		ratelimitDuration = r.ConfigRatelimitDuration
		strategyType = r.ConfigStrategyType
		strategyDuration = r.ConfigStrategyDuration
		strategyRetryCount = r.ConfigStrategyRetryCount
		signatureHeader = r.ConfigSignatureHeader
		signatureVersions = r.ConfigSignatureVersions
		disableEndpoint = r.ConfigDisableEndpoint
		sslEnforceSecureEndpoints = r.ConfigSslEnforceSecureEndpoints
		metaEventsEnabled = r.ConfigMetaEventsEnabled
		metaEventsType = r.ConfigMetaEventsType
		metaEventsEventType = r.ConfigMetaEventsEventType
		metaEventsUrl = r.ConfigMetaEventsUrl
		metaEventsSecret = r.ConfigMetaEventsSecret
		metaEventsPubSub = r.ConfigMetaEventsPubSub
		cbSampleRate = r.ConfigCbSampleRate
		cbErrorTimeout = r.ConfigCbErrorTimeout
		cbFailureThreshold = r.ConfigCbFailureThreshold
		cbSuccessThreshold = r.ConfigCbSuccessThreshold
		cbObservabilityWindow = r.ConfigCbObservabilityWindow
		cbMinimumRequestCount = r.ConfigCbMinimumRequestCount
		cbConsecutiveFailureThreshold = r.ConfigCbConsecutiveFailureThreshold
	default:
		return nil, fmt.Errorf("unsupported row type: %T", row)
	}

	// Build the project
	project := &datastore.Project{
		UID:             id,
		Name:            name,
		Type:            datastore.ProjectType(projectType),
		RetainedEvents:  int(retainedEvents.Int32),
		LogoURL:         logoUrl.String,
		OrganisationID:  orgID,
		ProjectConfigID: configID,
		CreatedAt:       createdAt.Time,
		UpdatedAt:       updatedAt.Time,
		DeletedAt:       null.NewTime(deletedAt.Time, deletedAt.Valid),
	}

	// Build nested config
	project.Config = &datastore.ProjectConfig{
		SearchPolicy:                  searchPolicy.String,
		MaxIngestSize:                 uint64(maxPayloadReadSize.Int32),
		MultipleEndpointSubscriptions: multipleEndpointSubscriptions.Bool,
		ReplayAttacks:                 replayAttacks.Bool,
		DisableEndpoint:               disableEndpoint.Bool,
		RateLimit: &datastore.RateLimitConfiguration{
			Count:    int(ratelimitCount.Int32),
			Duration: uint64(ratelimitDuration.Int32),
		},
		Strategy: &datastore.StrategyConfiguration{
			Type:       datastore.StrategyProvider(strategyType.String),
			Duration:   uint64(strategyDuration.Int32),
			RetryCount: uint64(strategyRetryCount.Int32),
		},
		Signature: &datastore.SignatureConfiguration{
			Header:   config.SignatureHeaderProvider(signatureHeader.String),
			Versions: jsonToSignatureVersions(signatureVersions),
		},
		SSL: &datastore.SSLConfiguration{
			EnforceSecureEndpoints: sslEnforceSecureEndpoints.Bool,
		},
		MetaEvent: &datastore.MetaEventConfiguration{
			IsEnabled: metaEventsEnabled.Bool,
			Type:      datastore.MetaEventType(metaEventsType),
			EventType: pgTextToSlice(metaEventsEventType),
			URL:       metaEventsUrl,
			Secret:    metaEventsSecret,
			PubSub:    jsonBytesToPubSub(metaEventsPubSub),
		},
		CircuitBreaker: &datastore.CircuitBreakerConfiguration{
			SampleRate:                  uint64(cbSampleRate.Int32),
			ErrorTimeout:                uint64(cbErrorTimeout.Int32),
			FailureThreshold:            uint64(cbFailureThreshold.Int32),
			SuccessThreshold:            uint64(cbSuccessThreshold.Int32),
			ObservabilityWindow:         uint64(cbObservabilityWindow.Int32),
			MinimumRequestCount:         uint64(cbMinimumRequestCount.Int32),
			ConsecutiveFailureThreshold: uint64(cbConsecutiveFailureThreshold.Int32),
		},
	}

	return project, nil
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateProject creates a new project with its configuration
func (s *Service) CreateProject(ctx context.Context, project *datastore.Project) error {
	if project == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("project cannot be nil"))
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Create configuration first
	configID := ulid.Make().String()
	configParams := projectConfigToCreateParams(configID, project.Config)

	err = qtx.CreateProjectConfiguration(ctx, configParams)
	if err != nil {
		s.logger.WithError(err).Error("failed to create project configuration")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Create project
	project.ProjectConfigID = configID
	err = qtx.CreateProject(ctx, repo.CreateProjectParams{
		ID:                     project.UID,
		Name:                   project.Name,
		Type:                   string(project.Type),
		LogoUrl:                common.StringToPgText(project.LogoURL),
		OrganisationID:         project.OrganisationID,
		ProjectConfigurationID: configID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return datastore.ErrDuplicateProjectName
		}
		s.logger.WithError(err).Error("failed to create project")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// LoadProjects retrieves all projects for an organization
func (s *Service) LoadProjects(ctx context.Context, f *datastore.ProjectFilter) ([]*datastore.Project, error) {
	orgID := ""
	if f != nil && !util.IsStringEmpty(f.OrgID) {
		orgID = f.OrgID
	}

	rows, err := s.repo.FetchProjects(ctx, orgID)
	if err != nil {
		s.logger.WithError(err).Error("failed to load projects")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	projects := make([]*datastore.Project, 0, len(rows))
	for _, row := range rows {
		project, err := rowToProject(row)
		if err != nil {
			s.logger.WithError(err).Error("failed to convert row to project")
			return nil, util.NewServiceError(http.StatusInternalServerError, err)
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// UpdateProject updates an existing project
func (s *Service) UpdateProject(ctx context.Context, project *datastore.Project) error {
	if project == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("project cannot be nil"))
	}

	// Fetch existing project for diff
	existing, err := s.FetchProjectByID(ctx, project.UID)
	if err != nil {
		return err
	}

	changelog, err := diff.Diff(existing, project)
	if err != nil {
		s.logger.WithError(err).Error("failed to generate diff")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Update project metadata
	result, err := qtx.UpdateProject(ctx, repo.UpdateProjectParams{
		ID:             project.UID,
		Name:           project.Name,
		LogoUrl:        common.StringToPgText(project.LogoURL),
		RetainedEvents: pgtype.Int4{Int32: int32(project.RetainedEvents), Valid: true},
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to update project")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() == 0 {
		return datastore.ErrProjectNotFound
	}

	// Update configuration
	configParams := projectConfigToUpdateParams(project.ProjectConfigID, project.Config)
	result, err = qtx.UpdateProjectConfiguration(ctx, configParams)
	if err != nil {
		s.logger.WithError(err).Error("failed to update project configuration")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() == 0 {
		s.logger.Error("project configuration not updated")
		return util.NewServiceError(http.StatusInternalServerError, errors.New("project configuration not updated"))
	}

	// Update endpoint statuses if DisableEndpoint is false
	if !project.Config.DisableEndpoint {
		_, err = qtx.UpdateProjectEndpointStatus(ctx, repo.UpdateProjectEndpointStatusParams{
			Status:    string(datastore.ActiveEndpointStatus),
			ProjectID: project.UID,
			Column3:   []string{string(datastore.InactiveEndpointStatus)},
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to update endpoint statuses")
			return util.NewServiceError(http.StatusInternalServerError, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Fire hook asynchronously
	go s.hook.Fire(context.Background(), datastore.ProjectUpdated, project, changelog)

	return nil
}

// FetchProjectByID retrieves a single project by ID
func (s *Service) FetchProjectByID(ctx context.Context, id string) (*datastore.Project, error) {
	row, err := s.repo.FetchProjectByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrProjectNotFound
		}
		s.logger.WithError(err).Error("failed to fetch project by id")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	project, err := rowToProject(row)
	if err != nil {
		s.logger.WithError(err).Error("failed to convert row to project")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return project, nil
}

// FillProjectsStatistics fills statistics for a project
func (s *Service) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	if project == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("project cannot be nil"))
	}

	stats, err := s.repo.FetchProjectStatistics(ctx, common.StringToPgText(project.UID))
	if err != nil {
		s.logger.WithError(err).Error("failed to fetch project statistics")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	project.Statistics = &datastore.ProjectStatistics{
		SubscriptionsExist: stats.SubscriptionsExist,
		EndpointsExist:     stats.EndpointsExist,
		SourcesExist:       stats.SourcesExist,
		EventsExist:        stats.EventsExist,
	}

	return nil
}

// DeleteProject soft deletes a project and cascades to related entities
func (s *Service) DeleteProject(ctx context.Context, uid string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Soft delete project
	_, err = qtx.DeleteProject(ctx, uid)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete project")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Cascade deletes
	_, err = qtx.DeleteProjectEndpoints(ctx, uid)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete project endpoints")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	_, err = qtx.DeleteProjectEvents(ctx, uid)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete project events")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	_, err = qtx.DeleteProjectSubscriptions(ctx, uid)
	if err != nil {
		s.logger.WithError(err).Error("failed to delete project subscriptions")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// GetProjectsWithEventsInTheInterval retrieves projects with event counts in a time interval
func (s *Service) GetProjectsWithEventsInTheInterval(ctx context.Context, interval int) ([]datastore.ProjectEvents, error) {
	rows, err := s.repo.GetProjectsWithEventsInInterval(ctx, int32(interval))
	if err != nil {
		s.logger.WithError(err).Error("failed to get projects with events in interval")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	projects := make([]datastore.ProjectEvents, 0, len(rows))
	for _, row := range rows {
		projects = append(projects, datastore.ProjectEvents{
			Id:          row.ID,
			EventsCount: int(row.EventsCount),
		})
	}

	return projects, nil
}

// CountProjects returns the total count of projects
func (s *Service) CountProjects(ctx context.Context) (int64, error) {
	count, err := s.repo.CountProjects(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to count projects")
		return 0, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return count, nil
}
