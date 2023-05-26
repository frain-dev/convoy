package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/config"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
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

	if newSource.Type == datastore.PubSubSource {
		if err := pubsub.Validate(&newSource.PubSub); err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, err)
		}
	}

	cfg, err := config.Get()
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to load configuration"))
	}

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		ProjectID: g.UID,
		MaskID:    uniuri.NewLen(16),
		Name:      newSource.Name,
		Type:      newSource.Type,
		Provider:  newSource.Provider,
		Verifier:  &newSource.Verifier,
		PubSub:    &newSource.PubSub,
		CustomResponse: datastore.CustomResponse{
			Body:        newSource.CustomResponse.Body,
			ContentType: newSource.CustomResponse.ContentType,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	buf := uint64(len([]byte(source.CustomResponse.Body)))
	if buf > cfg.MaxResponseSize {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("source custom response too large"))
	}

	if source.Provider == datastore.TwitterSourceProvider {
		source.ProviderConfig = &datastore.ProviderConfig{Twitter: &datastore.TwitterProviderConfig{}}
	}

	err = s.sourceRepo.CreateSource(ctx, source)
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

	if sourceUpdate.Type == datastore.PubSubSource {
		if err := pubsub.Validate(sourceUpdate.PubSub); err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, err)
		}
	}

	if sourceUpdate.ForwardHeaders != nil {
		source.ForwardHeaders = sourceUpdate.ForwardHeaders
	}

	if sourceUpdate.PubSub != nil {
		source.PubSub = sourceUpdate.PubSub
	}

	if sourceUpdate.CustomResponse.Body != nil {
		source.CustomResponse.Body = *sourceUpdate.CustomResponse.Body
	}

	if sourceUpdate.CustomResponse.ContentType != nil {
		source.CustomResponse.ContentType = *sourceUpdate.CustomResponse.ContentType
	}

	err := s.sourceRepo.UpdateSource(ctx, g.UID, source)
	if err != nil {
		log.WithError(err).Error("failed to update source")
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
