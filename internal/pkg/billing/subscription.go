package billing

func HasActiveSubscription(subscriptionData interface{}) bool {
	if subscriptionData == nil {
		return false
	}

	var subscriptions []BillingSubscription

	switch v := subscriptionData.(type) {
	case *BillingSubscription:
		subscriptions = []BillingSubscription{*v}
	case BillingSubscription:
		subscriptions = []BillingSubscription{v}
	case []BillingSubscription:
		subscriptions = v
	case []*BillingSubscription:
		if len(v) > 0 {
			subscriptions = make([]BillingSubscription, len(v))
			for i, sub := range v {
				subscriptions[i] = *sub
			}
		}
	default:
		return false
	}

	if len(subscriptions) == 0 {
		return false
	}

	for _, sub := range subscriptions {
		if sub.Status == "active" {
			return true
		}
	}

	return false
}
