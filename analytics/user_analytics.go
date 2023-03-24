package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type UserAnalytics struct {
	userRepo   datastore.UserRepository
	client     AnalyticsClient
	instanceID string
}

func newUserAnalytics(userRepo datastore.UserRepository, client AnalyticsClient, instanceID string) *UserAnalytics {
	return &UserAnalytics{
		userRepo:   userRepo,
		client:     client,
		instanceID: instanceID,
	}
}

func (u *UserAnalytics) Track() error {
	return u.track(PerPage, 0, DefaultCursor)
}

func (u *UserAnalytics) track(perPage, count int, cursor string) error {
	users, pagination, err := u.userRepo.LoadUsersPaged(context.Background(), datastore.Pageable{
		PerPage:    perPage,
		NextCursor: cursor,
		Direction:  datastore.Next,
	})
	if err != nil {
		return err
	}

	if len(users) == 0 && !pagination.HasNextPage {
		return u.client.Export(u.Name(), Event{"Count": count, "instanceID": u.instanceID})
	}

	count += len(users)
	cursor = pagination.NextPageCursor
	return u.track(perPage, count, cursor)
}

func (u *UserAnalytics) Name() string {
	return DailyUserCount
}
