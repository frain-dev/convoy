package billing

import (
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

// subscriptionStatusActive is the billing service status that marks a subscription as active.
const subscriptionStatusActive = "active"

// HasActiveSubscription reports whether the given subscription is active.
func HasActiveSubscription(sub BillingSubscription) bool {
	return sub.Status == subscriptionStatusActive
}

// AnyActive reports whether any subscription in the slice is active.
func AnyActive(subs []BillingSubscription) bool {
	for _, sub := range subs {
		if HasActiveSubscription(sub) {
			return true
		}
	}
	return false
}

// ApplySubscriptionStatus toggles org.DisabledAt to match subscription state.
// active=true clears DisabledAt (reinstate); active=false sets it (suspend).
// Returns true if the org was modified, so the caller persists and logs.
func ApplySubscriptionStatus(org *datastore.Organisation, active bool) (changed bool) {
	if active {
		if org.DisabledAt.Valid {
			org.DisabledAt = null.Time{}
			return true
		}
		return false
	}

	if !org.DisabledAt.Valid {
		org.DisabledAt = null.NewTime(time.Now(), true)
		return true
	}
	return false
}
