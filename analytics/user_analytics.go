package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type UserAnalytics struct {
	userRepo datastore.UserRepository
	client   AnalyticsClient
	source   AnalyticsSource
}

func newUserAnalytics(userRepo datastore.UserRepository, client AnalyticsClient, source AnalyticsSource) *UserAnalytics {
	return &UserAnalytics{
		userRepo: userRepo,
		client:   client,
		source:   source,
	}
}

func (u *UserAnalytics) Track() error {
	_, pagination, err := u.userRepo.LoadUsersPaged(context.Background(), datastore.Pageable{})
	if err != nil {
		return err
	}

	return u.client.Export(u.Name(), Event{"Count": pagination.Total, "Source": u.source})
}

func (u *UserAnalytics) Name() string {
	return DailyUserCount
}
