package testdb

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
)

// SeedFilter creates a filter for integration tests
func SeedFilter(db database.Database, subscriptionID, uid, eventType string, headers, body datastore.M) (*datastore.EventTypeFilter, error) {
	if uid == "" {
		uid = ulid.Make().String()
	}

	if eventType == "" {
		eventType = "*"
	}

	// Initialize empty maps if not provided
	if headers == nil {
		headers = datastore.M{}
	}

	if body == nil {
		body = datastore.M{}
	}

	filter := &datastore.EventTypeFilter{
		UID:            uid,
		SubscriptionID: subscriptionID,
		EventType:      eventType,
		Headers:        headers,
		Body:           body,
		RawHeaders:     headers,
		RawBody:        body,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	filterRepo := postgres.NewFilterRepo(db)
	err := filterRepo.CreateFilter(context.Background(), filter)
	if err != nil {
		return nil, err
	}

	return filter, nil
}
