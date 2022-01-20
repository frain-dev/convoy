package convoy

import "time"

const (
	HttpPost HttpMethod = "POST"
)

const (
	// With this Convoy will not process more than 3000
	// concurrent requests per minute. We use github.com/go-chi/httprate
	// which uses a sliding window algorithm, so we should be fine :)
	// TODO(subomi): We need to configure rate limiting to be per
	// client as well as implement distributed limiting backed by
	// a distributed key value store.
	RATE_LIMIT          = 5000
	RATE_LIMIT_DURATION = 1 * time.Minute
)
