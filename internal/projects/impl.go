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
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// Service implements the ProjectRepository using SQLc-generated queries
type Service struct {
	logger log.Logger
	repo   repo.Querier
	db     *pgxpool.Pool
	hook   *hooks.Hook
}

// Ensure Service implements datastore.ProjectRepository at compile time
var _ datastore.ProjectRepository = (*Service)(nil)

// New creates a new Project Service
func New(logger log.Logger, db database.Database) *Service {
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
		ID:                             common.StringToPgTextNullable(id),
		SearchPolicy:                   common.StringToPgTextNullable(config.SearchPolicy),
		MaxPayloadReadSize:             pgtype.Int4{Int32: int32(config.MaxIngestSize), Valid: true},
		ReplayAttacksPreventionEnabled: pgtype.Bool{Bool: config.ReplayAttacks, Valid: true},
		RatelimitCount:                 pgtype.Int4{Int32: int32(rlc.Count), Valid: true},
		RatelimitDuration:              pgtype.Int4{Int32: int32(rlc.Duration), Valid: true},
		StrategyType:                   common.StringToPgTextNullable(string(sc.Type)),
		StrategyDuration:               pgtype.Int4{Int32: int32(sc.Duration), Valid: true},
		StrategyRetryCount:             pgtype.Int4{Int32: int32(sc.RetryCount), Valid: true},
		SignatureHeader:                common.StringToPgTextNullable(string(sgc.Header)),
		SignatureVersions:              signatureVersionsToJSON(sgc.Versions),
		DisableEndpoint:                pgtype.Bool{Bool: config.DisableEndpoint, Valid: true},
		MetaEventsEnabled:              pgtype.Bool{Bool: me.IsEnabled, Valid: true},
		MetaEventsType:                 common.StringToPgTextNullable(string(me.Type)),
		MetaEventsEventType:            sliceToPgText(me.EventType),
		MetaEventsUrl:                  common.StringToPgTextNullable(me.URL),
		MetaEventsSecret:               common.StringToPgTextNullable(me.Secret),
		MetaEventsPubSub:               pubSubToJSON(me.PubSub),
		SslEnforceSecureEndpoints:      common.BoolToPgBool(ssl.EnforceSecureEndpoints),
		CbSampleRate:                   pgtype.Int4{Int32: int32(cb.SampleRate), Valid: true},
		CbErrorTimeout:                 pgtype.Int4{Int32: int32(cb.ErrorTimeout), Valid: true},
		CbFailureThreshold:             pgtype.Int4{Int32: int32(cb.FailureThreshold), Valid: true},
		CbSuccessThreshold:             pgtype.Int4{Int32: int32(cb.SuccessThreshold), Valid: true},
		CbObservabilityWindow:          pgtype.Int4{Int32: int32(cb.ObservabilityWindow), Valid: true},
		CbMinimumRequestCount:          pgtype.Int4{Int32: int32(cb.MinimumRequestCount), Valid: true},
		CbConsecutiveFailureThreshold:  pgtype.Int4{Int32: int32(cb.ConsecutiveFailureThreshold), Valid: true},
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
		ID:                             common.StringToPgTextNullable(id),
		MaxPayloadReadSize:             pgtype.Int4{Int32: int32(config.MaxIngestSize), Valid: true},
		ReplayAttacksPreventionEnabled: pgtype.Bool{Bool: config.ReplayAttacks, Valid: true},
		RatelimitCount:                 pgtype.Int4{Int32: int32(rlc.Count), Valid: true},
		RatelimitDuration:              pgtype.Int4{Int32: int32(rlc.Duration), Valid: true},
		StrategyType:                   common.StringToPgTextNullable(string(sc.Type)),
		StrategyDuration:               pgtype.Int4{Int32: int32(sc.Duration), Valid: true},
		StrategyRetryCount:             pgtype.Int4{Int32: int32(sc.RetryCount), Valid: true},
		SignatureHeader:                common.StringToPgTextNullable(string(sgc.Header)),
		SignatureVersions:              signatureVersionsToJSON(sgc.Versions),
		DisableEndpoint:                pgtype.Bool{Bool: config.DisableEndpoint, Valid: true},
		MetaEventsEnabled:              pgtype.Bool{Bool: me.IsEnabled, Valid: true},
		MetaEventsType:                 common.StringToPgTextNullable(string(me.Type)),
		MetaEventsEventType:            sliceToPgText(me.EventType),
		MetaEventsUrl:                  common.StringToPgTextNullable(me.URL),
		MetaEventsSecret:               common.StringToPgTextNullable(me.Secret),
		MetaEventsPubSub:               pubSubToJSON(me.PubSub),
		SearchPolicy:                   common.StringToPgTextNullable(config.SearchPolicy),
		SslEnforceSecureEndpoints:      common.BoolToPgBool(ssl.EnforceSecureEndpoints),
		CbSampleRate:                   pgtype.Int4{Int32: int32(cb.SampleRate), Valid: true},
		CbErrorTimeout:                 pgtype.Int4{Int32: int32(cb.ErrorTimeout), Valid: true},
		CbFailureThreshold:             pgtype.Int4{Int32: int32(cb.FailureThreshold), Valid: true},
		CbSuccessThreshold:             pgtype.Int4{Int32: int32(cb.SuccessThreshold), Valid: true},
		CbObservabilityWindow:          pgtype.Int4{Int32: int32(cb.ObservabilityWindow), Valid: true},
		CbMinimumRequestCount:          pgtype.Int4{Int32: int32(cb.MinimumRequestCount), Valid: true},
		CbConsecutiveFailureThreshold:  pgtype.Int4{Int32: int32(cb.ConsecutiveFailureThreshold), Valid: true},
	}
}

