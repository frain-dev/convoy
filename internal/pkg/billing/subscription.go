package billing

func HasActiveSubscription(subscriptionData interface{}) bool {
	if subscriptionData == nil {
		return false
	}

	var subscriptions []Subscription

	switch v := subscriptionData.(type) {
	case *Subscription:
		subscriptions = []Subscription{*v}
	case Subscription:
		subscriptions = []Subscription{v}
	case []Subscription:
		subscriptions = v
	case []*Subscription:
		if len(v) > 0 {
			subscriptions = make([]Subscription, len(v))
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
