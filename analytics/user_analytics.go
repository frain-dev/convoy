package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type UserAnalytics struct {
	userRepo datastore.UserRepository
	client   AnalyticsClient
	host     string
}

func newUserAnalytics(userRepo datastore.UserRepository, client AnalyticsClient, host string) *UserAnalytics {
	return &UserAnalytics{
		userRepo: userRepo,
		client:   client,
		host:     host,
	}
}

func (u *UserAnalytics) Track() error {
	_, pagination, err := u.userRepo.LoadUsersPaged(context.Background(), datastore.Pageable{})
	if err != nil {
		return err
	}

	return u.client.Export(u.Name(), Event{"Count": pagination.Total, "Host": u.host})
}

func (u *UserAnalytics) Name() string {
	return DailyUserCount
}
