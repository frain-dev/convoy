package services

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
)

type UpdateSourceService struct {
	SourceRepo   datastore.SourceRepository
	Cache        cache.Cache
	Project      *datastore.Project
	SourceUpdate *models.UpdateSource
	Source       *datastore.Source
}

func (s *UpdateSourceService) Run(ctx context.Context) (*datastore.Source, error) {
	s.Source.Name = *s.SourceUpdate.Name
	s.Source.Verifier = s.SourceUpdate.Verifier.Transform()
	s.Source.Type = s.SourceUpdate.Type

	if s.SourceUpdate.IsDisabled != nil {
		s.Source.IsDisabled = *s.SourceUpdate.IsDisabled
	}

	if s.SourceUpdate.Verifier.Type == datastore.HMacVerifier && s.SourceUpdate.Verifier.HMac == nil {
		return nil, &ServiceError{ErrMsg: "Invalid verifier config for hmac"}
	}

	if s.SourceUpdate.Verifier.Type == datastore.APIKeyVerifier && s.SourceUpdate.Verifier.ApiKey == nil {
		return nil, &ServiceError{ErrMsg: "Invalid verifier config for api key"}
	}

	if s.SourceUpdate.Verifier.Type == datastore.BasicAuthVerifier && s.SourceUpdate.Verifier.BasicAuth == nil {
		return nil, &ServiceError{ErrMsg: "Invalid verifier config for basic auth"}
	}

	if s.SourceUpdate.Type == datastore.PubSubSource {
		if err := pubsub.Validate(s.SourceUpdate.PubSub.Transform()); err != nil {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}
	}

	if s.SourceUpdate.ForwardHeaders != nil {
		s.Source.ForwardHeaders = s.SourceUpdate.ForwardHeaders
	}

	if s.SourceUpdate.PubSub != nil {
		s.Source.PubSub = s.SourceUpdate.PubSub.Transform()
	}

	if s.SourceUpdate.CustomResponse.Body != nil {
		s.Source.CustomResponse.Body = *s.SourceUpdate.CustomResponse.Body
	}

	if s.SourceUpdate.CustomResponse.ContentType != nil {
		s.Source.CustomResponse.ContentType = *s.SourceUpdate.CustomResponse.ContentType
	}

	err := s.SourceRepo.UpdateSource(ctx, s.Project.UID, s.Source)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update source")
		return nil, &ServiceError{ErrMsg: "an error occurred while updating source", Err: err}
	}

	if s.Source.Provider == datastore.TwitterSourceProvider {
		sourceCacheKey := convoy.SourceCacheKey.Get(s.Source.MaskID).String()
		err = s.Cache.Set(ctx, sourceCacheKey, &s.Source, time.Hour*24)
		if err != nil {
			return nil, &ServiceError{ErrMsg: "failed to create source cache", Err: err}
		}
	}

	return s.Source, nil
}
