package endpoints

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/endpoints/repo"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

var (
	ErrEndpointNotCreated = errors.New("endpoint could not be created")
	ErrEndpointNotUpdated = errors.New("endpoint could not be updated")
	ErrEndpointExists     = errors.New("an endpoint with that name already exists")
)

// Service implements datastore.EndpointRepository using sqlc-generated queries.
type Service struct {
	logger log.Logger
	repo   repo.Querier
	db     *pgxpool.Pool
	hook   *hooks.Hook
	km     keys.KeyManager
}

// Ensure Service implements datastore.EndpointRepository at compile time.
var _ datastore.EndpointRepository = (*Service)(nil)

// New creates a new endpoints service.
func New(logger log.Logger, db database.Database) *Service {
	km, err := keys.Get()
	if err != nil {
		panic(fmt.Sprintf("endpoints: failed to initialize key manager: %v", err))
	}
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
		hook:   db.GetHook(),
		km:     km,
	}
}

// CreateEndpoint inserts a new endpoint.
func (s *Service) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}

	isEncrypted, err := s.repo.CheckEncryptionStatus(ctx)
	if err != nil {
		isEncErr, err2 := s.isEncryptionError(ctx, err)
		if isEncErr && err2 != nil {
			return err2
		}
		return err
	}

	contentType, err := validateAndSetContentType(endpoint.ContentType)
	if err != nil {
		return err
	}

	apiKeyHeaderName, apiKeyHeaderValue, oauth2Config, basicAuthConfig, err := marshalAuthFields(endpoint)
	if err != nil {
		return err
	}

	secretsJSON, err := secretsToJSON(endpoint.Secrets)
	if err != nil {
		return err
	}

	var mtlsClientCert []byte
	if endpoint.MtlsClientCert != nil {
		mtlsClientCert, err = json.Marshal(endpoint.MtlsClientCert)
		if err != nil {
			return fmt.Errorf("failed to marshal mtls_client_cert: %w", err)
		}
	}

	params := repo.CreateEndpointParams{
		ID:                                  common.StringToPgTextNullable(endpoint.UID),
		Name:                                common.StringToPgTextNullable(endpoint.Name),
		Status:                              common.StringToPgTextNullable(string(endpoint.Status)),
		IsEncrypted:                         common.BoolToPgBool(isEncrypted),
		Secrets:                             secretsJSON,
		OwnerID:                             common.StringToPgTextNullable(endpoint.OwnerID),
		Url:                                 common.StringToPgTextNullable(endpoint.Url),
		Description:                         common.StringToPgTextNullable(endpoint.Description),
		HttpTimeout:                         pgtype.Int4{Int32: int32(endpoint.HttpTimeout), Valid: true},
		RateLimit:                           pgtype.Int4{Int32: int32(endpoint.RateLimit), Valid: true},
		RateLimitDuration:                   pgtype.Int4{Int32: int32(endpoint.RateLimitDuration), Valid: true},
		AdvancedSignatures:                  common.BoolToPgBool(endpoint.AdvancedSignatures),
		SlackWebhookUrl:                     common.StringToPgTextNullable(endpoint.SlackWebhookURL),
		SupportEmail:                        common.StringToPgTextNullable(endpoint.SupportEmail),
		AppID:                               common.StringToPgTextNullable(endpoint.AppID),
		ProjectID:                           common.StringToPgTextNullable(projectID),
		AuthenticationType:                  common.StringToPgTextNullable(string(endpoint.GetAuthConfig().Type)),
		AuthenticationTypeApiKeyHeaderName:  common.StringToPgTextNullable(apiKeyHeaderName),
		AuthenticationTypeApiKeyHeaderValue: common.StringToPgTextNullable(apiKeyHeaderValue),
		EncryptionKey:                       common.StringToPgTextNullable(key),
		MtlsClientCert:                      mtlsClientCert,
		Oauth2Config:                        oauth2Config,
		BasicAuthConfig:                     basicAuthConfig,
		ContentType:                         common.StringToPgText(contentType),
	}

	err = s.repo.CreateEndpoint(ctx, params)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return ErrEndpointExists
		}
		isEncErr, err2 := s.isEncryptionError(ctx, err)
		if isEncErr && err2 != nil {
			return err2
		}
		return err
	}

	go s.hook.Fire(context.Background(), datastore.EndpointCreated, endpoint, nil)

	return nil
}

