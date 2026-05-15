package billing

import "strings"

func collectBillingSubscriptions(subscriptionData interface{}) ([]BillingSubscription, bool) {
	if subscriptionData == nil {
		return nil, false
	}

	switch v := subscriptionData.(type) {
	case *BillingSubscription:
		if v == nil {
			return nil, false
		}
		return []BillingSubscription{*v}, true
	case BillingSubscription:
		return []BillingSubscription{v}, true
	case []BillingSubscription:
		return v, true
	case []*BillingSubscription:
		if len(v) == 0 {
			return nil, false
		}
		out := make([]BillingSubscription, 0, len(v))
		for _, sub := range v {
			if sub == nil {
				continue
			}
			out = append(out, *sub)
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	default:
		return nil, false
	}
}

func subscriptionHasResolvedPlan(sub BillingSubscription) bool {
	if strings.TrimSpace(sub.PlanID) != "" {
		return true
	}
	if sub.Plan != nil && (strings.TrimSpace(sub.Plan.ID) != "" || strings.TrimSpace(sub.Plan.Name) != "") {
		return true
	}
	return false
}

func HasActiveSubscription(subscriptionData interface{}) bool {
	subs, ok := collectBillingSubscriptions(subscriptionData)
	if !ok || len(subs) == 0 {
		return false
	}

	for _, sub := range subs {
		if sub.Status == "active" {
			return true
		}
	}

	return false
}

// OrgLicenseEntitledBySubscription is true when there is an active subscription tied to a
// billable plan (plan_id or nested plan id/name). This aligns org-scoped license cache with
// the billing UI, which shows "No plan" when the subscription is active but the plan is not resolved yet.
func OrgLicenseEntitledBySubscription(subscriptionData interface{}) bool {
	subs, ok := collectBillingSubscriptions(subscriptionData)
	if !ok || len(subs) == 0 {
		return false
	}

	for _, sub := range subs {
		if sub.Status == "active" && subscriptionHasResolvedPlan(sub) {
			return true
		}
	}

	return false
}
