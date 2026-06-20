package billing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

func TestHasActiveSubscription(t *testing.T) {
	require.True(t, HasActiveSubscription(BillingSubscription{Status: "active"}))
	require.False(t, HasActiveSubscription(BillingSubscription{Status: "canceled"}))
	require.False(t, HasActiveSubscription(BillingSubscription{}))
}

func TestAnyActive(t *testing.T) {
	require.False(t, AnyActive(nil))
	require.False(t, AnyActive([]BillingSubscription{{Status: "canceled"}, {Status: "past_due"}}))
	require.True(t, AnyActive([]BillingSubscription{{Status: "canceled"}, {Status: "active"}}))
}

func TestApplySubscriptionStatus(t *testing.T) {
	tests := []struct {
		name        string
		disabled    bool
		active      bool
		wantChanged bool
		wantDisabUp bool
	}{
		{name: "active reinstates disabled org", disabled: true, active: true, wantChanged: true, wantDisabUp: false},
		{name: "active on enabled org is a no-op", disabled: false, active: true, wantChanged: false, wantDisabUp: false},
		{name: "inactive suspends enabled org", disabled: false, active: false, wantChanged: true, wantDisabUp: true},
		{name: "inactive on disabled org is a no-op", disabled: true, active: false, wantChanged: false, wantDisabUp: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			org := &datastore.Organisation{}
			if tc.disabled {
				org.DisabledAt = null.NewTime(time.Now(), true)
			}

			changed := ApplySubscriptionStatus(org, tc.active)
			require.Equal(t, tc.wantChanged, changed)
			require.Equal(t, tc.wantDisabUp, org.DisabledAt.Valid)
		})
	}
}
