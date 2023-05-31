package services

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

var ErrSubscriptionNotFound = errors.New("subscription not found")

type FindSubscriptionByIDService struct {
	SubRepo        datastore.SubscriptionRepository
	EndpointRepo   datastore.EndpointRepository
	SourceRepo     datastore.SourceRepository
	Project        *datastore.Project
	SubscriptionId string
	SkipCache      bool
}

func (s *FindSubscriptionByIDService) Run(ctx context.Context) (*datastore.Subscription, error) {
	sub, err := s.SubRepo.FindSubscriptionByID(ctx, s.Project.UID, s.SubscriptionId)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error(ErrSubscriptionNotFound.Error())
		return nil, &ServiceError{ErrMsg: ErrSubscriptionNotFound.Error(), Err: err}
	}

	if s.SkipCache {
		return sub, nil
	}

	// only incoming s.Projects have sources
	if s.Project.Type == datastore.IncomingProject && sub.SourceID != "" {
		source, err := s.SourceRepo.FindSourceByID(ctx, s.Project.UID, sub.SourceID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to find subscription source")
			return nil, &ServiceError{ErrMsg: "failed to find subscription source", Err: err}
		}
		sub.Source = source
	}

	if sub.EndpointID != "" {
		endpoint, err := s.EndpointRepo.FindEndpointByID(ctx, sub.EndpointID, s.Project.UID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Error("failed to find subscription app endpoint")
			return nil, &ServiceError{ErrMsg: "failed to find subscription app endpoint", Err: err}
		}

		sub.Endpoint = endpoint
	}

	return sub, nil
}
