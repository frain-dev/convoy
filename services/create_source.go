package services

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/pkg/log"

	"github.com/frain-dev/convoy/config"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/pubsub"
	"github.com/oklog/ulid/v2"
)

type CreateSourceService struct {
	SourceRepo datastore.SourceRepository
	Cache      cache.Cache
	NewSource  *models.CreateSource
	Project    *datastore.Project
}

func (s *CreateSourceService) Run(ctx context.Context) (*datastore.Source, error) {
	if s.NewSource.Type == datastore.PubSubSource {
		if err := pubsub.Validate(s.NewSource.PubSub.Transform()); err != nil {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}
	}

	cfg, err := config.Get()
	if err != nil {
		return nil, &ServiceError{ErrMsg: "failed to load configuration", Err: err}
	}

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		ProjectID: s.Project.UID,
		MaskID:    uniuri.NewLen(16),
		Name:      s.NewSource.Name,
		Type:      s.NewSource.Type,
		Provider:  s.NewSource.Provider,
		Verifier:  s.NewSource.Verifier.Transform(),
		PubSub:    s.NewSource.PubSub.Transform(),
		CustomResponse: datastore.CustomResponse{
			Body:        s.NewSource.CustomResponse.Body,
			ContentType: s.NewSource.CustomResponse.ContentType,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	buf := uint64(len([]byte(source.CustomResponse.Body)))
	if buf > cfg.MaxResponseSize {
		return nil, &ServiceError{ErrMsg: "source custom response too large"}
	}

	if source.Provider == datastore.TwitterSourceProvider {
		source.ProviderConfig = &datastore.ProviderConfig{Twitter: &datastore.TwitterProviderConfig{}}
	}

	err = s.SourceRepo.CreateSource(ctx, source)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create source")
		return nil, &ServiceError{ErrMsg: "failed to create source", Err: err}
	}

	return source, nil
}
