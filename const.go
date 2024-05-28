package convoy

import "time"

const (
	HttpPost HttpMethod = "POST"
	HttpGet  HttpMethod = "GET"
)

const (
	RATE_LIMIT               = 0 // should be deleted
	RATE_LIMIT_DURATION      = 0 // should be deleted
	HTTP_TIMEOUT             = 10
	HTTP_TIMEOUT_IN_DURATION = time.Duration(HTTP_TIMEOUT) * time.Second
)
