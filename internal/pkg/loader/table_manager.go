package loader

import (
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/memorystore"
	"github.com/frain-dev/convoy/pkg/log"
)

// subscriptionTableManager implements SubscriptionTableManager
type subscriptionTableManager struct {
	log log.StdLogger
}

// NewSubscriptionTableManager creates a new table manager
func NewSubscriptionTableManager(log log.StdLogger) SubscriptionTableManager {
	return &subscriptionTableManager{
		log: log,
	}
}

// AddSubscription adds a subscription to the table for its event types
func (tm *subscriptionTableManager) AddSubscription(sub datastore.Subscription, table *memorystore.Table) {
	if sub.FilterConfig == nil {
		return
	}

	eventTypes := sub.FilterConfig.EventTypes
	if len(eventTypes) == 0 {
		return
	}

	// Add the subscription to its current event types
	for _, ev := range eventTypes {
		key := memorystore.NewKey(sub.ProjectID, ev)
		values := tm.getSubscriptionValues(key, table)

		// Remove the subscription if it already exists
		values = tm.removeSubscriptionFromValues(sub.UID, values)

		values = append(values, sub)
		table.Upsert(key, values)
	}
}

// RemoveSubscription removes a subscription from the table for its event types
func (tm *subscriptionTableManager) RemoveSubscription(sub datastore.Subscription, table *memorystore.Table) {
	if sub.FilterConfig == nil {
		return
	}

	eventTypes := sub.FilterConfig.EventTypes
	if len(eventTypes) == 0 {
		return
	}

	for _, ev := range eventTypes {
		key := memorystore.NewKey(sub.ProjectID, ev)
		tm.removeSubscriptionFromKey(sub.UID, key, table)
	}
}

// RemoveSubscriptionFromAllEventTypes removes a subscription from all event types in the table for a given project
func (tm *subscriptionTableManager) RemoveSubscriptionFromAllEventTypes(sub datastore.Subscription, table *memorystore.Table) {
	keys := table.GetKeys()

	for _, key := range keys {
		// Only process keys for this project
		if !key.HasPrefix(sub.ProjectID) {
			continue
		}

		tm.removeSubscriptionFromKey(sub.UID, key, table)
	}
}

// removeSubscriptionFromKey removes a subscription from a specific key in the table
func (tm *subscriptionTableManager) removeSubscriptionFromKey(subscriptionUID string, key memorystore.Key, table *memorystore.Table) {
	values := tm.getSubscriptionValues(key, table)

	// Remove the subscription if it exists in this event type
	found := false
	for id, v := range values {
		if v.UID == subscriptionUID {
			values = append(values[:id], values[id+1:]...)
			found = true
			break
		}
	}

	// Update or delete the key based on whether any subscriptions remain
	if found {
		if len(values) == 0 {
			table.Delete(key)
		} else {
			table.Upsert(key, values)
		}
	}
}

// removeSubscriptionFromValues removes a subscription from a slice of subscriptions
func (tm *subscriptionTableManager) removeSubscriptionFromValues(subscriptionUID string, values []datastore.Subscription) []datastore.Subscription {
	for id, v := range values {
		if v.UID == subscriptionUID {
			return append(values[:id], values[id+1:]...)
		}
	}
	return values
}

// getSubscriptionValues safely retrieves subscription values from a table key
func (tm *subscriptionTableManager) getSubscriptionValues(key memorystore.Key, table *memorystore.Table) []datastore.Subscription {
	row := table.Get(key)
	if row == nil {
		return make([]datastore.Subscription, 0)
	}

	values, ok := row.Value().([]datastore.Subscription)
	if !ok {
		tm.log.Errorf("malformed data in subscriptions memory store with key: %s", key)
		return make([]datastore.Subscription, 0)
	}

	return values
}

// DebugTableContents prints the contents of the table for debugging purposes
func (tm *subscriptionTableManager) DebugTableContents(table *memorystore.Table) {
	for _, key := range table.GetKeys() {
		value := table.Get(key)
		subs, ok := value.Value().([]datastore.Subscription)
		if !ok {
			continue
		}

		subIDs := make([]string, len(subs))
		for i, sub := range subs {
			subIDs[i] = sub.UID
		}
		tm.log.Infof("Key: %s, Subscription IDs: %s", key, fmt.Sprintf("%v", subIDs))
	}
}