// rowToProject converts a FetchProjectByIDRow to datastore.Project
func rowToProject(row interface{}) (*datastore.Project, error) {
	var (
		id, name, projectType, orgID, configID string
		logoUrl                                pgtype.Text
		retainedEvents                         pgtype.Int4
		createdAt, updatedAt, deletedAt        pgtype.Timestamptz
		// Config fields
		searchPolicy                        pgtype.Text
		strategyType, signatureHeader       string
		signatureVersions                   []byte
		maxPayloadReadSize                  int32
		multipleEndpointSubscriptions       bool
		replayAttacks                       bool
		ratelimitCount                      int32
		ratelimitDuration                   int32
		strategyDuration                    int32
		strategyRetryCount                  int32
		disableEndpoint                     bool
		sslEnforceSecureEndpoints           pgtype.Bool
		metaEventsEnabled                   bool
		metaEventsType, metaEventsEventType pgtype.Text
		metaEventsUrl, metaEventsSecret     pgtype.Text
		metaEventsPubSub                    []byte
		cbSampleRate                        int32
		cbErrorTimeout                      int32
		cbFailureThreshold                  int32
		cbSuccessThreshold                  int32
		cbObservabilityWindow               int32
		cbMinimumRequestCount               int32
		cbConsecutiveFailureThreshold       int32
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
		MaxIngestSize:                 uint64(maxPayloadReadSize),
		MultipleEndpointSubscriptions: multipleEndpointSubscriptions,
		ReplayAttacks:                 replayAttacks,
		DisableEndpoint:               disableEndpoint,
		RateLimit: &datastore.RateLimitConfiguration{
			Count:    int(ratelimitCount),
			Duration: uint64(ratelimitDuration),
		},
		Strategy: &datastore.StrategyConfiguration{
			Type:       datastore.StrategyProvider(strategyType),
			Duration:   uint64(strategyDuration),
			RetryCount: uint64(strategyRetryCount),
		},
		Signature: &datastore.SignatureConfiguration{
			Header:   config.SignatureHeaderProvider(signatureHeader),
			Versions: jsonToSignatureVersions(signatureVersions),
		},
		SSL: &datastore.SSLConfiguration{
			EnforceSecureEndpoints: sslEnforceSecureEndpoints.Bool,
		},
		MetaEvent: &datastore.MetaEventConfiguration{
			IsEnabled: metaEventsEnabled,
			Type:      datastore.MetaEventType(metaEventsType.String),
			EventType: pgTextToSlice(metaEventsEventType),
			URL:       metaEventsUrl.String,
			Secret:    metaEventsSecret.String,
			PubSub:    jsonBytesToPubSub(metaEventsPubSub),
		},
		CircuitBreaker: &datastore.CircuitBreakerConfiguration{
			SampleRate:                  uint64(cbSampleRate),
			ErrorTimeout:                uint64(cbErrorTimeout),
			FailureThreshold:            uint64(cbFailureThreshold),
			SuccessThreshold:            uint64(cbSuccessThreshold),
			ObservabilityWindow:         uint64(cbObservabilityWindow),
			MinimumRequestCount:         uint64(cbMinimumRequestCount),
			ConsecutiveFailureThreshold: uint64(cbConsecutiveFailureThreshold),
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
		s.logger.Error("failed to start transaction", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Create configuration first
	configID := ulid.Make().String()
	configParams := projectConfigToCreateParams(configID, project.Config)

	err = qtx.CreateProjectConfiguration(ctx, configParams)
	if err != nil {
		s.logger.Error("failed to create project configuration", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Create project
	project.ProjectConfigID = configID
	err = qtx.CreateProject(ctx, repo.CreateProjectParams{
		ID:                     common.StringToPgText(project.UID),
		Name:                   common.StringToPgText(project.Name),
		Type:                   common.StringToPgText(string(project.Type)),
		LogoUrl:                common.StringToPgTextNullable(project.LogoURL),
		OrganisationID:         common.StringToPgTextNullable(project.OrganisationID),
		ProjectConfigurationID: common.StringToPgTextNullable(configID),
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			return datastore.ErrDuplicateProjectName
		}
		s.logger.Error("failed to create project", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("failed to commit transaction", "error", err)
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

	rows, err := s.repo.FetchProjects(ctx, common.StringToPgText(orgID))
	if err != nil {
		s.logger.Error("failed to load projects", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	projects := make([]*datastore.Project, 0, len(rows))
	for _, row := range rows {
		project, err := rowToProject(row)
		if err != nil {
			s.logger.Error("failed to convert row to project", "error", err)
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
		s.logger.Error("failed to generate diff", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.Error("failed to start transaction", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Update project metadata
	result, err := qtx.UpdateProject(ctx, repo.UpdateProjectParams{
		ID:             common.StringToPgTextNullable(project.UID),
		Name:           common.StringToPgTextNullable(project.Name),
		LogoUrl:        common.StringToPgTextNullable(project.LogoURL),
		RetainedEvents: pgtype.Int4{Int32: int32(project.RetainedEvents), Valid: true},
	})
	if err != nil {
		s.logger.Error("failed to update project", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() == 0 {
		return datastore.ErrProjectNotFound
	}

	// Update configuration
	configParams := projectConfigToUpdateParams(project.ProjectConfigID, project.Config)
	result, err = qtx.UpdateProjectConfiguration(ctx, configParams)
	if err != nil {
		s.logger.Error("failed to update project configuration", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() == 0 {
		s.logger.Error("project configuration not updated")
		return util.NewServiceError(http.StatusInternalServerError, errors.New("project configuration not updated"))
	}

	// Update endpoint statuses if DisableEndpoint is false
	if !project.Config.DisableEndpoint {
		_, err = qtx.UpdateProjectEndpointStatus(ctx, repo.UpdateProjectEndpointStatusParams{
			Status:    common.StringToPgTextNullable(string(datastore.ActiveEndpointStatus)),
			ProjectID: common.StringToPgTextNullable(project.UID),
			Statuses:  []string{string(datastore.InactiveEndpointStatus)},
		})
		if err != nil {
			s.logger.Error("failed to update endpoint statuses", "error", err)
			return util.NewServiceError(http.StatusInternalServerError, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.Error("failed to commit transaction", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Fire hook asynchronously
	go s.hook.Fire(context.Background(), datastore.ProjectUpdated, project, changelog)

	return nil
}

// FetchProjectByID retrieves a single project by ID
func (s *Service) FetchProjectByID(ctx context.Context, id string) (*datastore.Project, error) {
	row, err := s.repo.FetchProjectByID(ctx, common.StringToPgTextNullable(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrProjectNotFound
		}
		s.logger.Error("failed to fetch project by id", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	project, err := rowToProject(row)
	if err != nil {
		s.logger.Error("failed to convert row to project", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return project, nil
}

// FillProjectsStatistics fills statistics for a project
func (s *Service) FillProjectsStatistics(ctx context.Context, project *datastore.Project) error {
	if project == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("project cannot be nil"))
	}

	stats, err := s.repo.FetchProjectStatistics(ctx, common.StringToPgTextNullable(project.UID))
	if err != nil {
		s.logger.Error("failed to fetch project statistics", "error", err)
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
		s.logger.Error("failed to start transaction", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	uidPgText := common.StringToPgTextNullable(uid)

	// Soft delete project
	_, err = qtx.DeleteProject(ctx, uidPgText)
	if err != nil {
		s.logger.Error("failed to delete project", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	// Cascade deletes
	_, err = qtx.DeleteProjectEndpoints(ctx, uidPgText)
	if err != nil {
		s.logger.Error("failed to delete project endpoints", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	_, err = qtx.DeleteProjectEvents(ctx, uidPgText)
	if err != nil {
		s.logger.Error("failed to delete project events", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	_, err = qtx.DeleteProjectSubscriptions(ctx, uidPgText)
	if err != nil {
		s.logger.Error("failed to delete project subscriptions", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if err := tx.Commit(ctx); err != nil {
		s.logger.Error("failed to commit transaction", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// GetProjectsWithEventsInTheInterval retrieves projects with event counts in a time interval
func (s *Service) GetProjectsWithEventsInTheInterval(ctx context.Context, interval int) ([]datastore.ProjectEvents, error) {
	rows, err := s.repo.GetProjectsWithEventsInInterval(ctx, pgtype.Int4{Int32: int32(interval), Valid: true})
	if err != nil {
		s.logger.Error("failed to get projects with events in interval", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	projects := make([]datastore.ProjectEvents, 0, len(rows))
	for _, row := range rows {
		projects = append(projects, datastore.ProjectEvents{
			Id:          row.ID,
			EventsCount: int(row.EventsCount.Int64),
		})
	}

	return projects, nil
}

// CountProjects returns the total count of projects
func (s *Service) CountProjects(ctx context.Context) (int64, error) {
	count, err := s.repo.CountProjects(ctx)
	if err != nil {
		s.logger.Error("failed to count projects", "error", err)
		return 0, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return count.Int64, nil
}