// FindEndpointByID retrieves a single endpoint by ID and project.
func (s *Service) FindEndpointByID(ctx context.Context, id, projectID string) (*datastore.Endpoint, error) {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}

	row, err := s.repo.FindEndpointByID(ctx, repo.FindEndpointByIDParams{
		EncryptionKey: common.StringToPgTextNullable(key),
		ID:            common.StringToPgTextNullable(id),
		ProjectID:     common.StringToPgTextNullable(projectID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrEndpointNotFound
		}
		isEncErr, err2 := s.isEncryptionError(ctx, err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	return rowToEndpoint(row)
}

// FindEndpointsByID retrieves multiple endpoints by their IDs.
func (s *Service) FindEndpointsByID(ctx context.Context, ids []string, projectID string) ([]datastore.Endpoint, error) {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.FindEndpointsByIDs(ctx, repo.FindEndpointsByIDsParams{
		EncryptionKey: common.StringToPgTextNullable(key),
		Ids:           ids,
		ProjectID:     common.StringToPgTextNullable(projectID),
	})
	if err != nil {
		isEncErr, err2 := s.isEncryptionError(ctx, err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	endpoints := make([]datastore.Endpoint, 0, len(rows))
	for _, r := range rows {
		ep, err := rowToEndpoint(r)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, *ep)
	}

	return endpoints, nil
}

// FindEndpointsByAppID retrieves all endpoints for a given app ID.
func (s *Service) FindEndpointsByAppID(ctx context.Context, appID, projectID string) ([]datastore.Endpoint, error) {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.FindEndpointsByAppID(ctx, repo.FindEndpointsByAppIDParams{
		EncryptionKey: common.StringToPgTextNullable(key),
		AppID:         common.StringToPgTextNullable(appID),
		ProjectID:     common.StringToPgTextNullable(projectID),
	})
	if err != nil {
		isEncErr, err2 := s.isEncryptionError(ctx, err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	endpoints := make([]datastore.Endpoint, 0, len(rows))
	for _, r := range rows {
		ep, err := rowToEndpoint(r)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, *ep)
	}

	return endpoints, nil
}

// FindEndpointsByOwnerID retrieves all endpoints for a given owner ID.
func (s *Service) FindEndpointsByOwnerID(ctx context.Context, projectID, ownerID string) ([]datastore.Endpoint, error) {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.FindEndpointsByOwnerID(ctx, repo.FindEndpointsByOwnerIDParams{
		EncryptionKey: common.StringToPgTextNullable(key),
		ProjectID:     common.StringToPgTextNullable(projectID),
		OwnerID:       common.StringToPgTextNullable(ownerID),
	})
	if err != nil {
		isEncErr, err2 := s.isEncryptionError(ctx, err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	endpoints := make([]datastore.Endpoint, 0, len(rows))
	for _, r := range rows {
		ep, err := rowToEndpoint(r)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, *ep)
	}

	return endpoints, nil
}

// FetchEndpointIDsByOwnerID returns only the IDs of endpoints for a given owner.
func (s *Service) FetchEndpointIDsByOwnerID(ctx context.Context, projectID, ownerID string) ([]string, error) {
	return s.repo.FetchEndpointIDsByOwnerID(ctx, repo.FetchEndpointIDsByOwnerIDParams{
		ProjectID: common.StringToPgTextNullable(projectID),
		OwnerID:   common.StringToPgTextNullable(ownerID),
	})
}

// FindEndpointByTargetURL retrieves an endpoint by its target URL within a project.
func (s *Service) FindEndpointByTargetURL(ctx context.Context, projectID, targetURL string) (*datastore.Endpoint, error) {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, err
	}

	row, err := s.repo.FindEndpointByTargetURL(ctx, repo.FindEndpointByTargetURLParams{
		EncryptionKey: common.StringToPgTextNullable(key),
		Url:           common.StringToPgTextNullable(targetURL),
		ProjectID:     common.StringToPgTextNullable(projectID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrEndpointNotFound
		}
		isEncErr, err2 := s.isEncryptionError(ctx, err)
		if isEncErr && err2 != nil {
			return nil, err2
		}
		return nil, err
	}

	return rowToEndpoint(row)
}

// UpdateEndpoint updates an existing endpoint.
func (s *Service) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}

	contentType, err := validateAndSetContentType(endpoint.ContentType)
	if err != nil {
		return err
	}

	apiKeyHeaderName, apiKeyHeaderValue, oauth2Config, basicAuthConfig, err := marshalAuthFields(endpoint)
	if err != nil {
		return err
	}

	secretsJSON, err := secretsToJSON(endpoint.Secrets)
	if err != nil {
		return err
	}
	var secretsText string
	if secretsJSON != nil {
		secretsText = string(secretsJSON)
	}

	var mtlsClientCertText []byte
	if endpoint.MtlsClientCert != nil {
		mtlsClientCertText, err = json.Marshal(endpoint.MtlsClientCert)
		if err != nil {
			return fmt.Errorf("failed to marshal mtls_client_cert: %w", err)
		}
	}

	params := repo.UpdateEndpointParams{
		Name:                                common.StringToPgTextNullable(endpoint.Name),
		Status:                              common.StringToPgTextNullable(string(endpoint.Status)),
		OwnerID:                             common.StringToPgTextNullable(endpoint.OwnerID),
		Url:                                 common.StringToPgTextNullable(endpoint.Url),
		Description:                         common.StringToPgTextNullable(endpoint.Description),
		HttpTimeout:                         pgtype.Int4{Int32: int32(endpoint.HttpTimeout), Valid: true},
		RateLimit:                           pgtype.Int4{Int32: int32(endpoint.RateLimit), Valid: true},
		RateLimitDuration:                   pgtype.Int4{Int32: int32(endpoint.RateLimitDuration), Valid: true},
		AdvancedSignatures:                  common.BoolToPgBool(endpoint.AdvancedSignatures),
		SlackWebhookUrl:                     common.StringToPgTextNullable(endpoint.SlackWebhookURL),
		SupportEmail:                        common.StringToPgTextNullable(endpoint.SupportEmail),
		AuthenticationType:                  common.StringToPgTextNullable(string(endpoint.GetAuthConfig().Type)),
		AuthenticationTypeApiKeyHeaderName:  common.StringToPgTextNullable(apiKeyHeaderName),
		AuthenticationTypeApiKeyHeaderValue: common.StringToPgTextNullable(apiKeyHeaderValue),
		EncryptionKey:                       common.StringToPgTextNullable(key),
		SecretsText:                         common.StringToPgText(secretsText),
		MtlsClientCertText:                  mtlsClientCertText,
		Oauth2ConfigText:                    oauth2Config,
		BasicAuthConfigText:                 basicAuthConfig,
		ContentType:                         common.StringToPgText(contentType),
		ID:                                  common.StringToPgTextNullable(endpoint.UID),
		ProjectID:                           common.StringToPgTextNullable(projectID),
	}

	result, err := s.repo.UpdateEndpoint(ctx, params)
	if err != nil {
		isEncErr, err2 := s.isEncryptionError(ctx, err)
		if isEncErr && err2 != nil {
			return err2
		}
		return err
	}

	if result.RowsAffected() < 1 {
		return ErrEndpointNotUpdated
	}

	go s.hook.Fire(context.Background(), datastore.EndpointUpdated, endpoint, nil)
	return nil
}

// UpdateEndpointStatus updates only the status of an endpoint.
func (s *Service) UpdateEndpointStatus(ctx context.Context, projectID, endpointID string, status datastore.EndpointStatus) error {
	_, err := s.repo.UpdateEndpointStatus(ctx, repo.UpdateEndpointStatusParams{
		Status:    common.StringToPgTextNullable(string(status)),
		ID:        common.StringToPgTextNullable(endpointID),
		ProjectID: common.StringToPgTextNullable(projectID),
	})
	return err
}

// DeleteEndpoint soft-deletes an endpoint and its associated subscriptions
// and portal link relationships within a transaction.
func (s *Service) DeleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint, projectID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := repo.New(tx)

	err = qtx.DeleteEndpoint(ctx, repo.DeleteEndpointParams{
		ID:        common.StringToPgTextNullable(endpoint.UID),
		ProjectID: common.StringToPgTextNullable(projectID),
	})
	if err != nil {
		return err
	}

	err = qtx.DeleteEndpointSubscriptions(ctx, repo.DeleteEndpointSubscriptionsParams{
		EndpointID: common.StringToPgTextNullable(endpoint.UID),
		ProjectID:  common.StringToPgTextNullable(projectID),
	})
	if err != nil {
		return err
	}

	err = qtx.DeletePortalLinkEndpoints(ctx, common.StringToPgTextNullable(endpoint.UID))
	if err != nil {
		return err
	}

	err = tx.Commit(ctx)
	if err != nil {
		return err
	}

	go s.hook.Fire(context.Background(), datastore.EndpointDeleted, endpoint, nil)
	return nil
}

// CountProjectEndpoints returns the total number of endpoints in a project.
func (s *Service) CountProjectEndpoints(ctx context.Context, projectID string) (int64, error) {
	count, err := s.repo.CountProjectEndpoints(ctx, common.StringToPgTextNullable(projectID))
	if err != nil {
		return 0, err
	}

	return count.Int64, nil
}

// LoadEndpointsPaged retrieves endpoints with cursor-based pagination.
func (s *Service) LoadEndpointsPaged(ctx context.Context, projectID string, filter *datastore.Filter, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	pageable.SetCursors()

	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	q := filter.Query
	if !util.IsStringEmpty(q) {
		q = fmt.Sprintf("%%%s%%", q)
	}

	hasOwnerFilter := !util.IsStringEmpty(filter.OwnerID)
	hasNameFilter := !util.IsStringEmpty(q)
	hasEndpointFilter := len(filter.EndpointIDs) > 0

	endpointIDs := filter.EndpointIDs
	if endpointIDs == nil {
		endpointIDs = []string{}
	}

	var endpoints []datastore.Endpoint

	if pageable.Direction == datastore.Next {
		rows, err := s.repo.FetchEndpointsPagedForward(ctx, repo.FetchEndpointsPagedForwardParams{
			EncryptionKey:     common.StringToPgTextNullable(key),
			ProjectID:         common.StringToPgTextNullable(projectID),
			HasOwnerFilter:    common.BoolToPgBool(hasOwnerFilter),
			OwnerID:           common.StringToPgTextNullable(filter.OwnerID),
			HasNameFilter:     common.BoolToPgBool(hasNameFilter),
			NameQuery:         common.StringToPgText(q),
			HasEndpointFilter: common.BoolToPgBool(hasEndpointFilter),
			EndpointIds:       endpointIDs,
			Cursor:            common.StringToPgText(pageable.Cursor()),
			LimitVal:          pgtype.Int8{Int64: int64(pageable.Limit()), Valid: true},
		})
		if err != nil {
			isEncErr, err2 := s.isEncryptionError(ctx, err)
			if isEncErr && err2 != nil {
				return nil, datastore.PaginationData{}, err2
			}
			return nil, datastore.PaginationData{}, err
		}

		for _, r := range rows {
			ep, convErr := rowToEndpoint(r)
			if convErr != nil {
				return nil, datastore.PaginationData{}, convErr
			}
			endpoints = append(endpoints, *ep)
		}
	} else {
		rows, err := s.repo.FetchEndpointsPagedBackward(ctx, repo.FetchEndpointsPagedBackwardParams{
			EncryptionKey:     common.StringToPgTextNullable(key),
			ProjectID:         common.StringToPgTextNullable(projectID),
			HasOwnerFilter:    common.BoolToPgBool(hasOwnerFilter),
			OwnerID:           common.StringToPgTextNullable(filter.OwnerID),
			HasNameFilter:     common.BoolToPgBool(hasNameFilter),
			NameQuery:         common.StringToPgText(q),
			HasEndpointFilter: common.BoolToPgBool(hasEndpointFilter),
			EndpointIds:       endpointIDs,
			Cursor:            common.StringToPgText(pageable.Cursor()),
			LimitVal:          pgtype.Int8{Int64: int64(pageable.Limit()), Valid: true},
		})
		if err != nil {
			isEncErr, err2 := s.isEncryptionError(ctx, err)
			if isEncErr && err2 != nil {
				return nil, datastore.PaginationData{}, err2
			}
			return nil, datastore.PaginationData{}, err
		}

		for _, r := range rows {
			ep, convErr := rowToEndpoint(r)
			if convErr != nil {
				return nil, datastore.PaginationData{}, convErr
			}
			endpoints = append(endpoints, *ep)
		}

		// Backward query returns ASC order; reverse to get DESC order.
		for i, j := 0, len(endpoints)-1; i < j; i, j = i+1, j-1 {
			endpoints[i], endpoints[j] = endpoints[j], endpoints[i]
		}
	}

	ids := make([]string, len(endpoints))
	for i := range endpoints {
		ids[i] = endpoints[i].UID
	}

	if len(endpoints) > pageable.PerPage {
		endpoints = endpoints[:len(endpoints)-1]
	}

	var prevCount datastore.PrevRowCount
	if len(endpoints) > 0 {
		first := endpoints[0]
		countResult, countErr := s.repo.CountPrevEndpoints(ctx, repo.CountPrevEndpointsParams{
			ProjectID:         common.StringToPgTextNullable(projectID),
			Cursor:            common.StringToPgText(first.UID),
			HasOwnerFilter:    common.BoolToPgBool(hasOwnerFilter),
			OwnerID:           common.StringToPgTextNullable(filter.OwnerID),
			HasNameFilter:     common.BoolToPgBool(hasNameFilter),
			NameQuery:         common.StringToPgText(q),
			HasEndpointFilter: common.BoolToPgBool(hasEndpointFilter),
			EndpointIds:       endpointIDs,
		})
		if countErr != nil {
			return nil, datastore.PaginationData{}, countErr
		}
		prevCount = datastore.PrevRowCount{Count: int(countResult.Int64)}
	}

	pagination := &datastore.PaginationData{PrevRowCount: prevCount}
	pagination = pagination.Build(pageable, ids)

	return endpoints, *pagination, nil
}

// UpdateSecrets replaces all secrets on an endpoint.
func (s *Service) UpdateSecrets(ctx context.Context, endpointID, projectID string, secrets datastore.Secrets) error {
	key, err := s.km.GetCurrentKeyFromCache()
	if err != nil {
		return err
	}

	secretsJSON, err := secretsToJSON(secrets)
	if err != nil {
		return err
	}

	_, err = s.repo.UpdateEndpointSecrets(ctx, repo.UpdateEndpointSecretsParams{
		SecretsText:   common.StringToPgText(string(secretsJSON)),
		EncryptionKey: common.StringToPgTextNullable(key),
		ID:            common.StringToPgTextNullable(endpointID),
		ProjectID:     common.StringToPgTextNullable(projectID),
	})

	return err
}

// DeleteSecret marks a single secret as deleted and persists the change.
func (s *Service) DeleteSecret(ctx context.Context, endpoint *datastore.Endpoint, secretID, projectID string) error {
	sc := endpoint.FindSecret(secretID)
	if sc == nil {
		return datastore.ErrSecretNotFound
	}

	sc.DeletedAt = null.NewTime(time.Now(), true)

	return s.UpdateSecrets(ctx, endpoint.UID, projectID, endpoint.Secrets)
}

// isEncryptionError checks whether an error is caused by a missing encryption
// key when encrypted data exists in the database.
func (s *Service) isEncryptionError(ctx context.Context, err error) (bool, error) {
	if strings.Contains(err.Error(), "Illegal argument") {
		isEncrypted, err2 := s.repo.CheckEncryptionStatus(ctx)
		if err2 == nil && isEncrypted {
			return true, keys.ErrCredentialEncryptionFeatureUnavailableUpgradeOrRevert
		}
	}
	return false, nil
}
