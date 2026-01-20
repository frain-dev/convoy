package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/sources/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// ServiceError represents an error that occurs during service operations
type ServiceError struct {
	ErrMsg string
	Err    error
}

func (s *ServiceError) Error() string {
	return s.ErrMsg
}

func (s *ServiceError) Unwrap() error {
	return s.Err
}

// Service implements the SourceRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Ensure Service implements datastore.SourceRepository at compile time
var _ datastore.SourceRepository = (*Service)(nil)

// New creates a new Source Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// ============================================================================
// Type Conversion Helpers
// ============================================================================

// stringToPgText converts a string to pgtype.Text
func stringToPgText(s string) pgtype.Text {
	if util.IsStringEmpty(s) {
		return pgtype.Text{String: "", Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

// stringPtrToPgText converts a string pointer to pgtype.Text
func stringPtrToPgText(s *string) pgtype.Text {
	if s == nil || util.IsStringEmpty(*s) {
		return pgtype.Text{String: "", Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// stringPtrFromPgText converts pgtype.Text to a string pointer
func stringPtrFromPgText(t pgtype.Text) *string {
	if !t.Valid || util.IsStringEmpty(t.String) {
		return nil
	}
	s := t.String
	return &s
}

// stringsToPgArray converts []string to []string for pgx
func stringsToPgArray(strs []string) []string {
	if strs == nil {
		return []string{}
	}
	return strs
}

// pubSubToPgJSON converts PubSubConfig to JSON bytes
func pubSubToPgJSON(config *datastore.PubSubConfig) []byte {
	if config == nil {
		return []byte("{}")
	}
	data, _ := json.Marshal(config)
	return data
}

// pgJSONToPubSub converts JSON bytes to PubSubConfig
func pgJSONToPubSub(data []byte) *datastore.PubSubConfig {
	if len(data) == 0 {
		return nil
	}
	var result datastore.PubSubConfig
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil
	}
	return &result
}

// extractVerifierParams extracts verifier parameters based on type
type verifierParams struct {
	basicUser    string
	basicPass    string
	apiKeyHeader string
	apiKeyValue  string
	hmacHash     string
	hmacHeader   string
	hmacSecret   string
	hmacEncoding string
}

func extractVerifierParams(verifier *datastore.VerifierConfig) verifierParams {
	var params verifierParams

	if verifier == nil {
		return params
	}

	switch verifier.Type {
	case datastore.APIKeyVerifier:
		if verifier.ApiKey != nil {
			params.apiKeyHeader = verifier.ApiKey.HeaderName
			params.apiKeyValue = verifier.ApiKey.HeaderValue
		}
	case datastore.BasicAuthVerifier:
		if verifier.BasicAuth != nil {
			params.basicUser = verifier.BasicAuth.UserName
			params.basicPass = verifier.BasicAuth.Password
		}
	case datastore.HMacVerifier:
		if verifier.HMac != nil {
			params.hmacHash = verifier.HMac.Hash
			params.hmacHeader = verifier.HMac.Header
			params.hmacSecret = verifier.HMac.Secret
			params.hmacEncoding = string(verifier.HMac.Encoding)
		}
	}

	return params
}

// buildVerifierConfig constructs VerifierConfig from row data
func buildVerifierConfig(verifierType, basicUser, basicPass, apiKeyHeader, apiKeyValue, hmacHash, hmacHeader, hmacSecret, hmacEncoding string) *datastore.VerifierConfig {
	if util.IsStringEmpty(verifierType) {
		return &datastore.VerifierConfig{Type: datastore.NoopVerifier}
	}

	config := &datastore.VerifierConfig{
		Type: datastore.VerifierType(verifierType),
	}

	switch config.Type {
	case datastore.APIKeyVerifier:
		config.ApiKey = &datastore.ApiKey{
			HeaderName:  apiKeyHeader,
			HeaderValue: apiKeyValue,
		}
	case datastore.BasicAuthVerifier:
		config.BasicAuth = &datastore.BasicAuth{
			UserName: basicUser,
			Password: basicPass,
		}
	case datastore.HMacVerifier:
		config.HMac = &datastore.HMac{
			Hash:     hmacHash,
			Header:   hmacHeader,
			Secret:   hmacSecret,
			Encoding: datastore.EncodingType(hmacEncoding),
		}
	}

	return config
}

// rowToSource converts query row to datastore.Source
func rowToSource(row interface{}) (*datastore.Source, error) {
	var (
		id, name, sourceType, maskID, provider, projectID                      string
		bodyFunction, headerFunction                                           pgtype.Text
		sourceVerifierID                                                       string
		customResponseBody, customResponseContentType                          string
		verifierType, verifierBasicUsername, verifierBasicPassword             string
		verifierAPIKeyHeaderName, verifierAPIKeyHeaderValue                    string
		verifierHmacHash, verifierHmacHeader, verifierHmacSecret, verifierHmac string
		isDisabled                                                             bool
		forwardHeaders, idempotencyKeys                                        []string
		pubSub                                                                 []byte
		createdAt, updatedAt                                                   pgtype.Timestamptz
	)

	switch r := row.(type) {
	case repo.FetchSourceByIDRow:
		id, name, sourceType = r.ID, r.Name, r.Type
		maskID, provider, projectID = r.MaskID, r.Provider, r.ProjectID
		bodyFunction, headerFunction = r.BodyFunction, r.HeaderFunction
		sourceVerifierID = r.SourceVerifierID
		customResponseBody, customResponseContentType = r.CustomResponseBody, r.CustomResponseContentType
		isDisabled = r.IsDisabled
		forwardHeaders, idempotencyKeys = r.ForwardHeaders, r.IdempotencyKeys
		pubSub = r.PubSub
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		verifierType = r.VerifierType
		verifierBasicUsername, verifierBasicPassword = r.VerifierBasicUsername, r.VerifierBasicPassword
		verifierAPIKeyHeaderName, verifierAPIKeyHeaderValue = r.VerifierApiKeyHeaderName, r.VerifierApiKeyHeaderValue
		verifierHmacHash, verifierHmacHeader = r.VerifierHmacHash, r.VerifierHmacHeader
		verifierHmacSecret, verifierHmac = r.VerifierHmacSecret, r.VerifierHmacEncoding

	case repo.FetchSourceByNameRow:
		id, name, sourceType = r.ID, r.Name, r.Type
		maskID, provider, projectID = r.MaskID, r.Provider, r.ProjectID
		bodyFunction, headerFunction = r.BodyFunction, r.HeaderFunction
		sourceVerifierID = r.SourceVerifierID
		customResponseBody, customResponseContentType = r.CustomResponseBody, r.CustomResponseContentType
		isDisabled = r.IsDisabled
		forwardHeaders, idempotencyKeys = r.ForwardHeaders, r.IdempotencyKeys
		pubSub = r.PubSub
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		verifierType = r.VerifierType
		verifierBasicUsername, verifierBasicPassword = r.VerifierBasicUsername, r.VerifierBasicPassword
		verifierAPIKeyHeaderName, verifierAPIKeyHeaderValue = r.VerifierApiKeyHeaderName, r.VerifierApiKeyHeaderValue
		verifierHmacHash, verifierHmacHeader = r.VerifierHmacHash, r.VerifierHmacHeader
		verifierHmacSecret, verifierHmac = r.VerifierHmacSecret, r.VerifierHmacEncoding

	case repo.FetchSourceByMaskIDRow:
		id, name, sourceType = r.ID, r.Name, r.Type
		maskID, provider, projectID = r.MaskID, r.Provider, r.ProjectID
		bodyFunction, headerFunction = r.BodyFunction, r.HeaderFunction
		sourceVerifierID = r.SourceVerifierID
		customResponseBody, customResponseContentType = r.CustomResponseBody, r.CustomResponseContentType
		isDisabled = r.IsDisabled
		forwardHeaders, idempotencyKeys = r.ForwardHeaders, r.IdempotencyKeys
		pubSub = r.PubSub
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		verifierType = r.VerifierType
		verifierBasicUsername, verifierBasicPassword = r.VerifierBasicUsername, r.VerifierBasicPassword
		verifierAPIKeyHeaderName, verifierAPIKeyHeaderValue = r.VerifierApiKeyHeaderName, r.VerifierApiKeyHeaderValue
		verifierHmacHash, verifierHmacHeader = r.VerifierHmacHash, r.VerifierHmacHeader
		verifierHmacSecret, verifierHmac = r.VerifierHmacSecret, r.VerifierHmacEncoding

	case repo.FetchSourcesPaginatedRow:
		id, name, sourceType = r.ID, r.Name, r.Type
		maskID, provider, projectID = r.MaskID, r.Provider, r.ProjectID
		bodyFunction, headerFunction = r.BodyFunction, r.HeaderFunction
		sourceVerifierID = r.SourceVerifierID
		customResponseBody, customResponseContentType = r.CustomResponseBody, r.CustomResponseContentType
		isDisabled = r.IsDisabled
		forwardHeaders, idempotencyKeys = r.ForwardHeaders, r.IdempotencyKeys
		pubSub = r.PubSub
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		verifierType = r.VerifierType
		verifierBasicUsername, verifierBasicPassword = r.VerifierBasicUsername, r.VerifierBasicPassword
		verifierAPIKeyHeaderName, verifierAPIKeyHeaderValue = r.VerifierApiKeyHeaderName, r.VerifierApiKeyHeaderValue
		verifierHmacHash, verifierHmacHeader = r.VerifierHmacHash, r.VerifierHmacHeader
		verifierHmacSecret, verifierHmac = r.VerifierHmacSecret, r.VerifierHmacEncoding

	default:
		return nil, fmt.Errorf("unsupported row type: %T", row)
	}

	source := &datastore.Source{
		UID:            id,
		Name:           name,
		Type:           datastore.SourceType(sourceType),
		MaskID:         maskID,
		Provider:       datastore.SourceProvider(provider),
		IsDisabled:     isDisabled,
		ForwardHeaders: forwardHeaders,
		ProjectID:      projectID,
		PubSub:         pgJSONToPubSub(pubSub),
		CustomResponse: datastore.CustomResponse{
			Body:        customResponseBody,
			ContentType: customResponseContentType,
		},
		IdempotencyKeys: idempotencyKeys,
		BodyFunction:    stringPtrFromPgText(bodyFunction),
		HeaderFunction:  stringPtrFromPgText(headerFunction),
		VerifierID:      sourceVerifierID,
		CreatedAt:       createdAt.Time,
		UpdatedAt:       updatedAt.Time,
	}

	// Build verifier config
	source.Verifier = buildVerifierConfig(
		verifierType, verifierBasicUsername, verifierBasicPassword,
		verifierAPIKeyHeaderName, verifierAPIKeyHeaderValue,
		verifierHmacHash, verifierHmacHeader, verifierHmacSecret, verifierHmac,
	)

	return source, nil
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateSource creates a new source with its verifier in a transaction
func (s *Service) CreateSource(ctx context.Context, source *datastore.Source) error {
	if source == nil {
		return &ServiceError{ErrMsg: "source cannot be nil"}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return &ServiceError{ErrMsg: "failed to create source", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Create verifier if present
	if !util.IsStringEmpty(string(source.Verifier.Type)) && source.Verifier.Type != datastore.NoopVerifier {
		verifierID := ulid.Make().String()

		params := extractVerifierParams(source.Verifier)

		err = qtx.CreateSourceVerifier(ctx, repo.CreateSourceVerifierParams{
			ID:                verifierID,
			Type:              string(source.Verifier.Type),
			BasicUsername:     stringToPgText(params.basicUser),
			BasicPassword:     stringToPgText(params.basicPass),
			ApiKeyHeaderName:  stringToPgText(params.apiKeyHeader),
			ApiKeyHeaderValue: stringToPgText(params.apiKeyValue),
			HmacHash:          stringToPgText(params.hmacHash),
			HmacHeader:        stringToPgText(params.hmacHeader),
			HmacSecret:        stringToPgText(params.hmacSecret),
			HmacEncoding:      stringToPgText(params.hmacEncoding),
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to create source verifier")
			return &ServiceError{ErrMsg: "failed to create source verifier", Err: err}
		}

		source.VerifierID = verifierID
	}

	// Create source
	err = qtx.CreateSource(ctx, repo.CreateSourceParams{
		ID:                        source.UID,
		SourceVerifierID:          stringToPgText(source.VerifierID),
		Name:                      source.Name,
		Type:                      string(source.Type),
		MaskID:                    source.MaskID,
		Provider:                  string(source.Provider),
		IsDisabled:                source.IsDisabled,
		ForwardHeaders:            stringsToPgArray(source.ForwardHeaders),
		ProjectID:                 source.ProjectID,
		PubSub:                    pubSubToPgJSON(source.PubSub),
		CustomResponseBody:        stringToPgText(source.CustomResponse.Body),
		CustomResponseContentType: stringToPgText(source.CustomResponse.ContentType),
		IdempotencyKeys:           stringsToPgArray(source.IdempotencyKeys),
		BodyFunction:              stringPtrToPgText(source.BodyFunction),
		HeaderFunction:            stringPtrToPgText(source.HeaderFunction),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to create source")
		return &ServiceError{ErrMsg: "failed to create source", Err: err}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return &ServiceError{ErrMsg: "failed to create source", Err: err}
	}

	return nil
}

// UpdateSource updates an existing source and its verifier in a transaction
func (s *Service) UpdateSource(ctx context.Context, projectID string, source *datastore.Source) error {
	if source == nil {
		return &ServiceError{ErrMsg: "source cannot be nil"}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return &ServiceError{ErrMsg: "failed to update source", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Update source
	result, err := qtx.UpdateSource(ctx, repo.UpdateSourceParams{
		ID:                        source.UID,
		Name:                      source.Name,
		Type:                      string(source.Type),
		MaskID:                    source.MaskID,
		Provider:                  string(source.Provider),
		IsDisabled:                source.IsDisabled,
		ForwardHeaders:            stringsToPgArray(source.ForwardHeaders),
		ProjectID:                 projectID,
		PubSub:                    pubSubToPgJSON(source.PubSub),
		CustomResponseBody:        stringToPgText(source.CustomResponse.Body),
		CustomResponseContentType: stringToPgText(source.CustomResponse.ContentType),
		IdempotencyKeys:           stringsToPgArray(source.IdempotencyKeys),
		BodyFunction:              stringPtrToPgText(source.BodyFunction),
		HeaderFunction:            stringPtrToPgText(source.HeaderFunction),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to update source")
		return &ServiceError{ErrMsg: "failed to update source", Err: err}
	}

	if result.RowsAffected() == 0 {
		return &ServiceError{ErrMsg: "source not found", Err: datastore.ErrSourceNotFound}
	}

	// Update verifier if present
	if !util.IsStringEmpty(string(source.Verifier.Type)) && source.Verifier.Type != datastore.NoopVerifier {
		params := extractVerifierParams(source.Verifier)

		result2, err := qtx.UpdateSourceVerifier(ctx, repo.UpdateSourceVerifierParams{
			ID:                source.VerifierID,
			Type:              string(source.Verifier.Type),
			BasicUsername:     stringToPgText(params.basicUser),
			BasicPassword:     stringToPgText(params.basicPass),
			ApiKeyHeaderName:  stringToPgText(params.apiKeyHeader),
			ApiKeyHeaderValue: stringToPgText(params.apiKeyValue),
			HmacHash:          stringToPgText(params.hmacHash),
			HmacHeader:        stringToPgText(params.hmacHeader),
			HmacSecret:        stringToPgText(params.hmacSecret),
			HmacEncoding:      stringToPgText(params.hmacEncoding),
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to update source verifier")
			return &ServiceError{ErrMsg: "failed to update source verifier", Err: err}
		}

		if result2.RowsAffected() == 0 {
			return &ServiceError{ErrMsg: "source verifier not found"}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return &ServiceError{ErrMsg: "failed to update source", Err: err}
	}

	return nil
}

// FindSourceByID retrieves a source by its ID
func (s *Service) FindSourceByID(ctx context.Context, projectID, id string) (*datastore.Source, error) {
	row, err := s.repo.FetchSourceByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrSourceNotFound
		}
		s.logger.WithError(err).Error("failed to fetch source by id")
		return nil, &ServiceError{ErrMsg: "error retrieving source", Err: err}
	}

	return rowToSource(row)
}

// FindSourceByName retrieves a source by its name and project ID
func (s *Service) FindSourceByName(ctx context.Context, projectID, name string) (*datastore.Source, error) {
	row, err := s.repo.FetchSourceByName(ctx, repo.FetchSourceByNameParams{
		ProjectID: projectID,
		Name:      name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrSourceNotFound
		}
		s.logger.WithError(err).Error("failed to fetch source by name")
		return nil, &ServiceError{ErrMsg: "error retrieving source", Err: err}
	}

	return rowToSource(row)
}

// FindSourceByMaskID retrieves a source by its mask ID
func (s *Service) FindSourceByMaskID(ctx context.Context, maskID string) (*datastore.Source, error) {
	row, err := s.repo.FetchSourceByMaskID(ctx, maskID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrSourceNotFound
		}
		s.logger.WithError(err).Error("failed to fetch source by mask id")
		return nil, &ServiceError{ErrMsg: "error retrieving source", Err: err}
	}

	return rowToSource(row)
}

// DeleteSourceByID soft deletes a source, its verifier, and associated subscriptions
func (s *Service) DeleteSourceByID(ctx context.Context, projectID, id, verifierID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		s.logger.WithError(err).Error("failed to start transaction")
		return &ServiceError{ErrMsg: "failed to delete source", Err: err}
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// Delete source
	result, err := qtx.DeleteSource(ctx, repo.DeleteSourceParams{
		ID:        id,
		ProjectID: projectID,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to delete source")
		return &ServiceError{ErrMsg: "failed to delete source", Err: err}
	}

	if result.RowsAffected() == 0 {
		return &ServiceError{ErrMsg: "source not found", Err: datastore.ErrSourceNotFound}
	}

	// Delete verifier if present
	if !util.IsStringEmpty(verifierID) {
		err = qtx.DeleteSourceVerifier(ctx, verifierID)
		if err != nil {
			s.logger.WithError(err).Error("failed to delete source verifier")
			return &ServiceError{ErrMsg: "failed to delete source verifier", Err: err}
		}
	}

	// Cascade delete subscriptions
	err = qtx.DeleteSourceSubscriptions(ctx, repo.DeleteSourceSubscriptionsParams{
		SourceID:  stringToPgText(id),
		ProjectID: projectID,
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to delete source subscriptions")
		return &ServiceError{ErrMsg: "failed to delete source subscriptions", Err: err}
	}

	if err = tx.Commit(ctx); err != nil {
		s.logger.WithError(err).Error("failed to commit transaction")
		return &ServiceError{ErrMsg: "failed to delete source", Err: err}
	}

	return nil
}

// LoadSourcesPaged retrieves sources with pagination and filtering
func (s *Service) LoadSourcesPaged(ctx context.Context, projectID string, filter *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	// Determine direction
	direction := "next"
	if pageable.Direction == datastore.Prev {
		direction = "prev"
	}

	// Prepare filters
	hasTypeFilter := !util.IsStringEmpty(string(filter.Type))
	hasProviderFilter := !util.IsStringEmpty(string(filter.Provider))
	hasQueryFilter := !util.IsStringEmpty(filter.Query)

	// Build query filter string
	queryFilter := ""
	if hasQueryFilter {
		queryFilter = "%" + filter.Query + "%"
	}

	// Query sources with pagination
	rows, err := s.repo.FetchSourcesPaginated(ctx, repo.FetchSourcesPaginatedParams{
		Direction:         direction,
		ProjectID:         projectID,
		Cursor:            pageable.Cursor(),
		HasTypeFilter:     hasTypeFilter,
		TypeFilter:        string(filter.Type),
		HasProviderFilter: hasProviderFilter,
		ProviderFilter:    string(filter.Provider),
		HasQueryFilter:    hasQueryFilter,
		QueryFilter:       queryFilter,
		LimitVal:          int64(pageable.Limit()),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to load sources paged")
		return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "an error occurred while fetching sources", Err: err}
	}

	// Convert rows to sources
	sources := make([]datastore.Source, 0, len(rows))
	for _, row := range rows {
		source, err := rowToSource(row)
		if err != nil {
			s.logger.WithError(err).Error("failed to convert row to source")
			return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "an error occurred while processing sources", Err: err}
		}
		sources = append(sources, *source)
	}

	// Build IDs for pagination
	ids := make([]string, len(sources))
	for i := range sources {
		ids[i] = sources[i].UID
	}

	// Trim extra row used for hasNext detection
	if len(sources) > pageable.PerPage {
		sources = sources[:len(sources)-1]
	}

	// Count previous rows for pagination
	var prevRowCount datastore.PrevRowCount
	if len(sources) > 0 {
		first := sources[0]
		count, err := s.repo.CountPrevSources(ctx, repo.CountPrevSourcesParams{
			ProjectID:         projectID,
			Cursor:            first.UID,
			HasTypeFilter:     hasTypeFilter,
			TypeFilter:        string(filter.Type),
			HasProviderFilter: hasProviderFilter,
			ProviderFilter:    string(filter.Provider),
			HasQueryFilter:    hasQueryFilter,
			QueryFilter:       queryFilter,
		})
		if err != nil {
			s.logger.WithError(err).Error("failed to count prev sources")
			return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "an error occurred while counting sources", Err: err}
		}
		prevRowCount.Count = int(count)
	}

	// Build pagination data
	pagination := &datastore.PaginationData{PrevRowCount: prevRowCount}
	pagination = pagination.Build(pageable, ids)

	return sources, *pagination, nil
}

// LoadPubSubSourcesByProjectIDs retrieves PubSub sources across multiple projects
func (s *Service) LoadPubSubSourcesByProjectIDs(ctx context.Context, projectIDs []string, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	// Query PubSub sources
	rows, err := s.repo.FetchPubSubSourcesByProjectIDs(ctx, repo.FetchPubSubSourcesByProjectIDsParams{
		SourceType: string(datastore.PubSubSource),
		ProjectIds: projectIDs,
		Cursor:     pageable.Cursor(),
		LimitVal:   int64(pageable.Limit()),
	})
	if err != nil {
		s.logger.WithError(err).Error("failed to load pubsub sources")
		return nil, datastore.PaginationData{}, &ServiceError{ErrMsg: "an error occurred while fetching pubsub sources", Err: err}
	}

	// Convert rows to sources
	sources := make([]datastore.Source, 0, len(rows))
	for _, row := range rows {
		source := &datastore.Source{
			UID:             row.ID,
			Name:            row.Name,
			Type:            datastore.SourceType(row.Type),
			MaskID:          row.MaskID,
			Provider:        datastore.SourceProvider(row.Provider),
			IsDisabled:      row.IsDisabled,
			ForwardHeaders:  row.ForwardHeaders,
			ProjectID:       row.ProjectID,
			PubSub:          pgJSONToPubSub(row.PubSub),
			IdempotencyKeys: row.IdempotencyKeys,
			BodyFunction:    stringPtrFromPgText(row.BodyFunction),
			HeaderFunction:  stringPtrFromPgText(row.HeaderFunction),
			CreatedAt:       row.CreatedAt.Time,
			UpdatedAt:       row.UpdatedAt.Time,
		}
		sources = append(sources, *source)
	}

	// Handle pagination (forward only for PubSub)
	var hasNext bool
	var cursor string
	if len(sources) > pageable.PerPage {
		cursor = sources[len(sources)-1].UID
		sources = sources[:len(sources)-1]
		hasNext = true
	}

	pagination := &datastore.PaginationData{
		PerPage:        int64(pageable.PerPage),
		HasNextPage:    hasNext,
		NextPageCursor: cursor,
	}

	return sources, *pagination, nil
}
