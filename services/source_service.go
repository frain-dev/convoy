package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"encoding/json"
	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SourceService struct {
	sourceRepo datastore.SourceRepository
	cache      cache.Cache
}

func NewSourceService(sourceRepo datastore.SourceRepository, cache cache.Cache) *SourceService {
	return &SourceService{sourceRepo: sourceRepo, cache: cache}
}

func (s *SourceService) CreateSource(ctx context.Context, newSource *models.Source, g *datastore.Project) (*datastore.Source, error) {
	if newSource.Provider.IsValid() {
		if err := validateSourceForProvider(newSource); err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, err)
		}
	} else {
		if err := util.Validate(newSource); err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, err)
		}
	}

	if newSource.Verifier.Type == datastore.HMacVerifier && newSource.Verifier.HMac == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("Invalid verifier config for hmac"))
	}

	if newSource.Verifier.Type == datastore.APIKeyVerifier && newSource.Verifier.ApiKey == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("Invalid verifier config for api key"))
	}

	if newSource.Verifier.Type == datastore.BasicAuthVerifier && newSource.Verifier.BasicAuth == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("Invalid verifier config for basic auth"))
	}

	if newSource.PubSub.Type == datastore.SqsPubSub && newSource.PubSub.Sqs == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("Invalid pub sub config for sqs"))
	}

	if newSource.PubSub.Type == datastore.GooglePubSub && newSource.PubSub.Google == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("Invalid pub sub config for sqs"))
	}

	source := &datastore.Source{
		UID:       uuid.New().String(),
		ProjectID: g.UID,
		MaskID:    uniuri.NewLen(16),
		Name:      newSource.Name,
		Type:      newSource.Type,
		Provider:  newSource.Provider,
		Verifier:  &newSource.Verifier,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	if source.Provider == datastore.TwitterSourceProvider {
		source.ProviderConfig = &datastore.ProviderConfig{Twitter: &datastore.TwitterProviderConfig{}}
	}

	if !util.IsStringEmpty(string(newSource.PubSub.Type)) {
		if newSource.PubSub.Type == datastore.GooglePubSub {
			serviceAccount, err := json.Marshal(newSource.PubSub.Google.ServiceAccount)
			if err != nil {
				return nil, util.NewServiceError(http.StatusBadRequest, err)
			}

			newSource.PubSub.Google.Credentials = serviceAccount
		}
		source.PubSubConfig = &newSource.PubSub
	}

	err := s.sourceRepo.CreateSource(ctx, source)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create source"))
	}

	return source, nil
}

func validateSourceForProvider(newSource *models.Source) error {
	if util.IsStringEmpty(newSource.Name) {
		return errors.New("please provide a source name")
	}

	if !newSource.Type.IsValid() {
		return errors.New("please provide a valid source type")
	}

	switch newSource.Provider {
	case datastore.GithubSourceProvider,
		datastore.ShopifySourceProvider,
		datastore.TwitterSourceProvider:
		verifierConfig := newSource.Verifier
		if verifierConfig.HMac == nil || verifierConfig.HMac.Secret == "" {
			return fmt.Errorf("hmac secret is required for %s source", newSource.Provider)
		}
	}

	return nil
}

func (s *SourceService) UpdateSource(ctx context.Context, g *datastore.Project, sourceUpdate *models.UpdateSource, source *datastore.Source) (*datastore.Source, error) {
	if err := util.Validate(sourceUpdate); err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	source.Name = *sourceUpdate.Name
	source.Verifier = &sourceUpdate.Verifier
	source.Type = sourceUpdate.Type

	if sourceUpdate.IsDisabled != nil {
		source.IsDisabled = *sourceUpdate.IsDisabled
	}

	if sourceUpdate.Verifier.Type == datastore.HMacVerifier && sourceUpdate.Verifier.HMac == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("Invalid verifier config for hmac"))
	}

	if sourceUpdate.Verifier.Type == datastore.APIKeyVerifier && sourceUpdate.Verifier.ApiKey == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("Invalid verifier config for api key"))
	}

	if sourceUpdate.Verifier.Type == datastore.BasicAuthVerifier && sourceUpdate.Verifier.BasicAuth == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("Invalid verifier config for basic auth"))
	}

	if sourceUpdate.ForwardHeaders != nil {
		source.ForwardHeaders = sourceUpdate.ForwardHeaders
	}

	err := s.sourceRepo.UpdateSource(ctx, g.UID, source)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating source"))
	}

	if source.Provider == datastore.TwitterSourceProvider {
		sourceCacheKey := convoy.SourceCacheKey.Get(source.MaskID).String()
		err = s.cache.Set(ctx, sourceCacheKey, &source, time.Hour*24)
		if err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create source cache"))
		}

	}

	return source, nil
}

func (s *SourceService) FindSourceByID(ctx context.Context, g *datastore.Project, id string) (*datastore.Source, error) {
	source, err := s.sourceRepo.FindSourceByID(ctx, g.UID, id)
	if err != nil {
		if err == datastore.ErrSourceNotFound {
			return nil, util.NewServiceError(http.StatusNotFound, err)
		}

		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("error retrieving source"))
	}

	return source, nil
}

func (s *SourceService) FindSourceByMaskID(ctx context.Context, maskID string) (*datastore.Source, error) {
	source, err := s.sourceRepo.FindSourceByMaskID(ctx, maskID)
	if err != nil {
		if errors.Is(err, datastore.ErrSourceNotFound) {
			return nil, util.NewServiceError(http.StatusNotFound, err)
		}

		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("error retrieving source"))
	}

	return source, nil
}

func (s *SourceService) LoadSourcesPaged(ctx context.Context, g *datastore.Project, filter *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	sources, paginationData, err := s.sourceRepo.LoadSourcesPaged(ctx, g.UID, filter, pageable)
	if err != nil {
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching sources"))
	}

	return sources, paginationData, nil
}

func (s *SourceService) DeleteSource(ctx context.Context, g *datastore.Project, source *datastore.Source) error {
	// ToDo: add check here to ensure the source doesn't have any existing subscriptions
	err := s.sourceRepo.DeleteSourceByID(ctx, g.UID, source.UID)
	if err != nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to delete source"))
	}

	if source.Provider == datastore.TwitterSourceProvider {
		sourceCacheKey := convoy.SourceCacheKey.Get(source.MaskID).String()
		err = s.cache.Delete(ctx, sourceCacheKey)
		if err != nil {
			return util.NewServiceError(http.StatusBadRequest, errors.New("failed to delete source cache"))
		}
	}

	return nil
}
