package billing

func HasActiveSubscription(subscriptionData interface{}) bool {
	if subscriptionData == nil {
		return false
	}

	var subscription map[string]interface{}
	switch v := subscriptionData.(type) {
	case map[string]interface{}:
		subscription = v
	case []interface{}:
		if len(v) > 0 {
			if subMap, ok := v[0].(map[string]interface{}); ok {
				subscription = subMap
			}
		}
	default:
		return false
	}

	if subscription == nil {
		return false
	}

	status, ok := subscription["status"].(string)
	if ok && status == "active" {
		return true
	}

	return false
}
