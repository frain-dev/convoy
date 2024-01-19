package convoy

import "time"

const (
	HttpPost HttpMethod = "POST"
	HttpGet  HttpMethod = "GET"
)

const (
	RATE_LIMIT                      = 5000
	RATE_LIMIT_DURATION             = 60
	RATE_LIMIT_DURATION_IN_DURATION = time.Duration(RATE_LIMIT_DURATION) * time.Second
	HTTP_TIMEOUT                    = 10
	HTTP_TIMEOUT_IN_DURATION        = time.Duration(HTTP_TIMEOUT) * time.Second
)
